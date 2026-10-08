package session

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

// SetSkillReviewStartedAt records when a skill-review child successfully started.
func (s *Service) SetSkillReviewStartedAt(ctx context.Context, sessionID string, at int64) error {
	return s.q.SetSkillReviewStartedAt(ctx, db.SetSkillReviewStartedAtParams{
		SkillReviewStartedAt: at,
		ID:                   sessionID,
	})
}

// SetSkillReviewLastTargets stores the previous turn's mutating paths and commands.
func (s *Service) SetSkillReviewLastTargets(ctx context.Context, sessionID, targets string) error {
	return s.q.SetSkillReviewLastTargets(ctx, db.SetSkillReviewLastTargetsParams{
		SkillReviewLastTargets: targets,
		ID:                     sessionID,
	})
}

// AddSkillReviewMutatingCount adds this turn's mutating calls and returns the new total.
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

// SetSkillReviewCounter sets the accumulated count and the window start.
func (s *Service) SetSkillReviewCounter(ctx context.Context, sessionID string, count, resetAt int64) error {
	return s.q.SetSkillReviewCounter(ctx, db.SetSkillReviewCounterParams{
		SkillReviewMutatingCount: count,
		SkillReviewCountResetAt:  resetAt,
		ID:                       sessionID,
	})
}

// SetWikiReviewStartedAt records when a wiki compile child successfully started.
func (s *Service) SetWikiReviewStartedAt(ctx context.Context, sessionID string, at int64) error {
	return s.q.SetWikiReviewStartedAt(ctx, db.SetWikiReviewStartedAtParams{
		WikiReviewStartedAt: at,
		ID:                  sessionID,
	})
}

// ResetSkillReviewAfterStart clears the counter when a review child starts.
func (s *Service) ResetSkillReviewAfterStart(ctx context.Context, sessionID string, at int64) error {
	return s.q.ResetSkillReviewAfterStart(ctx, db.ResetSkillReviewAfterStartParams{
		SkillReviewCountResetAt: at,
		SkillReviewStartedAt:    at,
		ID:                      sessionID,
	})
}
