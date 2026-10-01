package session

import (
	"context"
	"database/sql"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
)

// NewChildSession creates a delegated child session linked to a parent.
func (s *Service) NewChildSession(ctx context.Context, parent Session, purpose, subagentKind string) (Session, error) {
	title := purpose
	if len(title) > 80 {
		title = title[:80]
	}
	sess, err := s.q.CreateChildSession(ctx, db.CreateChildSessionParams{
		ID:               id.New(),
		WorkspaceID:      parent.WorkspaceID,
		Title:            title,
		ModelID:          parent.ModelID,
		ProviderID:       parent.ProviderID,
		Status:           "active",
		ParentSessionID:  sql.NullString{String: parent.ID, Valid: true},
		Purpose:          purpose,
		DelegationStatus: DelegationPending.String(),
		OutputSummary:    "",
		SubagentKind:     subagentKind,
		AgentMode:        parent.AgentMode,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(sess), nil
}

// CompactChildSession wipes a child transcript while preserving delegation metadata.
func (s *Service) CompactChildSession(ctx context.Context, childID string) error {
	if err := s.q.DeleteMessagesBySession(ctx, childID); err != nil {
		return err
	}
	return s.q.CompactChildSession(ctx, childID)
}

// LastAssistantText returns the most recent assistant message text for a session.
func (s *Service) LastAssistantText(ctx context.Context, sessionID string) (string, error) {
	msgs, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return "", err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			return msgs[i].Content, nil
		}
	}
	return "", nil
}

// ListChildSessions returns delegated sessions for a parent session.
func (s *Service) ListChildSessions(ctx context.Context, parentSessionID string) ([]Session, error) {
	rows, err := s.q.ListChildSessions(ctx, sql.NullString{String: parentSessionID, Valid: true})
	if err != nil {
		return nil, err
	}
	return sessionsFromDB(rows), nil
}

// UpdateDelegationState persists delegation status and summary for a child session.
func (s *Service) UpdateDelegationState(ctx context.Context, sessionID string, status DelegationStatus, summary string) error {
	return s.q.UpdateSessionDelegation(ctx, db.UpdateSessionDelegationParams{
		DelegationStatus: status.String(),
		OutputSummary:    summary,
		ID:               sessionID,
	})
}

// GetActiveChildForParent returns the most recently updated active delegated child.
func (s *Service) GetActiveChildForParent(ctx context.Context, parentSessionID string) (Session, error) {
	row, err := s.q.GetActiveChildForParent(ctx, sql.NullString{String: parentSessionID, Valid: true})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(row), nil
}
