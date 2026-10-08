package agent

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"go.uber.org/zap"
)

// completeTurn runs the post-turn hooks for a turn that finished normally.
func (r *Runner) completeTurn(ctx context.Context, s *turnState) error {
	if r.Compactor != nil && s.turn.ID != "" {
		if err := r.Compactor.Prune(ctx, s.turn.ID); err != nil {
			logging.L().Warn("context.prune.failed", zap.String("session", s.turn.ID), zap.Error(err))
		}
	}
	s.sendDone()
	// Background work starts after done so the SSE stream can close and the
	// next queued message can start without waiting on it.
	bg := context.WithoutCancel(ctx)
	go r.extractMemoryAfterTurn(bg, s.turn, nil)
	go r.reviewSkillsAfterTurn(bg, s.turn)
	return nil
}
