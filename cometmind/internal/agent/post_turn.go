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
	// Extraction runs in the background so the SSE stream can close on done
	// and the next queued message can start without waiting on the extractor.
	go r.extractMemoryAfterTurn(context.WithoutCancel(ctx), s.turn, nil)
	return nil
}
