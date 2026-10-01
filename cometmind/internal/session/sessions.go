package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
	"github.com/Cometline/cometline/cometmind/internal/media"
)

// NewSession creates a persisted session row scoped to a workspace.
func (s *Service) NewSession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	return s.newSessionWithOrigin(ctx, workspaceID, modelID, providerID, "user")
}

// NewAutonomySession creates a session dedicated to an autonomous job run.
func (s *Service) NewAutonomySession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	return s.newSessionWithOrigin(ctx, workspaceID, modelID, providerID, "autonomy")
}

// NewInboxSession creates a short-lived session for inbox reply internalization.
func (s *Service) NewInboxSession(ctx context.Context, workspaceID string, modelID, providerID string) (Session, error) {
	return s.newSessionWithOrigin(ctx, workspaceID, modelID, providerID, "inbox")
}

func (s *Service) newSessionWithOrigin(ctx context.Context, workspaceID string, modelID, providerID, origin string) (Session, error) {
	sess, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:          id.New(),
		WorkspaceID: workspaceID,
		Title:       "",
		ModelID:     modelID,
		ProviderID:  providerID,
		Status:      "active",
		Origin:      origin,
		AgentMode:   string(AgentModeAuto),
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(sess), nil
}

// GetSession loads a session by id.
func (s *Service) GetSession(ctx context.Context, sessionID string) (Session, error) {
	row, err := s.q.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, mapNotFound(err, ErrSessionNotFound)
	}
	return attachGatewayMetadata(
		sessionFromDB(row.Session),
		row.GatewayPlatform,
		row.GatewayChannelID,
		row.GatewayThreadID,
	), nil
}

// ListSessions lists sessions for a workspace ordered by recent activity.
func (s *Service) ListSessions(ctx context.Context, workspaceID string) ([]Session, error) {
	rows, err := s.q.ListSessionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return sessionsFromDB(rows), nil
}

// ListAllSessions lists user-facing top-level sessions across every workspace,
// ordered by recent activity. Delegated child sessions and autonomous job-run
// sessions are excluded.
func (s *Service) ListAllSessions(ctx context.Context) ([]Session, error) {
	rows, err := s.q.ListAllSessions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Session, len(rows))
	for i, row := range rows {
		out[i] = attachGatewayMetadata(
			sessionFromDB(row.Session),
			row.GatewayPlatform,
			row.GatewayChannelID,
			row.GatewayThreadID,
		)
	}
	return out, nil
}

// ephemeralSessionOrigin reports origins that exist only as an execution
// container. Their media is a run artifact, not a Gallery item.
func ephemeralSessionOrigin(origin string) bool {
	switch strings.TrimSpace(origin) {
	case "autonomy", "inbox":
		return true
	default:
		return false
	}
}

// DeleteSession removes a session and cascades its messages and tool calls.
// Child sessions are deleted first so delegated rows cannot orphan into the sidebar.
// Gallery media stays on disk and in the catalog with a null session_id.
func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	children, err := s.ListChildSessions(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.DeleteSession(ctx, child.ID); err != nil {
			return err
		}
	}
	return s.q.DeleteSession(ctx, sessionID)
}

// DiscardEphemeralSession deletes an autonomy or inbox execution container
// together with its on-disk media. User chats must not go through this path:
// their gallery files are meant to survive session deletion.
func (s *Service) DiscardEphemeralSession(ctx context.Context, sessionID string) error {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}
	if !ephemeralSessionOrigin(sess.Origin) {
		return fmt.Errorf("session %s is not ephemeral", sessionID)
	}
	children, err := s.ListChildSessions(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.DeleteSession(ctx, child.ID); err != nil {
			return err
		}
	}
	if err := media.DeleteSession(sessionID); err != nil {
		return err
	}
	return s.q.DeleteSession(ctx, sessionID)
}

// DiscardFinishedEphemeralSessions removes autonomy and inbox sessions that
// are no longer an active run. Startup uses this to clean containers left
// behind by older builds.
func (s *Service) DiscardFinishedEphemeralSessions(ctx context.Context, running func(sessionID string) bool) (int, error) {
	ids, err := s.q.ListEphemeralSessionIDs(ctx)
	if err != nil {
		return 0, err
	}
	discarded := 0
	for _, id := range ids {
		if running != nil && running(id) {
			continue
		}
		if err := s.DiscardEphemeralSession(ctx, id); err != nil {
			return discarded, err
		}
		discarded++
	}
	return discarded, nil
}

// PruneUnusedUserSessions removes top-level user sessions that were created
// but never changed or given a transcript. Cleared conversations are retained.
func (s *Service) PruneUnusedUserSessions(ctx context.Context) (int, error) {
	ids, err := s.q.ListUnusedUserSessionIDs(ctx)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.DeleteSession(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// ClearSessionTranscript deletes all transcript rows for a session and resets
// compaction and token usage while preserving the session identity and title.
// Delegated child sessions are removed as well so subagent UI does not reappear
// on transcript reload.
func (s *Service) ClearSessionTranscript(ctx context.Context, sessionID string) error {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return err
	}
	children, err := s.ListChildSessions(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.DeleteSession(ctx, child.ID); err != nil {
			return err
		}
	}
	if err := s.q.DeleteMessagesBySession(ctx, sessionID); err != nil {
		return err
	}
	return s.q.ResetSessionTranscriptState(ctx, db.ResetSessionTranscriptStateParams{
		TokenUsage: "{}",
		ID:         sessionID,
	})
}

// UpdateContextSummary persists rolling compaction state for a session.
func (s *Service) UpdateContextSummary(ctx context.Context, sessionID, summary, untilMessageID string) error {
	var until sql.NullString
	if strings.TrimSpace(untilMessageID) != "" {
		until = sql.NullString{String: untilMessageID, Valid: true}
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	return s.q.UpdateSessionContextSummary(ctx, db.UpdateSessionContextSummaryParams{
		ContextSummary:          summary,
		CompactedUntilMessageID: until,
		ContextSummaryUpdatedAt: sql.NullString{String: updatedAt, Valid: true},
		ID:                      sessionID,
	})
}

// SetTitleIfEmpty updates session title once (used after first user turn).
// The update is expressed as a single atomic SQL statement whose WHERE clause
// checks for a blank title, eliminating the read-check-write TOCTOU race that
// would occur if two concurrent callers both observed an empty title.
func (s *Service) SetTitleIfEmpty(ctx context.Context, sessionID, title string) error {
	return s.q.SetTitleIfEmpty(ctx, db.SetTitleIfEmptyParams{
		ID:    sessionID,
		Title: title,
	})
}

// UpdateTitle unconditionally overwrites a session's title. Used when an
// LLM-generated title replaces the provisional first-turn placeholder.
func (s *Service) UpdateTitle(ctx context.Context, sessionID, title string) error {
	return s.q.UpdateSessionTitle(ctx, db.UpdateSessionTitleParams{
		ID:    sessionID,
		Title: title,
	})
}

// UpdateSessionModel persists a new model/provider pair for an existing session.
func (s *Service) UpdateSessionModel(ctx context.Context, sessionID, modelID, providerID string) (Session, error) {
	modelID = strings.TrimSpace(modelID)
	providerID = strings.TrimSpace(providerID)
	if modelID == "" || providerID == "" {
		return Session{}, fmt.Errorf("model_id and provider_id are required")
	}
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	if err := s.q.UpdateSessionModel(ctx, db.UpdateSessionModelParams{
		ModelID:    modelID,
		ProviderID: providerID,
		ID:         sessionID,
	}); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}

// UpdateSessionPinned persists whether a session is pinned in the sidebar.
func (s *Service) UpdateSessionPinned(ctx context.Context, sessionID string, pinned bool) (Session, error) {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	var pinnedInt int64
	if pinned {
		pinnedInt = 1
	}
	if err := s.q.UpdateSessionPinned(ctx, db.UpdateSessionPinnedParams{
		Pinned: pinnedInt,
		ID:     sessionID,
	}); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}

// UpdateSessionAgentMode persists the preferred agent mode for a session.
func (s *Service) UpdateSessionAgentMode(ctx context.Context, sessionID string, mode AgentMode) (Session, error) {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	row, err := s.q.UpdateSessionAgentMode(ctx, db.UpdateSessionAgentModeParams{
		AgentMode: string(mode),
		ID:        sessionID,
	})
	if err != nil {
		return Session{}, err
	}
	return sessionFromDB(row), nil
}

// UpdateSessionTitle persists a new display title for an existing session.
func (s *Service) UpdateSessionTitle(ctx context.Context, sessionID, title string) (Session, error) {
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Session{}, err
	}
	if err := s.q.UpdateSessionTitle(ctx, db.UpdateSessionTitleParams{
		ID:    sessionID,
		Title: strings.TrimSpace(title),
	}); err != nil {
		return Session{}, err
	}
	return s.GetSession(ctx, sessionID)
}
