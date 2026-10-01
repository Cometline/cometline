package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
