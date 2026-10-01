package subagent

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
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

// SubagentToolConfig holds limits passed into subagent tools.
type SubagentToolConfig struct {
	GeneralMaxSteps int
}

// Session kind and display label constants. The tools package re-exports them.
const (
	SessionKindResearch = "general"
	SessionKindCoding   = "coding"
	SessionKindACP      = "acp"

	AgentLabelResearch = "cometmind"
	AgentLabelCoding   = "cometmind-coding"
)

// SessionKindForMode is the persisted subagent_kind value.
func SessionKindForMode(mode SubagentMode) string {
	if mode == SubagentModeCoding {
		return SessionKindCoding
	}
	return SessionKindResearch
}

// AgentLabelForMode is the SSE/display agent name for in-process children.
func AgentLabelForMode(mode SubagentMode) string {
	if mode == SubagentModeCoding {
		return AgentLabelCoding
	}
	return AgentLabelResearch
}

// AgentLabelForSessionKind maps persisted kind to a display label.
func AgentLabelForSessionKind(kind string) string {
	switch kind {
	case SessionKindCoding:
		return AgentLabelCoding
	case SessionKindResearch, "":
		return AgentLabelResearch
	default:
		// External harness kinds (acp) keep harness-specific names elsewhere.
		return ""
	}
}
