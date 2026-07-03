package planning

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusBlocked    = "blocked"
)

// Step is one persisted step in a session-scoped agent plan.
type Step struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	StepIndex     int64  `json:"step_index"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	BlockerReason string `json:"blocker_reason"`
	DismissedAt   *int64 `json:"dismissed_at,omitempty"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type StepInput struct {
	Description   string
	Status        string
	BlockerReason string
}
