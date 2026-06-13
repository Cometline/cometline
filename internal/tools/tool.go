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

// Tool is a built-in capability exposed to the LLM.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, workspaceRoot string, input json.RawMessage) (Result, error)
}
