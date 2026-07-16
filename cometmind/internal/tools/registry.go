package tools

import (
	"context"
	"encoding/json"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/skills"
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

// NewSubagentRegistry returns tools for in-process subagent workers via ToolSurface.
func NewSubagentRegistry(workspaceRoot string, skillReg *skills.Registry, mode SubagentMode) *Registry {
	opt := RegistryOptions{Skills: skillReg}
	return newRegistryWithSurface(workspaceRoot, SurfaceForMode(mode), opt)
}

func newRegistryWithSurface(workspaceRoot string, surface ToolSurface, opt RegistryOptions) *Registry {
	ws := Workspace{Root: workspaceRoot}
	r := &Registry{workspace: ws, byName: make(map[string]Tool)}
	add := func(t Tool) {
		spec := t.Spec()
		r.byName[spec.Name] = t
		r.order = append(r.order, t)
	}

	if surface.Read {
		add(ReadFile{Workspace: ws})
	}
	if surface.Edit {
		add(EditFile{Workspace: ws})
		add(WriteFile{Workspace: ws})
	}
	if surface.Read {
		add(ListDir{Workspace: ws})
		add(Glob{Workspace: ws})
		add(Grep{Workspace: ws})
	}
	if surface.Run {
		add(RunCommand{Workspace: ws})
	}
	if surface.Read {
		add(WebFetch{})
		add(WebSearch{Endpoint: opt.BrowserSearchURL, Token: opt.BrowserSearchToken})
	}
	if surface.Skills && opt.Skills != nil {
		add(LoadSkill{Skills: opt.Skills})
		add(ReadSkillFile{Skills: opt.Skills})
		if surface.SkillMut {
			add(WriteSkill{})
			add(WriteSkillDraft{})
			add(ListSkillDrafts{})
			add(ReadSkillDraft{})
			add(PromoteSkillDraft{})
		}
	}
	if surface.Delegate && opt.Sessions != nil {
		add(DelegateCodingTask{
			Workspace:    ws,
			Sessions:     opt.Sessions,
			ACP:          opt.ACP,
			ACPMgr:       opt.ACPMgr,
			Orchestrator: opt.Orchestrator,
		})
	}
	if surface.Spawn && opt.Sessions != nil && opt.Orchestrator != nil && opt.RunnerFactory != nil {
		add(SpawnGeneralAgent{
			Workspace:      ws,
			Sessions:       opt.Sessions,
			Orchestrator:   opt.Orchestrator,
			RunnerFactory:  opt.RunnerFactory,
			SubagentConfig: opt.SubagentConfig,
		})
		add(WaitSubagents{
			Sessions:     opt.Sessions,
			Orchestrator: opt.Orchestrator,
		})
	}
	if surface.MCP && opt.MCP != nil {
		add(listMCPServersTool{mgr: opt.MCP})
		add(reconnectMCPServerTool{mgr: opt.MCP})
		for _, tool := range mcpToolsFromManager(opt.MCP) {
			add(tool)
		}
	}
	if surface.Jobs && (opt.Jobs != nil || opt.Scheduler != nil) {
		RegisterJobTools(r, JobsDeps{
			Service:              opt.Jobs,
			Scheduler:            opt.Scheduler,
			SessionID:            opt.SessionID,
			SessionWorkspacePath: workspaceRoot,
			SourcePlatform:       opt.JobPlatform,
			SourceChannelID:      opt.JobSourceChannelID,
		})
	}
	if surface.Memory && opt.Memory != nil {
		add(RecallTaskOutcome{Memory: opt.Memory})
		add(ListMemories{Memory: opt.Memory})
		add(SearchMemories{Memory: opt.Memory})
		add(CreateMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
		add(UpdateMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
		add(DeleteMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
	}
	if surface.Settings {
		add(listSettingsTool{})
		add(getSettingsTool{})
		add(patchSettingsTool{Runtime: opt.SettingsRuntime})
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

// Has reports whether a tool is registered.
func (r *Registry) Has(name string) bool {
	_, ok := r.byName[name]
	return ok
}
