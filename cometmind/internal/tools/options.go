package tools

import (
	"context"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/subagent"
)

// AgentLoopRunner is the subset of the agent runner used by subagent tools.
type AgentLoopRunner interface {
	Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error
}

// ChildRunnerFactory builds a runner for a general subagent child session.
type ChildRunnerFactory func(child session.Session, workspaceRoot string, maxSteps int) (AgentLoopRunner, error)

// RegistryOptions configures optional registry capabilities.
type RegistryOptions struct {
	Sessions       *session.Service
	ACP            acp.Config
	ACPMgr         *acp.SessionManager
	Skills         *skills.Registry
	MCP            *mcppkg.Manager
	Orchestrator   *subagent.Orchestrator
	RunnerFactory  ChildRunnerFactory
	SubagentConfig SubagentToolConfig
}

// SubagentToolConfig holds limits passed into subagent tools.
type SubagentToolConfig struct {
	GeneralMaxSteps int
	WaitTimeoutSec  int
}
