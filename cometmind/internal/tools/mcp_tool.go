package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpCallToolTimeout bounds a single MCP tool call, mirroring the timeout
// pattern used by other built-in tools (grep.go, runcommand.go, webfetch.go).
// Without this, a zombie MCP session (see internal/mcp keepalive handling)
// could otherwise hang for the full duration of the parent turn's context.
const mcpCallToolTimeout = 60 * time.Second

type mcpTool struct {
	serverID    string
	toolName    string
	description string
	parameters  json.RawMessage
	mgr         *mcppkg.Manager
}

func mcpToolsFromManager(mgr *mcppkg.Manager) []Tool {
	if mgr == nil {
		return nil
	}
	bindings := mgr.ToolBindings()
	out := make([]Tool, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, mcpTool{
			serverID:    binding.ServerID,
			toolName:    binding.Tool.Name,
			description: binding.Tool.Description,
			parameters:  binding.Tool.Parameters,
			mgr:         mgr,
		})
	}
	return out
}

func (t mcpTool) Spec() ToolSpec {
	desc := strings.TrimSpace(t.description)
	if desc == "" {
		desc = fmt.Sprintf("MCP tool %s from server %s", t.toolName, t.serverID)
	}
	params := t.parameters
	if len(params) == 0 {
		params = json.RawMessage(`{"type":"object","properties":{}}`)
	}
	return ToolSpec{
		Name:        mcppkg.ToolName(t.serverID, t.toolName),
		Description: desc,
		Parameters:  params,
	}
}

func (t mcpTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.mgr == nil {
		return Result{OK: false, Output: "MCP session not connected"}, nil
	}
	var args map[string]any
	if len(input) > 0 && string(input) != "null" {
		if err := json.Unmarshal(input, &args); err != nil {
			return Result{OK: false, Output: "invalid tool input: " + err.Error()}, nil
		}
	}
	callCtx, cancel := context.WithTimeout(ctx, mcpCallToolTimeout)
	defer cancel()
	res, err := t.mgr.CallTool(callCtx, t.serverID, t.toolName, args)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	output := formatMCPCallToolResult(res)
	return Result{OK: !res.IsError, Output: output}, nil
}

func formatMCPCallToolResult(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	if res.StructuredContent != nil {
		if data, err := json.MarshalIndent(res.StructuredContent, "", "  "); err == nil {
			return string(data)
		}
	}
	var parts []string
	for _, content := range res.Content {
		switch c := content.(type) {
		case *mcp.TextContent:
			if c.Text != "" {
				parts = append(parts, c.Text)
			}
		default:
			if data, err := json.Marshal(c); err == nil {
				parts = append(parts, string(data))
			}
		}
	}
	return strings.Join(parts, "\n")
}
