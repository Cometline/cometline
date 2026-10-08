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
