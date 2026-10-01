package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
)

// EnsureWorkspace registers the absolute workspace root in the global store when missing.
func (s *Service) EnsureWorkspace(ctx context.Context, absRoot string) (Workspace, error) {
	clean := filepath.Clean(absRoot)
	w, err := s.q.GetWorkspaceByPath(ctx, clean)
	if err == nil {
		return workspaceFromDB(w), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, err
	}
	created, err := s.q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		ID:   id.New(),
		Name: filepath.Base(clean),
		Path: clean,
	})
	if err != nil {
		return Workspace{}, err
	}
	return workspaceFromDB(created), nil
}

// GetWorkspace loads a workspace by id.
func (s *Service) GetWorkspace(ctx context.Context, workspaceID string) (Workspace, error) {
	w, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return Workspace{}, mapNotFound(err, ErrWorkspaceNotFound)
	}
	return workspaceFromDB(w), nil
}

// LookupWorkspaceByPath loads a workspace by path without creating it.
func (s *Service) LookupWorkspaceByPath(ctx context.Context, absRoot string) (Workspace, error) {
	w, err := s.q.GetWorkspaceByPath(ctx, filepath.Clean(absRoot))
	if err != nil {
		return Workspace{}, mapNotFound(err, ErrWorkspaceNotFound)
	}
	return workspaceFromDB(w), nil
}

// ListWorkspaces returns registered workspace roots that still exist on disk.
func (s *Service) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	rows, err := s.q.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, 0, len(rows))
	for _, row := range rows {
		ws := workspaceFromDB(row)
		if !workspaceRootExists(ws.Path) {
			continue
		}
		out = append(out, ws)
	}
	return out, nil
}

// CountSessionsForWorkspace returns how many sessions reference a workspace.
func (s *Service) CountSessionsForWorkspace(ctx context.Context, workspaceID string) (int64, error) {
	return s.q.CountSessionsForWorkspace(ctx, workspaceID)
}

// PruneMissingWorkspaces removes registered workspaces whose directories are
// gone and that have no sessions. Workspaces with sessions are kept for history.
func (s *Service) PruneMissingWorkspaces(ctx context.Context) (int, error) {
	rows, err := s.q.ListWorkspaces(ctx)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, row := range rows {
		if workspaceRootExists(row.Path) {
			continue
		}
		count, err := s.q.CountSessionsForWorkspace(ctx, row.ID)
		if err != nil {
			return pruned, err
		}
		if count > 0 {
			continue
		}
		if err := s.q.DeleteWorkspace(ctx, row.ID); err != nil {
			return pruned, err
		}
		pruned++
	}
	return pruned, nil
}

// DeleteWorkspaceByPath removes a workspace registration when it has no sessions.
func (s *Service) DeleteWorkspaceByPath(ctx context.Context, absRoot string) error {
	clean := filepath.Clean(strings.TrimSpace(absRoot))
	if clean == "" {
		return fmt.Errorf("workspace path is required")
	}
	w, err := s.q.GetWorkspaceByPath(ctx, clean)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	count, err := s.q.CountSessionsForWorkspace(ctx, w.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrWorkspaceHasSessions
	}
	return s.q.DeleteWorkspace(ctx, w.ID)
}

func workspaceRootExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ChangeSessionWorkspace reassigns a session to a different workspace root.
func (s *Service) ChangeSessionWorkspace(ctx context.Context, sessionID, absPath string) (Session, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if sess.DelegationStatus.IsActive() {
		return Session{}, ErrActiveDelegation
	}

	ws, err := s.EnsureWorkspace(ctx, absPath)
	if err != nil {
		return Session{}, err
	}
	if ws.ID == sess.WorkspaceID {
		return sess, nil
	}

	oldPath, err := s.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return Session{}, err
	}

	if err := s.q.UpdateSessionWorkspace(ctx, db.UpdateSessionWorkspaceParams{
		WorkspaceID: ws.ID,
		ID:          sessionID,
	}); err != nil {
		return Session{}, err
	}
	_ = s.q.UpdateGatewaySessionWorkspace(ctx, db.UpdateGatewaySessionWorkspaceParams{
		WorkspaceID:        ws.ID,
		CometmindSessionID: sessionID,
	})
	_ = s.q.UpdateSessionMediaWorkspace(ctx, db.UpdateSessionMediaWorkspaceParams{
		WorkspaceID: nullSessionID(ws.ID),
		SessionID:   nullSessionID(sessionID),
	})

	note := fmt.Sprintf(
		"Workspace changed from %s to %s. File tools now operate under this directory.",
		oldPath,
		ws.Path,
	)
	if _, err := s.AppendSystemMessage(ctx, sessionID, note); err != nil {
		return Session{}, err
	}

	return s.GetSession(ctx, sessionID)
}

// WorkspacePath resolves the filesystem root for a workspace id. This method
// is intentionally duplicated from the WorkspaceStore interface seam so the
// full *Service can satisfy it.
func (s *Service) WorkspacePath(ctx context.Context, workspaceID string) (string, error) {
	w, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return "", mapNotFound(err, ErrWorkspaceNotFound)
	}
	return w.Path, nil
}
