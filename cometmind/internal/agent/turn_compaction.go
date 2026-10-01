package agent

import (
	"context"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

// compactBeforeStep gives the compactor a chance to fold old history into the
// session summary and returns the system prompt to use for the step.
func (r *Runner) compactBeforeStep(ctx context.Context, s *turnState, baseSystem string, tools []cometsdk.Tool) string {
	if r.Compactor == nil || s.sess.ID == "" {
		return baseSystem
	}
	r.emitContextBudget(ctx, s, baseSystem, tools, false)
	before := s.sess
	updated, err := r.Compactor.MaybeCompact(
		ctx,
		s.sess,
		baseSystem,
		tools,
		r.Provider,
		s.turn.ProviderID,
		s.turn.ModelID,
		false,
		s.emit,
	)
	if err != nil {
		return baseSystem
	}
	s.sess = updated
	baseSystem = r.buildSystemPrompt(s.sess.ContextSummary, s.maxTokens)
	if compactionChanged(before, s.sess) {
		r.emitContextBudget(ctx, s, baseSystem, tools, true)
	}
	return baseSystem
}

func (r *Runner) emitContextBudget(ctx context.Context, s *turnState, system string, tools []cometsdk.Tool, compacted bool) {
	budget, err := r.Compactor.EstimatePromptBudget(
		ctx, s.sess.ID, system, tools, s.turn.ProviderID, s.turn.ModelID,
	)
	if err != nil {
		logging.L().Warn("context.budget.estimate_failed", "session", s.sess.ID, "error", err)
		return
	}
	s.ch <- event.ContextBudget(budget.Estimated, budget.Available, budget.ContextWindow, compacted)
}

// recoverFromOverflow force-compacts after a context-overflow stream failure
// and rebuilds p's request. It reports whether the step should be retried.
func (r *Runner) recoverFromOverflow(ctx context.Context, s *turnState, p *stepRequest, err error) bool {
	logging.L().Warn("agent.step.overflow_recover", "session", s.turn.ID, "provider", r.Provider.ID(), "model", s.turn.ModelID, "step", s.steps+1, "error", err)
	before := s.sess
	updated, compactErr := r.Compactor.MaybeCompact(
		ctx,
		s.sess,
		p.baseSystem,
		p.tools,
		r.Provider,
		s.turn.ProviderID,
		s.turn.ModelID,
		true,
		s.emit,
	)
	if compactErr != nil {
		logging.L().Warn("agent.step.overflow_compact_failed", "session", s.turn.ID, "error", compactErr)
		return false
	}
	s.sess = updated
	p.baseSystem = r.buildSystemPrompt(s.sess.ContextSummary, s.maxTokens)
	rebuildMsgs, rebuildErr := r.Sessions.BuildSDKMessages(ctx, s.turn.ID)
	if rebuildErr != nil {
		logging.L().Warn("agent.step.overflow_rebuild_failed", "session", s.turn.ID, "error", rebuildErr)
		return false
	}
	rebuildMsgs, _ = NormalizeHistory(rebuildMsgs)
	rebuildMsgs = append(rebuildMsgs, s.nudges.messages(s.jobTracker.JobID)...)
	if p.finalizing {
		rebuildMsgs = append(rebuildMsgs, FinalAnswerNudgeMessages()...)
	}
	p.req = r.buildTurnRequest(ctx, s, p.baseSystem+p.memoryPromptSuffix, rebuildMsgs, p.tools)
	p.toolOutputBytes = toolResultBytes(p.req.Messages)
	if compactionChanged(before, s.sess) {
		budget, budgetErr := r.Compactor.EstimatePromptBudget(
			ctx, s.sess.ID, p.baseSystem, p.tools, s.turn.ProviderID, s.turn.ModelID,
		)
		if budgetErr == nil {
			s.ch <- event.ContextBudget(budget.Estimated, budget.Available, budget.ContextWindow, true)
		}
	}
	return true
}

func compactionChanged(before, after session.Session) bool {
	return after.ContextSummary != before.ContextSummary || after.CompactedUntilMessageID != before.CompactedUntilMessageID
}
