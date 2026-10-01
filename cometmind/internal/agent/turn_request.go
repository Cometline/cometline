package agent

import (
	"context"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func (r *Runner) buildTurnRequest(ctx context.Context, s *turnState, system string, msgs []cometsdk.Message, tools []cometsdk.Tool) *cometsdk.Request {
	requestMsgs := DowngradeImagesForNonVision(msgs, s.budget.VisionKnown, s.budget.Vision)
	req := BuildRequest(s.turn.ModelID, system, requestMsgs, tools, s.maxTokens)
	req.ReasoningEffort = r.reasoningEffortFor(s.turn)
	r.applyCompatibility(ctx, req)
	return req
}

func (r *Runner) applyCompatibility(ctx context.Context, req *cometsdk.Request) {
	if r.Compatibility != nil {
		req.Compatibility = r.Compatibility.ResolveCapabilityPolicy(ctx, r.CompatibilityScope)
	}
}

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
