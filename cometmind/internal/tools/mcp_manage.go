package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
)

// listMCPServersTool lets the agent inspect MCP server connection status
// directly (e.g. "is Jira connected?") instead of relying on the user to
// check Settings or a human operator to hit the management API by hand.
type listMCPServersTool struct {
	mgr *mcppkg.Manager
}

func (listMCPServersTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "list_mcp_servers",
		Description: "List configured MCP servers and their live connection status (connected, error, disconnected, disabled, reloading), tool count, and last error if any. Use this to check whether an MCP server (e.g. Jira, Confluence, Atlassian) is currently reachable before or after reconnecting it.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (t listMCPServersTool) Execute(ctx context.Context, _ json.RawMessage) (Result, error) {
	if t.mgr == nil {
		return Result{OK: false, Output: "MCP is not configured"}, nil
	}
	servers := t.mgr.ListServers()
	if len(servers) == 0 {
		return Result{OK: true, Output: "No MCP servers configured."}, nil
	}
	var b strings.Builder
	for _, s := range servers {
		fmt.Fprintf(&b, "- %s (%s) [%s transport] status=%s tools=%d",
			s.Name, s.ID, s.Transport, s.Status, s.ToolCount)
		if s.OAuthConnected {
			b.WriteString(" oauth=signed_in")
		}
		if s.ErrorCode != "" {
			fmt.Fprintf(&b, " error_code=%s", s.ErrorCode)
		}
		if s.ErrorHint != "" {
			fmt.Fprintf(&b, " hint=%q", s.ErrorHint)
		} else if s.LastError != "" {
			fmt.Fprintf(&b, " last_error=%q", s.LastError)
		}
		b.WriteString("\n")
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}

// reconnectMCPServerTool lets the agent force a disconnect+reconnect of a
// single MCP server on request (e.g. "reconnect Jira for me"), instead of
// requiring the user or an operator to call the management API by hand.
type reconnectMCPServerTool struct {
	mgr *mcppkg.Manager
}

func (reconnectMCPServerTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "reconnect_mcp_server",
		Description: "Force a disconnect and fresh reconnect of one configured MCP server by its server ID (see list_mcp_servers for IDs). Use this when a server's status is error/disconnected, or when the user asks to reconnect an MCP server (e.g. Jira, Confluence).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"server_id":{"type":"string","description":"The MCP server ID to reconnect, as returned by list_mcp_servers"}
			},
			"required":["server_id"]
		}`),
	}
}

func (t reconnectMCPServerTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.mgr == nil {
		return Result{OK: false, Output: "MCP is not configured"}, nil
	}
	var in struct {
		ServerID string `json:"server_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: "invalid tool input: " + err.Error()}, nil
	}
	serverID := strings.TrimSpace(in.ServerID)
	if serverID == "" {
		return Result{OK: false, Output: "server_id is required"}, nil
	}
	if err := t.mgr.Reconnect(ctx, serverID); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	for _, s := range t.mgr.ListServers() {
		if s.ID == serverID {
			if s.Status == mcppkg.StatusConnected {
				return Result{OK: true, Output: fmt.Sprintf("Reconnected %s (%s): status=%s tools=%d", s.Name, s.ID, s.Status, s.ToolCount)}, nil
			}
			out := fmt.Sprintf("Reconnect attempted for %s (%s): status=%s", s.Name, s.ID, s.Status)
			if s.LastError != "" {
				out += fmt.Sprintf(" last_error=%q", s.LastError)
			}
			return Result{OK: false, Output: out}, nil
		}
	}
	return Result{OK: false, Output: fmt.Sprintf("unknown server: %s", serverID)}, nil
}
