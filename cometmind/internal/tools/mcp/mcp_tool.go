package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpCallToolTimeout bounds a single MCP tool call, mirroring the timeout
// pattern used by other built-in tools (grep.go, runcommand.go, webfetch.go).
// Without this, a zombie MCP session (see internal/mcp keepalive handling)
// could otherwise hang for the full duration of the parent turn's context.
const mcpCallToolTimeout = 60 * time.Second

type mcpTool struct {
	serverID string
	toolName string
	mgr      *mcppkg.Manager
}

// AddMCPTools registers MCP management tools and the two discovery tools.
// Remote tool schemas stay off the native list. The model searches for them
// and calls them through use_mcp_tool.
func AddMCPTools(add func(Tool), mgr *mcppkg.Manager) {
	if add == nil || mgr == nil {
		return
	}
	add(listMCPServersTool{mgr: mgr})
	add(reconnectMCPServerTool{mgr: mgr})
	add(searchMCPToolsTool{mgr: mgr})
	add(useMCPToolTool{mgr: mgr})
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
