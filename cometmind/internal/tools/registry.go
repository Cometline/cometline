package tools

import (
	"context"
	"encoding/json"

	cometsdk "github.com/cometline/comet-sdk"
)

// Registry holds built-in tools for a workspace.
type Registry struct {
	workspace Workspace
	byName    map[string]Tool
	order     []Tool
}

// NewRegistry returns read/list/write/run tools scoped to the workspace root on disk.
func NewRegistry(workspaceRoot string, opts ...RegistryOptions) *Registry {
	ws := Workspace{Root: workspaceRoot}
	r := &Registry{workspace: ws, byName: make(map[string]Tool)}
	var opt RegistryOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	add := func(t Tool) {
		spec := t.Spec()
		r.byName[spec.Name] = t
		r.order = append(r.order, t)
	}

	add(ReadFile{Workspace: ws})
	add(WriteFile{Workspace: ws})
	add(ListDir{Workspace: ws})
	add(RunCommand{Workspace: ws})
	add(WebFetch{})
	if opt.Skills != nil {
		add(LoadSkill{Skills: opt.Skills})
		add(ReadSkillFile{Skills: opt.Skills})
		add(WriteSkill{})
	}
	if opt.Sessions != nil {
		add(DelegateCodingTask{
			Workspace: ws,
			Sessions:  opt.Sessions,
			ACP:       opt.ACP,
			ACPMgr:    opt.ACPMgr,
		})
	}

	return r
}

// Workspace returns the sandbox the registry's tools operate in.
func (r *Registry) Workspace() Workspace { return r.workspace }

// CometSDK returns tool schemas for the LLM request.
func (r *Registry) CometSDK() []cometsdk.Tool {
	out := make([]cometsdk.Tool, 0, len(r.order))
	for _, t := range r.order {
		spec := t.Spec()
		out = append(out, cometsdk.Tool{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return out
}

// Execute runs a tool by name.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	t, ok := r.byName[name]
	if !ok {
		return Result{OK: false, Output: "unknown tool: " + name}, nil
	}
	return t.Execute(ctx, input)
}
