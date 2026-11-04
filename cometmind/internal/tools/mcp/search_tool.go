package mcp

import (
	"context"
	"encoding/json"
	"strings"

	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
)

const searchMCPToolsName = "search_mcp_tools"
const useMCPToolName = "use_mcp_tool"

type searchMCPToolsTool struct {
	mgr *mcppkg.Manager
}

func (searchMCPToolsTool) Spec() ToolSpec {
	return ToolSpec{
		Name: searchMCPToolsName,
		Description: "Search connected MCP tools by keyword and return their input schemas. " +
			"Call this before use_mcp_tool. MCP tools are not on the native tool list. " +
			"An exact tool name match wins over keyword ranking.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"query":{"type":"string","description":"Tool name, server name, or keywords"},
				"limit":{"type":"integer","description":"Maximum hits to return. Default 5, maximum 8."}
			},
			"required":["query"]
		}`),
	}
}

func (t searchMCPToolsTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	if t.mgr == nil {
		return Result{OK: false, Output: "MCP is not configured"}, nil
	}
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: "invalid tool input: " + err.Error()}, nil
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return Result{OK: false, Output: "query is required"}, nil
	}
	catalog := t.mgr.Catalog()
	hits := searchCatalog(catalog, query, in.Limit)
	return Result{OK: true, Output: formatSearchResult(catalog, hits)}, nil
}

type searchResult struct {
	Results []searchResultHit `json:"results"`
	Hidden  int               `json:"total_hidden_tools"`
	Note    string            `json:"note,omitempty"`
}

type searchResultHit struct {
	Name        string          `json:"name"`
	Server      string          `json:"server"`
	Description string          `json:"description"`
	Parameters  []string        `json:"parameters"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func formatSearchResult(catalog []mcppkg.CatalogEntry, hits []searchHit) string {
	out := searchResult{
		Results: make([]searchResultHit, 0, len(hits)),
		Hidden:  len(catalog) - len(hits),
	}
	if len(catalog) == 0 {
		out.Note = "No MCP tools are connected."
	}
	for _, hit := range hits {
		schema := hit.entry.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Results = append(out.Results, searchResultHit{
			Name:        hit.entry.RegistryName,
			Server:      hit.entry.ServerID,
			Description: truncateDescription(hit.entry.Description),
			Parameters:  parameterNames(schema),
			InputSchema: schema,
		})
	}
	if out.Hidden < 0 {
		out.Hidden = 0
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return `{"results":[],"total_hidden_tools":0}`
	}
	return string(data)
}
