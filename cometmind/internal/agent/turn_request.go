package agent

import (
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

// reasoningEffortFor resolves the per-turn reasoning effort override. Empty
// means the provider default; no runtime-wide default is applied.
func (r *Runner) reasoningEffortFor(turn session.AgentTurn) string {
	return strings.TrimSpace(turn.ReasoningEffort)
}

func toolResultBytes(messages []cometsdk.Message) int {
	total := 0
	for _, m := range messages {
		for _, block := range m.Content {
			if result, ok := block.(cometsdk.ToolResultBlock); ok {
				total += len(result.Content)
			}
		}
	}
	return total
}
