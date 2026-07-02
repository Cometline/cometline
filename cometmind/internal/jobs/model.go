package jobs

import (
	"time"
)

const (
	StatusTodo    = "todo"
	StatusOngoing = "ongoing"
	StatusDone    = "done"
	StatusBlocked = "blocked"

	CreatedByUser  = "user"
	CreatedByAgent = "agent"

	PlatformDesktop = "desktop"
	PlatformDiscord = "discord"

	DefaultLeaseMinutes       = 30
	DefaultReconcileInterval  = 2 * time.Minute
	DefaultStaleReviewMinutes = 30
	DefaultMaxFailures        = 3
	DefaultRetryCooldownMins  = 5
	DefaultMaxRetryCooldown   = 60

	EventCreated      = "created"
	EventClaimed      = "claimed"
	EventReleased     = "released"
	EventCompleted    = "completed"
	EventUpdated      = "updated"
	EventArchived     = "archived"
	EventUnarchived   = "unarchived"
	EventFailed       = "failed"
	EventBlocked      = "blocked"
	EventDeleted      = "deleted"
	EventLeaseExpired = "lease_expired"
	EventNotified     = "notified"
)

type FailureClass string

const (
	FailureNone         FailureClass = "none"
	FailureAgentHandoff FailureClass = "agent_handoff"
	FailureWorkerError  FailureClass = "worker_error"
	FailureInfra        FailureClass = "infra"
)

// Job is the domain view of a global work item.
type Job struct {
	ID                string
	Description       string
	DefinitionOfDone  string
	Progress          string
	Status            string
	WorkspacePath     string
	AssignedSessionID string
	LeaseExpiresAt    *int64
	CreatedBy         string
	SourceSessionID   string
	SourcePlatform    string
	SourceChannelID   string
	ArchivedAt        *int64
	FailureCount      int64
	NextRetryAt       *int64
	LastFailureReason string
	DeletedAt         *int64
	CreatedAt         int64
	UpdatedAt         int64
}

// JobEvent is an audit log entry for a job.
type JobEvent struct {
	ID             string
	JobID          string
	Action         string
	Detail         string
	ActorSessionID string
	CreatedAt      int64
}

// CreateInput holds fields for creating a job.
type CreateInput struct {
	Description      string
	DefinitionOfDone string
	WorkspacePath    string
	CreatedBy        string
	SourceSessionID  string
	SourcePlatform   string
	SourceChannelID  string
}

// UpdateTodoInput holds editable fields while status is todo.
type UpdateTodoInput struct {
	Description      string
	DefinitionOfDone string
	WorkspacePath    string
}

// ListFilter filters job listings.
type ListFilter struct {
	Status          string
	ReadyOnly       bool
	IncludeDeleted  bool
	IncludeArchived bool
}
