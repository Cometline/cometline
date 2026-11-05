package agent

import (
	"encoding/json"

	"github.com/Cometline/cometline/cometmind/internal/hooks"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

// HooksWorkspace returns the workspace root used for hook subprocess cwd, when hooks are configured.
func (r *Runner) HooksWorkspace() string {
	if r == nil || r.Hooks == nil {
		return ""
	}
	return r.Hooks.WorkspacePath()
}

// hookToolEnv builds the subprocess hook stdin payload for one tool call.
// AgentTurn.ID is the persisted session id; CometMind runs one in-flight agent
// loop per session row, so turn_id matches session_id until distinct turn ids exist.
func (r *Runner) hookToolEnv(
	s *turnState,
	turn session.AgentTurn,
	toolName string,
	toolInput json.RawMessage,
	toolOutput string,
	toolOK bool,
) hooks.ToolEnv {
	return hooks.ToolEnv{
		SessionID:     turn.ID,
		TurnID:        turn.ID,
		WorkspacePath: r.HooksWorkspace(),
		ToolName:      toolName,
		ToolInput:     toolInput,
		ModelID:       turn.ModelID,
		ProviderID:    turn.ProviderID,
		JobID:         s.jobTracker.JobID,
		ToolOutput:    toolOutput,
		ToolOK:        toolOK,
	}
}
