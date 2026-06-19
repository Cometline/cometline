package retention

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
)

// Result summarizes one retention pass.
type Result struct {
	SessionsDeleted      int
	MemoriesPurged       int
	MemoryEventsPurged   int
	Vacuumed             bool
}

// Runner performs session retention and memory purge.
type Runner struct {
	DB       *sql.DB
	Sessions *session.Service
	Memory   *memory.Service
	Config   config.StorageConfig
	// IsRunning, when set, skips sessions with an in-flight agent turn.
	IsRunning func(sessionID string) bool
}

// Run applies configured retention rules. It is safe to call on startup.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	if r == nil || r.DB == nil || r.Sessions == nil {
		return Result{}, nil
	}
	cfg := r.Config
	var out Result

	if cfg.RetentionEnabled() {
		n, err := r.purgeSessions(ctx)
		if err != nil {
			return out, err
		}
		out.SessionsDeleted = n
	}

	if r.Memory != nil && cfg.MemoryPurgeEnabled() {
		memories, events, err := r.Memory.PurgeArchived(ctx, cfg.ArchivedMemoryPurgeDays)
		if err != nil {
			return out, err
		}
		out.MemoriesPurged = memories
		out.MemoryEventsPurged = events
	}

	if cfg.VacuumAfterPurge && (out.SessionsDeleted > 0 || out.MemoriesPurged > 0) {
		if _, err := r.DB.ExecContext(ctx, "VACUUM"); err != nil {
			return out, err
		}
		out.Vacuumed = true
	}

	if out.SessionsDeleted > 0 || out.MemoriesPurged > 0 {
		log.Printf(
			"cometmind: retention complete sessions_deleted=%d memories_purged=%d memory_events_purged=%d vacuumed=%v",
			out.SessionsDeleted,
			out.MemoriesPurged,
			out.MemoryEventsPurged,
			out.Vacuumed,
		)
	}
	return out, nil
}

func (r *Runner) purgeSessions(ctx context.Context) (int, error) {
	q := db.New(r.DB)
	workspaces, err := q.ListWorkspaces(ctx)
	if err != nil {
		return 0, err
	}

	deleted := make(map[string]struct{})
	for _, ws := range workspaces {
		if r.Config.RetentionDays > 0 {
			cutoff := time.Now().Add(-time.Duration(r.Config.RetentionDays) * 24 * time.Hour).UnixMilli()
			ids, err := q.ListStaleSessionIDs(ctx, db.ListStaleSessionIDsParams{
				WorkspaceID: ws.ID,
				UpdatedAt:   cutoff,
			})
			if err != nil {
				return 0, err
			}
			for _, id := range ids {
				if r.skipSession(id) {
					continue
				}
				if err := r.Sessions.DeleteSession(ctx, id); err != nil {
					return 0, err
				}
				deleted[id] = struct{}{}
			}
		}

		if r.Config.MaxSessionsPerWorkspace > 0 {
			rows, err := q.ListSessionsByWorkspaceAsc(ctx, ws.ID)
			if err != nil {
				return 0, err
			}
			extra := len(rows) - r.Config.MaxSessionsPerWorkspace
			if extra <= 0 {
				continue
			}
			for _, row := range rows {
				if extra <= 0 {
					break
				}
				if _, ok := deleted[row.ID]; ok {
					continue
				}
				if protectedDelegation(row.DelegationStatus) {
					continue
				}
				if r.skipSession(row.ID) {
					continue
				}
				if err := r.Sessions.DeleteSession(ctx, row.ID); err != nil {
					return 0, err
				}
				deleted[row.ID] = struct{}{}
				extra--
			}
		}
	}
	return len(deleted), nil
}

func (r *Runner) skipSession(sessionID string) bool {
	return r.IsRunning != nil && r.IsRunning(sessionID)
}

func protectedDelegation(status string) bool {
	switch status {
	case "pending", "running", "awaiting_user", "awaiting_permission":
		return true
	default:
		return false
	}
}
