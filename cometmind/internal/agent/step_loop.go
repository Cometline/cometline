package agent

import (
	"context"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"go.uber.org/zap"
)

// stepRequest is the model request for one step plus the inputs overflow
// recovery needs to rebuild it.
type stepRequest struct {
	finalizing         bool
	tools              []cometsdk.Tool
	baseSystem         string
	memoryPromptSuffix string
	req                *cometsdk.Request
	toolOutputBytes    int
	recentTools        []ToolFingerprint
}

// prepareStep builds the request for the next step and consumes the pending
// continue nudges. Finalizing steps are sent without tools.
func (r *Runner) prepareStep(ctx context.Context, s *turnState) (*stepRequest, error) {
	p := &stepRequest{finalizing: s.steps == r.MaxSteps || s.doomLoopHalt}
	p.tools = r.Registry.CometSDK()
	if p.finalizing {
		p.tools = nil
	}
	if s.steps > 0 {
		s.emitStatus(event.PhaseContinuing)
	}
	p.baseSystem = r.compactBeforeStep(ctx, s, r.buildSystemPrompt(s.sess.ContextSummary, s.maxTokens), p.tools)

	msgs, err := r.Sessions.BuildSDKMessages(ctx, s.turn.ID)
	if err != nil {
		return nil, s.fail(err, "history")
	}

	// Normalize replay history and report any lossy degradations once per turn.
	normalized, degradations := NormalizeHistory(msgs)
	msgs = normalized
	p.recentTools = toolFingerprintsSinceLastUser(normalized)
	// Continue nudges are in-memory user turns so providers that reject
	// trailing assistant prefills (Claude 4.6+) still accept the request.
	msgs = append(msgs, s.nudges.messages(s.jobTracker.JobID)...)
	if s.doomLoopHalt {
		msgs = append(msgs, DoomLoopStopMessages()...)
	} else if p.finalizing {
		msgs = append(msgs, FinalAnswerNudgeMessages()...)
	}
	if !s.degradationsReported {
		for _, d := range degradations {
			logging.L().Info("history.normalized", zap.String("session", s.turn.ID), zap.String("kind", d.Kind), zap.Int("count", d.Count))
		}
		s.degradationsReported = true
	}

	logging.L().Info("agent.step.start", zap.String("session", s.turn.ID), zap.Int("step", s.steps+1), zap.String("model", s.turn.ModelID), zap.Int("messages", len(msgs)), zap.Int("max_tokens", s.maxTokens), zap.Int("context_window", s.budget.Context), zap.String("limit_source", s.budget.LimitSource))

	s.nudges = continueNudges{}
	p.memoryPromptSuffix = r.injectTurnMemories(ctx, s, p.baseSystem, msgs)

	s.emitStatus(event.PhaseContactingModel)
	p.req = r.buildTurnRequest(ctx, s, p.baseSystem+p.memoryPromptSuffix, msgs, p.tools)
	p.toolOutputBytes = toolResultBytes(p.req.Messages)
	return p, nil
}
