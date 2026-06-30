package mcp

import (
	"context"
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
}

type managedServer struct {
	cfg       ServerConfig
	conn      *connectedServer
	status    ServerStatus
	lastError string
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
	m.mu.Unlock()

	conn, err := connectServer(ctx, cfg)
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok = m.servers[serverID]
	if !ok {
		if conn != nil && conn.session != nil {
			_ = conn.session.Close()
		}
		return nil
	}
	if err != nil {
		entry.conn = nil
		entry.status = StatusError
		entry.lastError = err.Error()
		return err
	}
	entry.conn = conn
	entry.status = StatusConnected
	entry.lastError = ""
	return nil
}

// Close disconnects all MCP sessions.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range m.servers {
		if entry.conn != nil && entry.conn.session != nil {
			_ = entry.conn.session.Close()
			entry.conn = nil
		}
		entry.status = StatusDisconnected
	}
	return nil
}

// Reload replaces the manager config and reconnects enabled servers.
func (m *Manager) Reload(ctx context.Context, cfg Config) error {
	if m == nil {
		return nil
	}
	_ = m.Close()
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	m.Start(ctx)
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
	entry, ok := m.servers[serverID]
	m.mu.RUnlock()
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

// Reconnect disconnects and reconnects one server.
func (m *Manager) Reconnect(ctx context.Context, serverID string) error {
	m.mu.RLock()
	entry, ok := m.servers[serverID]
	m.mu.RUnlock()
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
