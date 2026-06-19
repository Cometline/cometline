package tools

import (
	"context"
	"encoding/json"
)

// Result is the structured outcome of a local tool execution.
type Result struct {
	OK       bool
	Output   string
	ExitCode *int
}

// ToolSpec is the static metadata exposed to the LLM for a tool.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// Tool is a built-in capability exposed to the LLM. Implementations capture
// their Workspace at construction time so Execute only needs runtime input.
type Tool interface {
	Spec() ToolSpec
	Execute(ctx context.Context, input json.RawMessage) (Result, error)
}
