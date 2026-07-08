package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/logging"
)

// ServerStatus is the runtime connection state for one MCP server.
type ServerStatus string

const (
	StatusDisabled     ServerStatus = "disabled"
	StatusConnected    ServerStatus = "connected"
	StatusError        ServerStatus = "error"
	StatusDisconnected ServerStatus = "disconnected"
	// StatusReloading is a transient, manager-wide overlay (not a per-server
	// stored state) reported by ListServers while a settings Reload is in
	// flight. It replaces what would otherwise read as "disconnected" for
	// servers that are enabled and were connected before the reload started,
	// so a UI polling status mid-reload does not mistake "reconnecting" for
	// "got disabled".
	StatusReloading ServerStatus = "reloading"
)

// ServerRuntimeStatus is exposed via the management API.
type ServerRuntimeStatus struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Enabled        bool         `json:"enabled"`
	Transport      string       `json:"transport"`
	Status         ServerStatus `json:"status"`
	ToolCount      int          `json:"tool_count"`
	LastError      string       `json:"last_error,omitempty"`
	OAuthConnected bool         `json:"oauth_connected,omitempty"`
}

// ToolInfo describes one registered MCP tool.
type ToolInfo struct {
	ServerID     string `json:"server_id"`
	ServerName   string `json:"server_name"`
	ToolName     string `json:"tool_name"`
	RegistryName string `json:"registry_name"`
	Description  string `json:"description"`
}

// TestResult is returned by ephemeral connect tests.
type TestResult struct {
	OK        bool     `json:"ok"`
	ToolCount int      `json:"tool_count"`
	Tools     []string `json:"tools,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// Manager owns MCP client sessions and discovered tools.
type Manager struct {
	mu      sync.RWMutex
	cfg     Config
	servers map[string]*managedServer
	// reloading is true for the duration of Reload's Close+Start cycle. It is
	// surfaced via ListServers as StatusReloading and used to reject
	// Reconnect/TestServer calls that would otherwise race against Start
	// rebuilding the servers map from scratch (see #6 in the MCP stability
	// review: a manual reconnect that grabs the pre-reload managedServer
	// pointer would silently write into an entry Start() is about to discard).
	reloading bool
}

type managedServer struct {
	cfg       ServerConfig
	conn      *connectedServer
	status    ServerStatus
	lastError string
	// generation identifies the current connection "episode" for this
	// server. It is bumped by connectOne every time it (re)connects — success
	// or failure — and by Close for every entry it tears down. A
	// monitorConnection/autoReconnect goroutine captures the generation at
	// the moment its connection was installed; before acting on a wake-up it
	// re-checks the entry's current generation against that captured value,
	// so a stale goroutine racing a newer connect/reconnect/reload for the
	// same server safely no-ops instead of clobbering fresher state.
	generation uint64
}

// NewManager builds a manager from settings without connecting.
func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg:     cfg,
		servers: make(map[string]*managedServer),
	}
}

// Config returns the manager settings snapshot.
func (m *Manager) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// Start connects all enabled servers in parallel.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	m.servers = make(map[string]*managedServer, len(m.cfg.Servers))
	for _, srv := range m.cfg.Servers {
		entry := &managedServer{cfg: srv}
		if !m.cfg.Enabled || !srv.Enabled {
			entry.status = StatusDisabled
		} else {
			entry.status = StatusDisconnected
		}
		m.servers[srv.ID] = entry
	}
	m.mu.Unlock()

	if !m.cfg.Enabled {
		return
	}

	var wg sync.WaitGroup
	for _, srv := range m.cfg.Servers {
		if !srv.Enabled {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := m.connectOne(ctx, id); err != nil {
				logging.L().Error("mcp.connect_failed", "server", id, "error", err)
			}
		}(srv.ID)
	}
	wg.Wait()
}

func (m *Manager) connectOne(ctx context.Context, serverID string) error {
	m.mu.Lock()
	entry, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	cfg := entry.cfg
	if entry.conn != nil && entry.conn.session != nil {
		_ = entry.conn.session.Close()
		entry.conn = nil
	}
	// Bump generation before the (slow, unlocked) connectServer call so that
	// any monitorConnection goroutine watching a session we just closed above
	// sees a generation mismatch when its Wait() unblocks, and treats the
	// closure as an intentional supersession rather than an unexpected death.
	entry.generation++
	gen := entry.generation
	m.mu.Unlock()

	conn, err := connectServer(ctx, cfg)

	m.mu.Lock()
	entry, ok = m.servers[serverID]
	if !ok || entry.generation != gen {
		// The server was removed, or a newer connect/reconnect/reload already
		// superseded this attempt while connectServer was in flight — discard
		// this (now-stale) result instead of clobbering fresher state.
		m.mu.Unlock()
		if conn != nil && conn.session != nil {
			_ = conn.session.Close()
		}
		return nil
	}
	if err != nil {
		entry.conn = nil
		entry.status = StatusError
		entry.lastError = err.Error()
		m.mu.Unlock()
		return err
	}
	entry.conn = conn
	entry.status = StatusConnected
	entry.lastError = ""
	m.mu.Unlock()

	go m.monitorConnection(serverID, gen, conn)
	return nil
}

// mcpAutoReconnectBackoff defines the bounded automatic-reconnect policy
// applied when monitorConnection observes a live session die unexpectedly.
// After these attempts are exhausted, the server is left in StatusError with
// lastError set — matching manual recovery via the existing
// reconnection-runs API / Settings UI reconnect button, rather than retrying
// silently forever.
var mcpAutoReconnectBackoff = []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second}

// monitorConnection watches one connected session for the underlying
// transport dying — a failed keepalive ping (see defaultKeepAlive in
// client.go), a stdio subprocess exiting, or any other transport-level
// error — and reacts by correcting the cached status (which would otherwise
// stay StatusConnected forever) and driving a bounded automatic reconnect.
//
// gen is the generation captured at the moment this conn was installed. If,
// by the time session.Wait() returns, the entry's live generation no longer
// matches gen, some other connect/reconnect/reload/close already superseded
// this connection intentionally, and this goroutine is a no-op.
func (m *Manager) monitorConnection(serverID string, gen uint64, conn *connectedServer) {
	waitErr := conn.session.Wait()

	m.mu.Lock()
	entry, ok := m.servers[serverID]
	if !ok || entry.generation != gen {
		m.mu.Unlock()
		return
	}
	entry.conn = nil
	entry.status = StatusError
	if waitErr != nil {
		entry.lastError = waitErr.Error()
	} else {
		entry.lastError = "MCP session closed unexpectedly"
	}
	m.mu.Unlock()

	logging.L().Error("mcp.session_closed", "server", serverID, "error", waitErr)
	m.autoReconnect(serverID)
}

// autoReconnect retries connectOne a bounded number of times with backoff
// after monitorConnection detects an unexpected session death. It bails out
// early if the server was disabled, the manager started reloading, or
// something else (a manual Reconnect racing this loop) already restored the
// connection — connectOne itself remains the single source of truth for
// generation-safe connect races, so this loop only needs to decide whether
// it's still worth attempting another connect.
func (m *Manager) autoReconnect(serverID string) {
	for attempt, delay := range mcpAutoReconnectBackoff {
		time.Sleep(delay)

		m.mu.RLock()
		entry, ok := m.servers[serverID]
		reloading := m.reloading
		alreadyConnected := ok && entry.status == StatusConnected
		enabled := ok && m.cfg.Enabled && entry.cfg.Enabled
		m.mu.RUnlock()
		if !ok || reloading || !enabled || alreadyConnected {
			return
		}

		connectCtx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
		err := m.connectOne(connectCtx, serverID)
		cancel()
		if err == nil {
			logging.L().Info("mcp.auto_reconnect_succeeded", "server", serverID, "attempt", attempt+1)
			return
		}
		logging.L().Warn("mcp.auto_reconnect_failed", "server", serverID, "attempt", attempt+1, "error", err)
	}
}

// Close disconnects all MCP sessions.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.servers {
		// Bump generation first so any monitorConnection goroutine watching
		// this session sees a mismatch when session.Close() below unblocks
		// its Wait() call, and treats this as an intentional shutdown rather
		// than an unexpected death — otherwise it would overwrite
		// StatusDisconnected with StatusError and kick off a pointless
		// autoReconnect loop against a manager that is shutting down.
		entry.generation++
		if entry.conn != nil && entry.conn.session != nil {
			_ = entry.conn.session.Close()
			entry.conn = nil
		}
		entry.status = StatusDisconnected
	}
	return nil
}

// Reload replaces the manager config synchronously, then reconnects enabled
// servers in the background. While that background reconnect is in flight,
// ListServers reports enabled servers as StatusReloading (rather than the
// misleading StatusDisconnected produced by Close) and Reconnect rejects
// concurrent calls for the same window, since Start rebuilds the servers map
// from scratch and would otherwise race a manual reconnect for a leaked,
// untracked connection.
func (m *Manager) Reload(ctx context.Context, cfg Config) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.reloading = true
	m.mu.Unlock()

	_ = m.Close()
	m.mu.Lock()
	m.cfg = cfg
	m.servers = make(map[string]*managedServer, len(cfg.Servers))
	for _, srv := range cfg.Servers {
		entry := &managedServer{cfg: srv}
		if !cfg.Enabled || !srv.Enabled {
			entry.status = StatusDisabled
		} else {
			entry.status = StatusDisconnected
		}
		m.servers[srv.ID] = entry
	}
	m.mu.Unlock()

	// Detach reconnects from the SIGHUP reload context. The caller only needs to
	// know that settings were applied; slow or wedged MCP transports should not
	// keep the settings save UI stuck until a full reload timeout elapses.
	go func() {
		defer func() {
			m.mu.Lock()
			m.reloading = false
			m.mu.Unlock()
		}()
		connectCtx, cancel := context.WithTimeout(context.Background(), defaultConnectTimeout)
		defer cancel()
		var wg sync.WaitGroup
		for _, srv := range cfg.Servers {
			if !cfg.Enabled || !srv.Enabled {
				continue
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := m.connectOne(connectCtx, id); err != nil {
					logging.L().Error("mcp.connect_failed", "server", id, "error", err)
				}
			}(srv.ID)
		}
		wg.Wait()
	}()
	return nil
}

// ToolBindings returns live MCP tool bindings for registry wiring.
func (m *Manager) ToolBindings() []ToolBinding {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ToolBinding
	for _, entry := range m.servers {
		if entry.conn == nil || entry.status != StatusConnected {
			continue
		}
		for _, tool := range entry.conn.tools {
			out = append(out, ToolBinding{
				ServerID: entry.cfg.ID,
				Tool:     tool,
				Session:  entry.conn.session,
			})
		}
	}
	return out
}

// ListServers returns configured servers and runtime status.
func (m *Manager) ListServers() []ServerRuntimeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ServerRuntimeStatus, 0, len(m.servers))
	for _, entry := range m.servers {
		status := entry.status
		lastError := entry.lastError
		if !m.cfg.Enabled || !entry.cfg.Enabled {
			status = StatusDisabled
		} else if m.reloading {
			status = StatusReloading
		}
		toolCount := 0
		if entry.conn != nil {
			toolCount = len(entry.conn.tools)
		}
		out = append(out, ServerRuntimeStatus{
			ID:             entry.cfg.ID,
			Name:           entry.cfg.Name,
			Enabled:        entry.cfg.Enabled,
			Transport:      string(entry.cfg.Transport),
			Status:         status,
			ToolCount:      toolCount,
			LastError:      lastError,
			OAuthConnected: entry.cfg.OAuth != nil && OAuthConnected(entry.cfg.ID),
		})
	}
	return out
}

// ListToolInfos returns flat tool metadata for the management API.
func (m *Manager) ListToolInfos() []ToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ToolInfo
	for _, entry := range m.servers {
		if entry.conn == nil {
			continue
		}
		for _, tool := range entry.conn.tools {
			out = append(out, ToolInfo{
				ServerID:     entry.cfg.ID,
				ServerName:   entry.cfg.Name,
				ToolName:     tool.Name,
				RegistryName: ToolName(entry.cfg.ID, tool.Name),
				Description:  tool.Description,
			})
		}
	}
	return out
}

// TestServer attempts a one-off connect + list tools without persisting.
func (m *Manager) TestServer(ctx context.Context, serverID string) TestResult {
	m.mu.RLock()
	reloading := m.reloading
	entry, ok := m.servers[serverID]
	m.mu.RUnlock()
	if reloading {
		return TestResult{Error: "MCP is reloading; try again in a moment"}
	}
	if !ok {
		return TestResult{Error: "unknown server: " + serverID}
	}
	testCfg := entry.cfg
	testCfg.Enabled = true
	conn, err := connectServer(ctx, testCfg)
	if err != nil {
		return TestResult{Error: err.Error()}
	}
	defer conn.session.Close()
	names := make([]string, 0, len(conn.tools))
	for _, tool := range conn.tools {
		names = append(names, tool.Name)
	}
	return TestResult{OK: true, ToolCount: len(conn.tools), Tools: names}
}

// Reconnect disconnects and reconnects one server. It refuses to run while a
// full Reload is in flight: Reload's Start() rebuilds the servers map from
// scratch, so a Reconnect racing that rebuild could write a stale connection
// into an entry Start() is about to discard or has already superseded.
func (m *Manager) Reconnect(ctx context.Context, serverID string) error {
	m.mu.RLock()
	reloading := m.reloading
	entry, ok := m.servers[serverID]
	m.mu.RUnlock()
	if reloading {
		return fmt.Errorf("MCP is reloading; try again in a moment")
	}
	if !ok {
		return nil
	}
	if !m.cfg.Enabled || !entry.cfg.Enabled {
		m.mu.Lock()
		entry.status = StatusDisabled
		m.mu.Unlock()
		return nil
	}
	return m.connectOne(ctx, serverID)
}

// Enabled reports whether MCP is globally enabled.
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg.Enabled
}

// WaitForStartup blocks until all enabled servers finish connecting or timeout.
func (m *Manager) WaitForStartup(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.RLock()
		pending := 0
		for _, entry := range m.servers {
			if !entry.cfg.Enabled || !m.cfg.Enabled {
				continue
			}
			if entry.status == StatusDisconnected {
				pending++
			}
		}
		m.mu.RUnlock()
		if pending == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
