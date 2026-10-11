package agent

const (
	reviewTurnThreshold = 10
	reviewMaxSteps      = 30
	reviewKind          = "skill_review"
)

type skillReviewInput struct {
	UserChat   bool
	HasModel   bool
	Iterations int
}

type skillReviewDecision struct {
	Start      bool
	Reason     string
	WideWindow bool
	LogSkip    string
}

func decideSkillReview(in skillReviewInput) skillReviewDecision {
	var decision skillReviewDecision
	if !in.UserChat || in.Iterations < reviewTurnThreshold {
		return decision
	}
	if !in.HasModel {
		decision.LogSkip = "skills.review.skipped_no_model"
		return decision
	}
	decision.Start = true
	decision.WideWindow = true
	decision.Reason = "iterations"
	return decision
}
