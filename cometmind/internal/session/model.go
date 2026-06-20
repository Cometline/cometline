package session

import "github.com/cometline/cometmind/internal/db"

// Workspace is the session-store view of a registered workspace root.
type Workspace struct {
	ID        string
	Name      string
	Path      string
	CreatedAt int64
}

// SessionGateway links a top-level session to an external chat surface.
type SessionGateway struct {
	Platform  string
	ChannelID string
	ThreadID  string
}

// Session is the session-store view of a persisted chat session.
type Session struct {
	ID                      string
	WorkspaceID             string
	Title                   string
	ModelID                 string
	ProviderID              string
	Status                  string
	TokenUsage              string
	ParentSessionID         string
	Purpose                 string
	DelegationStatus        string
	OutputSummary           string
	ACPSessionID            string
	PendingQuestion         string
	SubagentKind            string
	Gateway                 *SessionGateway
	Pinned                  bool
	ContextSummary          string
	CompactedUntilMessageID string
	ContextSummaryUpdatedAt string
	CreatedAt               int64
	UpdatedAt               int64
}

// Message is the session-store view of one persisted transcript row.
type Message struct {
	ID               string
	SessionID        string
	Role             string
	Content          string
	ReasoningContent string
	TokenCount       int64
	CreatedAt        int64
}

func workspaceFromDB(w db.Workspace) Workspace {
	return Workspace{
		ID:        w.ID,
		Name:      w.Name,
		Path:      w.Path,
		CreatedAt: w.CreatedAt,
	}
}

func sessionFromDB(s db.Session) Session {
	parent := ""
	if s.ParentSessionID.Valid {
		parent = s.ParentSessionID.String
	}
	compactedUntil := ""
	if s.CompactedUntilMessageID.Valid {
		compactedUntil = s.CompactedUntilMessageID.String
	}
	summaryUpdatedAt := ""
	if s.ContextSummaryUpdatedAt.Valid {
		summaryUpdatedAt = s.ContextSummaryUpdatedAt.String
	}
	return Session{
		ID:                      s.ID,
		WorkspaceID:             s.WorkspaceID,
		Title:                   s.Title,
		ModelID:                 s.ModelID,
		ProviderID:              s.ProviderID,
		Status:                  s.Status,
		TokenUsage:              s.TokenUsage,
		ParentSessionID:         parent,
		Purpose:                 s.Purpose,
		DelegationStatus:        s.DelegationStatus,
		OutputSummary:           s.OutputSummary,
		ACPSessionID:            s.AcpSessionID,
		PendingQuestion:         s.PendingQuestion,
		SubagentKind:            s.SubagentKind,
		Pinned:                  s.Pinned != 0,
		ContextSummary:          s.ContextSummary,
		CompactedUntilMessageID: compactedUntil,
		ContextSummaryUpdatedAt: summaryUpdatedAt,
		CreatedAt:               s.CreatedAt,
		UpdatedAt:               s.UpdatedAt,
	}
}

func messageFromDB(m db.Message) Message {
	return Message{
		ID:               m.ID,
		SessionID:        m.SessionID,
		Role:             m.Role,
		Content:          m.Content,
		ReasoningContent: m.ReasoningContent,
		TokenCount:       m.TokenCount,
		CreatedAt:        m.CreatedAt,
	}
}

func sessionsFromDB(rows []db.Session) []Session {
	out := make([]Session, len(rows))
	for i, s := range rows {
		out[i] = sessionFromDB(s)
	}
	return out
}
