package session

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

// AddSkillReviewMutatingCount adds this turn to the wiki user-turn counter.
func (s *Service) AddSkillReviewMutatingCount(ctx context.Context, sessionID string, n int64) (int64, error) {
	if n == 0 {
		sess, err := s.GetSession(ctx, sessionID)
		if err != nil {
			return 0, err
		}
		return sess.SkillReviewMutatingCount, nil
	}
	return s.q.AddSkillReviewMutatingCount(ctx, db.AddSkillReviewMutatingCountParams{
		SkillReviewMutatingCount: n,
		ID:                       sessionID,
	})
}

// AddSkillReviewToolIterations adds tool-calling iterations and returns the new total.
func (s *Service) AddSkillReviewToolIterations(ctx context.Context, sessionID string, n int64) (int64, error) {
	if n == 0 {
		sess, err := s.GetSession(ctx, sessionID)
		if err != nil {
			return 0, err
		}
		return sess.SkillReviewToolIterations, nil
	}
	return s.q.AddSkillReviewToolIterations(ctx, db.AddSkillReviewToolIterationsParams{
		SkillReviewToolIterations: n,
		ID:                        sessionID,
	})
}

// SetSkillReviewToolIterations sets the skill-review iteration counter.
func (s *Service) SetSkillReviewToolIterations(ctx context.Context, sessionID string, count int64) error {
	return s.q.SetSkillReviewToolIterations(ctx, db.SetSkillReviewToolIterationsParams{
		SkillReviewToolIterations: count,
		ID:                        sessionID,
	})
}

// SetSkillReviewCounter sets the wiki user-turn counter and the skill transcript window.
func (s *Service) SetSkillReviewCounter(ctx context.Context, sessionID string, count, resetAt int64) error {
	return s.q.SetSkillReviewCounter(ctx, db.SetSkillReviewCounterParams{
		SkillReviewMutatingCount: count,
		SkillReviewCountResetAt:  resetAt,
		ID:                       sessionID,
	})
}

// ResetWikiReviewAfterStart clears the user-turn counter when a wiki review starts.
func (s *Service) ResetWikiReviewAfterStart(ctx context.Context, sessionID string) error {
	return s.q.ResetWikiReviewAfterStart(ctx, sessionID)
}

// ResetSkillReviewAfterStart clears the iteration counter and moves the transcript window.
func (s *Service) ResetSkillReviewAfterStart(ctx context.Context, sessionID string, at int64) error {
	return s.q.ResetSkillReviewAfterStart(ctx, db.ResetSkillReviewAfterStartParams{
		SkillReviewCountResetAt: at,
		ID:                      sessionID,
	})
}
