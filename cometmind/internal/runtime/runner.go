package runtime

import (
	"os"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/agent"
	"github.com/Cometline/cometline/cometmind/internal/generation"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/provider"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

// RunnerOptions controls how a runner is assembled. Most callers should use
// RunnerFor, RunnerForMode, SubagentRunnerFor, or RunnerForGateway instead.
type RunnerOptions struct {
	MaxSteps        int
	Platform        string
	SourceChannelID string
	Subagent        bool
	SubagentMode    tools.SubagentMode
	AgentMode       session.AgentMode
}

// RunnerFor returns an agent runner wired for a specific session and workspace
// using the session's persisted agent mode.
func (r *Runtime) RunnerFor(sess session.Session, workspacePath string) (*agent.Runner, error) {
	mode, err := session.ParseAgentMode(sess.AgentMode)
	if err != nil {
		return nil, err
	}
	return r.runnerFor(sess, workspacePath, RunnerOptions{AgentMode: mode})
}

// RunnerForMode returns an agent runner wired for a specific session, workspace,
// and explicit per-turn agent mode.
func (r *Runtime) RunnerForMode(sess session.Session, workspacePath string, mode session.AgentMode) (*agent.Runner, error) {
	return r.runnerFor(sess, workspacePath, RunnerOptions{AgentMode: mode})
}

// SubagentRunnerFor returns a runner for an in-process subagent child session.
// mode selects research (read-only) vs coding (edit/write/run) tool surface.
func (r *Runtime) SubagentRunnerFor(child session.Session, workspacePath string, maxSteps int, mode tools.SubagentMode) (*agent.Runner, error) {
	return r.runnerFor(child, workspacePath, RunnerOptions{
		MaxSteps:     maxSteps,
		Subagent:     true,
		SubagentMode: mode,
	})
}

// RunnerForGateway is like RunnerFor but tags job tool metadata for a gateway channel.
func (r *Runtime) RunnerForGateway(sess session.Session, workspacePath, platform, sourceChannelID string) (*agent.Runner, error) {
	return r.runnerFor(sess, workspacePath, RunnerOptions{Platform: platform, SourceChannelID: sourceChannelID})
}

func (r *Runtime) runnerFor(sess session.Session, workspacePath string, opts RunnerOptions) (*agent.Runner, error) {
	p, err := r.ProviderForSession(sess)
	if err != nil {
		return nil, err
	}
	skillRegistry := r.SkillsForWorkspace(workspacePath)

	maxSteps := opts.MaxSteps
	if maxSteps == 0 {
		maxSteps = r.Config.MaxSteps
	}
	platform := opts.Platform
	if platform == "" {
		platform = jobs.PlatformDesktop
	}

	var registry *tools.Registry
	var agentMode session.AgentMode
	if opts.Subagent {
		mode := opts.SubagentMode
		if mode == "" {
			mode = tools.SubagentModeResearch
		}
		registry = tools.NewSubagentRegistry(workspacePath, &skillRegistry, mode)
	} else {
		agentMode = opts.AgentMode
		if agentMode == "" {
			var err error
			agentMode, err = session.ParseAgentMode(sess.AgentMode)
			if err != nil {
				return nil, err
			}
		}
		registry = r.toolRegistryWithJobMeta(workspacePath, skillRegistry, sess.ID, platform, opts.SourceChannelID, agentMode)
	}

	runner := &agent.Runner{
		Config:               r.Config,
		Provider:             p,
		Sessions:             r.Sessions,
		Memory:               r.Memory,
		Registry:             registry,
		Jobs:                 r.Jobs,
		MaxSteps:             maxSteps,
		SystemPrompt:         r.SystemPrompt,
		AgentMode:            agentMode,
		SkillIndex:           skillRegistry.PromptIndex(),
		SubagentOrchestrator: r.subagentOrchestratorForRunner(opts.Subagent),
		MemorySem:            r.memorySem,
		Compatibility:        r.Compatibility,
		CompatibilityScope: cometsdk.CapabilityScope{
			ProviderID: sess.ProviderID,
			Endpoint:   provider.CompatibilityEndpoint(r.Config, sess.ProviderID),
			ModelID:    sess.ModelID,
		},
	}
	if !opts.Subagent {
		runner.JobIndex = tools.JobPromptIndex(workspacePath, platform)
		runner.Compactor = &agent.ContextCompactor{Sessions: r.Sessions, Config: r.Config}
	}
	return runner, nil
}

func (r *Runtime) subagentOrchestratorForRunner(isSubagent bool) *subagent.Orchestrator {
	if isSubagent {
		return nil
	}
	return r.SubagentOrchestrator()
}

func (r *Runtime) toolRegistryWithJobMeta(workspacePath string, skillRegistry skills.Registry, sessionID, platform, sourceChannelID string, mode session.AgentMode) *tools.Registry {
	return tools.NewRegistry(workspacePath, r.toolRegistryOptions(skillRegistry, sessionID, platform, sourceChannelID, mode))
}

func (r *Runtime) toolRegistryOptions(skillRegistry skills.Registry, sessionID, platform, sourceChannelID string, mode session.AgentMode) tools.RegistryOptions {
	sub := r.Config.EffectiveSubagentSettings()
	return tools.RegistryOptions{
		Sessions:       r.Sessions,
		AssistantMedia: r.Sessions,
		ReadyMedia:     r.Sessions,
		GenerationResolver: func(kind string) generation.Binding {
			if r.Config == nil {
				return generation.Binding{}
			}
			return r.Config.GenerationBinding(kind)
		},
		ACP:                r.Config.ACPSettings(),
		ACPMgr:             r.ACPManager(),
		Skills:             &skillRegistry,
		MCP:                r.mcpMgr,
		Orchestrator:       r.SubagentOrchestrator(),
		Jobs:               r.Jobs,
		Scheduler:          r.Scheduler,
		Inbox:              r.Inbox,
		Memory:             r.Memory,
		MemoryEvents:       r.Events,
		SessionID:          sessionID,
		JobPlatform:        platform,
		JobSourceChannelID: sourceChannelID,
		AgentMode:          mode,
		BrowserSearchURL:   os.Getenv("COMETLINE_BROWSER_SEARCH_URL"),
		BrowserSearchToken: os.Getenv("COMETLINE_BROWSER_SEARCH_TOKEN"),
		ScreenCaptureURL:   os.Getenv("COMETLINE_SCREEN_CAPTURE_URL"),
		ScreenCaptureToken: os.Getenv("COMETLINE_SCREEN_CAPTURE_TOKEN"),
		SettingsRuntime:    r,
		RunnerFactory: func(child session.Session, workspaceRoot string, maxSteps int, mode tools.SubagentMode) (tools.AgentLoopRunner, error) {
			return r.SubagentRunnerFor(child, workspaceRoot, maxSteps, mode)
		},
		SubagentConfig: tools.SubagentToolConfig{
			GeneralMaxSteps: sub.GeneralMaxSteps,
		},
	}
}

// RunnerForInbox builds a runner for an inbox internalization session.
// The caller supplies the registry; it is the parent surface, not a reduced tool list.
func (r *Runtime) RunnerForInbox(sess session.Session, workspacePath string, registry *tools.Registry, maxSteps int) (*agent.Runner, error) {
	p, err := r.ProviderForSession(sess)
	if err != nil {
		return nil, err
	}
	if maxSteps <= 0 {
		maxSteps = 8
	}
	return &agent.Runner{
		Config:       r.Config,
		Provider:     p,
		Sessions:     r.Sessions,
		Memory:       r.Memory,
		Registry:     registry,
		Jobs:         r.Jobs,
		MaxSteps:     maxSteps,
		SystemPrompt: "You process a user reply to an inbox note. Save durable memory when the reply states a lasting preference or fact. Do real work the reply asks for with the tools you have. Draft reusable workflows with write_skill_draft and leave them for human review. This session cannot spawn subagents or write or promote live skills. Prefer no-op over noisy memories or invented work.",
		SkillIndex:   r.SkillsForWorkspace(workspacePath).PromptIndex(),
		JobIndex:     tools.JobPromptIndex(workspacePath, jobs.PlatformDesktop),
		MemorySem:    r.memorySem,
	}, nil
}
