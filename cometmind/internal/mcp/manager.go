package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/logging"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerStatus is the runtime connection state for one MCP server.
type ServerStatus string

const (
	StatusDisabled     ServerStatus = "disabled"
	StatusConnecting   ServerStatus = "connecting"
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
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Enabled        bool             `json:"enabled"`
	Transport      string           `json:"transport"`
	Status         ServerStatus     `json:"status"`
	ToolCount      int              `json:"tool_count"`
	LastError      string           `json:"last_error,omitempty"`
	ErrorCode      ConnectErrorCode `json:"error_code,omitempty"`
	ErrorHint      string           `json:"error_hint,omitempty"`
	OAuthConnected bool             `json:"oauth_connected,omitempty"`
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
	OK        bool             `json:"ok"`
	ToolCount int              `json:"tool_count"`
	Tools     []string         `json:"tools,omitempty"`
	Error     string           `json:"error,omitempty"`
	ErrorCode ConnectErrorCode `json:"error_code,omitempty"`
	ErrorHint string           `json:"error_hint,omitempty"`
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
	errorCode ConnectErrorCode
	errorHint string
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

// Start connects all enabled servers in parallel. The argument is kept for
// call-site compatibility; each server uses its own budget on Background so a
// cancelled or short parent deadline cannot fail its neighbors.
func (m *Manager) Start(_ context.Context) {
	m.mu.Lock()
	cfg := m.cfg
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

	if !cfg.Enabled {
		return
	}

	var wg sync.WaitGroup
	for _, srv := range cfg.Servers {
		if !srv.Enabled {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			// Detach from the caller deadline so one slow neighbor cannot
			// inherit a shared parent timeout. Each server has its own budget.
			if err := m.connectOneWithBudget(context.Background(), id); err != nil {
				logging.L().Error("mcp.connect_failed", "server", id, "error", err)
			}
		}(srv.ID)
	}
	wg.Wait()
}

// connectOneWithBudget runs connectOne under a per-server timeout so a slow
// remote handshake cannot starve its neighbors by sharing one parent deadline.
func (m *Manager) connectOneWithBudget(parent context.Context, serverID string) error {
	if parent == nil {
		parent = context.Background()
	}
	m.mu.RLock()
	entry, ok := m.servers[serverID]
	var cfg ServerConfig
	if ok {
		cfg = entry.cfg
	}
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	timeout := connectTimeoutFor(cfg) + listToolsTimeoutFor(cfg) + 2*time.Second
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return m.connectOne(ctx, serverID)
}

func (m *Manager) connectOne(ctx context.Context, serverID string) error {
	m.mu.Lock()
	entry, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	cfg := entry.cfg
	var oldSession *mcp.ClientSession
	if entry.conn != nil && entry.conn.session != nil {
		oldSession = entry.conn.session
		entry.conn = nil
	}
	// Bump generation before the (slow, unlocked) connectServer call so that
	// any monitorConnection goroutine watching a session we just closed above
	// sees a generation mismatch when its Wait() unblocks, and treats the
	// closure as an intentional supersession rather than an unexpected death.
	entry.generation++
	gen := entry.generation
	entry.status = StatusConnecting
	entry.lastError = ""
	entry.errorCode = ""
	entry.errorHint = ""
	m.mu.Unlock()
	if oldSession != nil {
		_ = oldSession.Close()
	}

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
		classified := classifyConnectErrorFor(serverID, err)
		entry.conn = nil
		entry.status = StatusError
		entry.lastError = classified.Error()
		entry.errorCode = classified.Code
		entry.errorHint = classified.Hint
		m.mu.Unlock()
		return classified
	}
	entry.conn = conn
	entry.status = StatusConnected
	entry.lastError = ""
	entry.errorCode = ""
	entry.errorHint = ""
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
		classified := classifyConnectErrorFor(serverID, waitErr)
		entry.lastError = classified.Error()
		entry.errorCode = classified.Code
		entry.errorHint = classified.Hint
	} else {
		entry.lastError = "MCP session closed unexpectedly"
		entry.errorCode = CodeTransportTimeout
		entry.errorHint = "The MCP server stopped responding. Click Reconnect."
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
		skipRetry := ok && skipAutoReconnect(entry.errorCode)
		m.mu.RUnlock()
		if !ok || reloading || !enabled || alreadyConnected || skipRetry {
			return
		}

		err := m.connectOneWithBudget(context.Background(), serverID)
		if err == nil {
			logging.L().Info("mcp.auto_reconnect_succeeded", "server", serverID, "attempt", attempt+1)
			return
		}
		if skipAutoReconnect(errorCodeOf(err)) {
			return
		}
		logging.L().Warn("mcp.auto_reconnect_failed", "server", serverID, "attempt", attempt+1, "error", err)
	}
}

// Close disconnects all MCP sessions.
func (m *Manager) Close() error {
	m.mu.Lock()
	sessions := make([]*mcp.ClientSession, 0, len(m.servers))
	for _, entry := range m.servers {
		// Bump generation first so any monitorConnection goroutine watching
		// this session sees a mismatch when session.Close() below unblocks
		// its Wait() call, and treats this as an intentional shutdown rather
		// than an unexpected death — otherwise it would overwrite
		// StatusDisconnected with StatusError and kick off a pointless
		// autoReconnect loop against a manager that is shutting down.
		entry.generation++
		if entry.conn != nil && entry.conn.session != nil {
			sessions = append(sessions, entry.conn.session)
			entry.conn = nil
		}
		entry.status = StatusDisconnected
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, session := range sessions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = session.Close()
		}()
	}
	wg.Wait()
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
		var wg sync.WaitGroup
		for _, srv := range cfg.Servers {
			if !cfg.Enabled || !srv.Enabled {
				continue
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if err := m.connectOneWithBudget(context.Background(), id); err != nil {
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
		} else if m.reloading && status == StatusDisconnected {
			// Only the Close→connect gap is "reloading". connecting/connected/error
			// must stay visible so one slow handshake cannot mask its neighbors.
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
			ErrorCode:      entry.errorCode,
			ErrorHint:      entry.errorHint,
			OAuthConnected: OAuthConnected(entry.cfg.ID),
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
func (m *Manager) TestServer(_ context.Context, serverID string) TestResult {
	m.mu.RLock()
	reloading := m.reloading
	entry, ok := m.servers[serverID]
	m.mu.RUnlock()
	if reloading {
		return TestResult{
			Error:     "MCP is reloading; try again in a moment",
			ErrorCode: CodeProtocol,
			ErrorHint: "MCP is reloading. Try Test again in a moment.",
		}
	}
	if !ok {
		return TestResult{
			Error:     "unknown server: " + serverID,
			ErrorCode: CodeProtocol,
			ErrorHint: "Save this MCP server before testing it.",
		}
	}
	testCfg := entry.cfg
	testCfg.Enabled = true
	timeout := connectTimeoutFor(testCfg) + listToolsTimeoutFor(testCfg) + 2*time.Second
	testCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := connectServer(testCtx, testCfg)
	if err != nil {
		classified := classifyConnectErrorFor(serverID, err)
		return TestResult{
			Error:     classified.Error(),
			ErrorCode: classified.Code,
			ErrorHint: classified.Hint,
		}
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
func (m *Manager) Reconnect(_ context.Context, serverID string) error {
	m.mu.Lock()
	if m.reloading {
		m.mu.Unlock()
		return fmt.Errorf("MCP is reloading; try again in a moment")
	}
	entry, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	if !m.cfg.Enabled || !entry.cfg.Enabled {
		entry.status = StatusDisabled
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()
	// Detach from the inbound HTTP request: a cancelled browser tab must not
	// abort a 45s remote handshake the user just asked for.
	return m.connectOneWithBudget(context.Background(), serverID)
}

// CallTool invokes a live MCP tool by server id. It always looks up the
// current session so a reconnect between turns is visible to the next call.
func (m *Manager) CallTool(ctx context.Context, serverID, toolName string, args map[string]any) (*mcp.CallToolResult, error) {
	m.mu.RLock()
	entry, ok := m.servers[serverID]
	var session *mcp.ClientSession
	if ok && entry.conn != nil {
		session = entry.conn.session
	}
	m.mu.RUnlock()
	if session == nil {
		return nil, fmt.Errorf("MCP server %q is not connected", serverID)
	}
	return session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: args})
}
