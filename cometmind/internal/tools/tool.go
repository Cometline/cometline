package tools

import (
	"context"
	"encoding/json"

	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// Result is the structured outcome of a local tool execution.
type Result = toolkit.Result

// ToolSpec is the static metadata exposed to the LLM for a tool.
type ToolSpec = toolkit.ToolSpec

// Tool is a built-in capability exposed to the LLM.
type Tool = toolkit.Tool

// ProgressFn emits runtime events during long-running tool execution.
type ProgressFn = toolkit.ProgressFn

// WithToolSession attaches the active CometMind session id to the tool context.
func WithToolSession(ctx context.Context, sessionID string) context.Context {
	return toolkit.WithToolSession(ctx, sessionID)
}

// WithProgress attaches a callback for streaming tool progress to the parent turn.
func WithProgress(ctx context.Context, fn ProgressFn) context.Context {
	return toolkit.WithProgress(ctx, fn)
}

// IsInvalidToolInput reports schema/JSON argument failures, including wrapped
// Execute errors and the recoverable Result produced by the registry.
func IsInvalidToolInput(res Result, err error) bool {
	return toolkit.IsInvalidToolInput(res, err)
}

// IsCompleteJSONObject reports whether input is a finished JSON object.
func IsCompleteJSONObject(input json.RawMessage) bool {
	return toolkit.IsCompleteJSONObject(input)
}
