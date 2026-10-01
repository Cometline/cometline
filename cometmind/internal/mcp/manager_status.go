package mcp

import (
	"context"
	"time"
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
