package tools

import (
	"context"
	"encoding/json"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// Registry holds built-in tools for a workspace.
type Registry struct {
	workspace Workspace
	byName    map[string]Tool
	order     []Tool
}

// NewRegistry returns tools for the parent agent using ParentSurface policy.
func NewRegistry(workspaceRoot string, opts ...RegistryOptions) *Registry {
	var opt RegistryOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	delegate := opt.Sessions != nil && opt.ACP.Enabled && opt.ACP.CommandAvailable()
	return newRegistryWithSurface(workspaceRoot, ParentSurface(delegate), opt)
}

// NewInboxProcessRegistry builds the unattended tool surface for inbox reply
// internalization. It can read, edit, run, and draft skills, but it cannot
// spawn children or write/promote live skills. Plan mode still wins.
func NewInboxProcessRegistry(workspaceRoot string, opt RegistryOptions) *Registry {
	return newRegistryWithSurface(workspaceRoot, InboxProcessSurface(), opt)
}

// NewSubagentRegistry returns tools for in-process subagent workers via ToolSurface.
func NewSubagentRegistry(workspaceRoot string, skillReg *skills.Registry, mode SubagentMode) *Registry {
	opt := RegistryOptions{Skills: skillReg}
	return newRegistryWithSurface(workspaceRoot, SurfaceForMode(mode), opt)
}

// NewWikiReviewRegistry is the hidden wiki compile surface: read tools,
// web_fetch, and file writes that reject paths outside the wiki.
func NewWikiReviewRegistry(workspaceRoot string) *Registry {
	ws := Workspace{Root: workspaceRoot}
	r := &Registry{workspace: ws, byName: make(map[string]Tool)}
	r.Add(ReadFile{Workspace: ws})
	r.Add(ListDir{Workspace: ws})
	r.Add(Glob{Workspace: ws})
	r.Add(Grep{Workspace: ws})
	r.Add(WebFetch{})
	r.Add(WikiEditFile{Workspace: ws})
	r.Add(WikiWriteFile{Workspace: ws})
	return r
}

// NewSkillReviewRegistry is the hidden skill-review surface. It is not a
// research or coding registry: read tools, load_skill, read_skill_file, and
// the guarded write_skill only. Read does not include web tools here.
func NewSkillReviewRegistry(workspaceRoot string, skillReg *skills.Registry, used func(string)) *Registry {
	ws := Workspace{Root: workspaceRoot}
	r := &Registry{workspace: ws, byName: make(map[string]Tool)}
	r.Add(ReadFile{Workspace: ws})
	r.Add(ListDir{Workspace: ws})
	r.Add(Glob{Workspace: ws})
	r.Add(Grep{Workspace: ws})
	if skillReg != nil {
		r.Add(LoadSkill{Skills: skillReg, Used: used})
		r.Add(ReadSkillFile{Skills: skillReg})
	}
	r.Add(ReviewWriteSkill{})
	return r
}

// NewCuratorMergeRegistry is the skill-review surface plus a merge report tool.
func NewCuratorMergeRegistry(workspaceRoot string, skillReg *skills.Registry) *Registry {
	r := NewSkillReviewRegistry(workspaceRoot, skillReg, nil)
	r.Add(ReportMergedSkills{})
	return r
}

func newRegistryWithSurface(workspaceRoot string, surface ToolSurface, opt RegistryOptions) *Registry {
	// Plan mode overrides the default parent surface with the read-only
	// allowlist. Auto (and empty) registries keep the caller-provided surface.
	if SurfaceForPlan(opt.AgentMode) {
		surface = PlanSurface()
	}
	ws := Workspace{Root: workspaceRoot}
	r := &Registry{workspace: ws, byName: make(map[string]Tool)}
	r.addFileTools(ws, surface)
	r.addNetworkTools(ws, surface, opt)
	r.addSkillTools(surface, opt)
	r.addChildTools(ws, surface, opt)
	r.addServiceTools(workspaceRoot, surface, opt)
	return r
}

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
	res, err := t.Execute(ctx, input)
	if toolkit.IsJSONSchemaError(err) {
		return toolkit.InvalidToolInputResult(name, input, err), nil
	}
	return res, err
}

// Add registers a tool. Later tools with the same name replace earlier ones
// in the lookup map but remain in registration order.
func (r *Registry) Add(t Tool) {
	if r == nil || t == nil {
		return
	}
	spec := t.Spec()
	r.byName[spec.Name] = t
	r.order = append(r.order, t)
}

// Has reports whether a tool is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.byName[name]
	return ok
}
