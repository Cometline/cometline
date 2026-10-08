package tools

import (
	"github.com/Cometline/cometline/cometmind/internal/acp"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/generation"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/scheduler"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
	childagent "github.com/Cometline/cometline/cometmind/internal/tools/subagent"
)

// Agent loop types live with the subagent tools. These aliases keep the
// registry and runtime call sites stable.
type (
	AgentLoopRunner    = childagent.AgentLoopRunner
	SubagentMode       = childagent.SubagentMode
	ChildRunnerFactory = childagent.ChildRunnerFactory
	SubagentToolConfig = childagent.SubagentToolConfig
)

const (
	// SubagentModeResearch is read-only exploration (no edit/write/run).
	SubagentModeResearch = childagent.SubagentModeResearch
	// SubagentModeCoding allows edit/write/run_command for native coding work.
	SubagentModeCoding = childagent.SubagentModeCoding
)

// RegistryOptions configures optional registry capabilities.
type RegistryOptions struct {
	Sessions           session.ChildSessionReader
	ACP                acp.Config
	ACPMgr             *acp.SessionManager
	Skills             *skills.Registry
	SkillUsed          func(name string)
	MCP                *mcppkg.Manager
	Orchestrator       *subagent.Orchestrator
	RunnerFactory      ChildRunnerFactory
	SubagentConfig     SubagentToolConfig
	Jobs               *jobs.Service
	Scheduler          *scheduler.Service
	Inbox              *inbox.Service
	Memory             *memory.Service
	MemoryEvents       *event.Hub
	SessionID          string
	JobPlatform        string
	JobSourceChannelID string
	// AgentMode selects the tool surface for parent-agent registries.
	// Empty or auto keeps the full ParentSurface; plan applies the read-only
	// PlanSurface with research-only subagent spawning.
	AgentMode          session.AgentMode
	BrowserSearchURL   string
	BrowserSearchToken string
	ScreenCaptureURL   string
	ScreenCaptureToken string
	SettingsRuntime    SettingsRuntime
	AssistantMedia     session.AssistantMediaAppender
	ReadyMedia         session.ReadyMediaReader
	GenerationResolver func(kind string) generation.Binding
}
