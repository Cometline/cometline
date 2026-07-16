package tools

import (
	"context"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/jobs"
	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/scheduler"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/subagent"
)

// AgentLoopRunner is the subset of the agent runner used by subagent tools.
type AgentLoopRunner interface {
	Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error
}

// SubagentMode selects the tool surface for an in-process child agent.
type SubagentMode string

const (
	// SubagentModeResearch is read-only exploration (no edit/write/run).
	SubagentModeResearch SubagentMode = "research"
	// SubagentModeCoding allows edit/write/run_command for native coding work.
	SubagentModeCoding SubagentMode = "coding"
)

// ChildRunnerFactory builds a runner for an in-process subagent child session.
type ChildRunnerFactory func(child session.Session, workspaceRoot string, maxSteps int, mode SubagentMode) (AgentLoopRunner, error)

// RegistryOptions configures optional registry capabilities.
type RegistryOptions struct {
	Sessions           session.ChildSessionReader
	ACP                acp.Config
	ACPMgr             *acp.SessionManager
	Skills             *skills.Registry
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
	BrowserSearchURL   string
	BrowserSearchToken string
	SettingsRuntime    SettingsRuntime
}

// SubagentToolConfig holds limits passed into subagent tools.
type SubagentToolConfig struct {
	GeneralMaxSteps int
}
