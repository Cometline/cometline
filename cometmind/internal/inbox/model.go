package inbox

// Status values for inbox messages.
const (
	StatusOpen     = "open"
	StatusArchived = "archived"
)

// Archive reasons.
const (
	ArchiveReasonReplied   = "replied"
	ArchiveReasonDismissed = "dismissed"
)

// MaxProcessAttempts caps background internalization retries.
const MaxProcessAttempts = 3

// Message is a durable agent→user inbox note.
type Message struct {
	ID              string
	Title           string
	Body            string
	WorkspaceID     string
	JobID           string
	SessionID       string
	Status          string
	ArchiveReason   string
	UserReply       string
	ProcessedAt     *int64
	ProcessError    string
	ProcessAttempts int64
	ArchivedAt      *int64
	DeletedAt       *int64
	CreatedAt       int64
	UpdatedAt       int64
}

// CreateInput is the payload for leaving an inbox message.
type CreateInput struct {
	Title       string
	Body        string
	WorkspaceID string
	JobID       string
	SessionID   string
}

// ListFilter selects inbox messages.
type ListFilter struct {
	Status string // open | archived | empty for all non-deleted
}
