package tools

import (
	"context"
	"encoding/json"

	cometsdk "github.com/cometline/comet-sdk"
)

// Registry holds built-in tools for a workspace.
type Registry struct {
	byName map[string]Tool
	order  []Tool
}

// NewRegistry returns read/list/write/run tools scoped to the workspace root on disk.
func NewRegistry(workspaceRoot string) *Registry {
	r := &Registry{byName: make(map[string]Tool)}
	add := func(t Tool) {
		r.byName[t.Name()] = t
		r.order = append(r.order, t)
	}

	ws := workspaceRoot
	add(ReadFile{Root: ws})
	add(WriteFile{Root: ws})
	add(ListDir{Root: ws})
	add(RunCommand{Root: ws})

	return r
}

// CometSDK returns tool schemas for the LLM request.
func (r *Registry) CometSDK() []cometsdk.Tool {
	out := make([]cometsdk.Tool, 0, len(r.order))
	for _, t := range r.order {
		out = append(out, cometsdk.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return out
}

// Execute runs a tool by name.
func (r *Registry) Execute(ctx context.Context, workspaceRoot, name string, input json.RawMessage) (Result, error) {
	t, ok := r.byName[name]
	if !ok {
		return Result{OK: false, Output: "unknown tool: " + name}, nil
	}
	return t.Execute(ctx, workspaceRoot, input)
}
