package mcp

import (
	"context"
	"encoding/json"
	"strings"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

type useMCPToolTool struct {
	mgr *mcppkg.Manager
}

func (useMCPToolTool) Spec() ToolSpec {
	return ToolSpec{
		Name: useMCPToolName,
		Description: "Call one MCP tool by the name returned from search_mcp_tools. " +
			"Pass that tool's arguments in arguments. Do not call MCP tools as native tools.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"name":{"type":"string","description":"Qualified tool name from search_mcp_tools, such as mcp_server_tool"},
				"arguments":{"type":"object","description":"Arguments for the MCP tool. Use {} when it takes none."}
			},
			"required":["name"]
		}`),
	}
}

func (t useMCPToolTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.mgr == nil {
		return Result{OK: false, Output: "MCP is not configured"}, nil
	}
	var in struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: "invalid tool input: " + err.Error()}, nil
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Result{OK: false, Output: "name is required. Search with search_mcp_tools first."}, nil
	}
	entry, ok := findCatalogEntry(t.mgr.Catalog(), name)
	if !ok {
		return Result{OK: false, Output: "unknown MCP tool " + name + ". Call search_mcp_tools and pass the returned name."}, nil
	}
	args, err := decodeArguments(in.Arguments)
	if err != nil {
		return Result{OK: false, Output: "invalid arguments: " + err.Error()}, nil
	}
	return mcpTool{
		serverID: entry.ServerID,
		toolName: entry.ToolName,
		mgr:      t.mgr,
	}.Execute(ctx, mustArgs(args))
}

func findCatalogEntry(entries []mcppkg.CatalogEntry, name string) (mcppkg.CatalogEntry, bool) {
	q := strings.ToLower(name)
	var bare mcppkg.CatalogEntry
	bareCount := 0
	for _, entry := range entries {
		if strings.ToLower(entry.RegistryName) == q {
			return entry, true
		}
		if strings.ToLower(entry.ToolName) == q {
			bare = entry
			bareCount++
		}
	}
	// A bare name is safe only when one server exposes it. Otherwise the
	// caller must pass the qualified name from search.
	if bareCount == 1 {
		return bare, true
	}
	return mcppkg.CatalogEntry{}, false
}

func decodeArguments(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	return args, nil
}

func mustArgs(args map[string]any) json.RawMessage {
	if args == nil {
		return nil
	}
	data, err := json.Marshal(args)
	if err != nil {
		return nil
	}
	return data
}
