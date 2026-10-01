package tools

import "github.com/Cometline/cometline/cometmind/internal/generation"

// addFileTools registers file tools in the stable CometSDK spec order:
// read, then edit/write, then list/glob/grep, then run.
func (r *Registry) addFileTools(ws Workspace, surface ToolSurface) {
	if surface.Read {
		r.Add(ReadFile{Workspace: ws})
	}
	if surface.Edit {
		r.Add(EditFile{Workspace: ws})
		r.Add(WriteFile{Workspace: ws})
	}
	if surface.Read {
		r.Add(ListDir{Workspace: ws})
		r.Add(Glob{Workspace: ws})
		r.Add(Grep{Workspace: ws})
	}
	if surface.Run {
		r.Add(RunCommand{Workspace: ws})
	}
}

func (r *Registry) addNetworkTools(ws Workspace, surface ToolSurface, opt RegistryOptions) {
	if !surface.Read {
		return
	}
	r.Add(WebFetch{})
	r.Add(WebSearch{Endpoint: opt.BrowserSearchURL, Token: opt.BrowserSearchToken})
	r.Add(PresentImage{Workspace: ws, Media: opt.AssistantMedia})
	r.Add(PresentImageURL{Media: opt.AssistantMedia})
	r.Add(CaptureScreenshot{
		Endpoint: opt.ScreenCaptureURL,
		Token:    opt.ScreenCaptureToken,
		Media:    opt.AssistantMedia,
	})
	r.Add(ListCaptureTargets{
		Endpoint: opt.ScreenCaptureURL,
		Token:    opt.ScreenCaptureToken,
	})
}

func (r *Registry) addSkillTools(surface ToolSurface, opt RegistryOptions) {
	if !surface.Skills || opt.Skills == nil {
		return
	}
	r.Add(LoadSkill{Skills: opt.Skills})
	r.Add(ReadSkillFile{Skills: opt.Skills})
	if surface.SkillDraft {
		r.Add(WriteSkillDraft{})
		r.Add(ListSkillDrafts{})
		r.Add(ReadSkillDraft{})
	}
	if surface.SkillMut {
		r.Add(WriteSkill{})
		r.Add(PromoteSkillDraft{})
	}
}

func (r *Registry) addChildTools(ws Workspace, surface ToolSurface, opt RegistryOptions) {
	if surface.Delegate && opt.Sessions != nil {
		r.Add(DelegateCodingTask{
			Workspace:    ws,
			Sessions:     opt.Sessions,
			ACP:          opt.ACP,
			ACPMgr:       opt.ACPMgr,
			Orchestrator: opt.Orchestrator,
		})
	}
	if surface.Spawn && opt.Sessions != nil && opt.Orchestrator != nil && opt.RunnerFactory != nil {
		r.Add(SpawnGeneralAgent{
			Workspace:      ws,
			Sessions:       opt.Sessions,
			Orchestrator:   opt.Orchestrator,
			RunnerFactory:  opt.RunnerFactory,
			SubagentConfig: opt.SubagentConfig,
			AgentMode:      opt.AgentMode,
		})
		r.Add(WaitSubagents{
			Sessions:     opt.Sessions,
			Orchestrator: opt.Orchestrator,
		})
	}
}

func (r *Registry) addServiceTools(workspaceRoot string, surface ToolSurface, opt RegistryOptions) {
	if surface.MCP && opt.MCP != nil {
		AddMCPTools(r.Add, opt.MCP)
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
		r.Add(RecallTaskOutcome{Memory: opt.Memory})
		r.Add(ListMemories{Memory: opt.Memory})
		r.Add(SearchMemories{Memory: opt.Memory})
		r.Add(CreateMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
		r.Add(UpdateMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
		r.Add(DeleteMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
	}
	if surface.Settings {
		AddSettingsTools(r.Add, opt.SettingsRuntime)
	}
	r.addGenerateTools(surface, opt)
	if surface.Inbox && opt.Inbox != nil {
		RegisterInboxLeaveTool(r, InboxDeps{
			Inbox:     opt.Inbox,
			Jobs:      opt.Jobs,
			Sessions:  opt.Sessions,
			Events:    opt.MemoryEvents,
			SessionID: opt.SessionID,
		})
	}
}

func (r *Registry) addGenerateTools(surface ToolSurface, opt RegistryOptions) {
	if !surface.Generate {
		return
	}
	r.Add(GenerateImage{
		Media:    opt.AssistantMedia,
		Resolver: generationBinding(opt, generation.KindImage),
	})
	r.Add(GenerateVideo{
		Media:    opt.ReadyMedia,
		Appender: opt.AssistantMedia,
		Resolver: generationBinding(opt, generation.KindVideo),
	})
}

func generationBinding(opt RegistryOptions, kind string) func() generation.Binding {
	return func() generation.Binding {
		if opt.GenerationResolver == nil {
			return generation.Binding{}
		}
		return opt.GenerationResolver(kind)
	}
}
