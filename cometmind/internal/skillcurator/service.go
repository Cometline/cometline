package skillcurator

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"go.uber.org/zap"
)

// Service persists curator rows and applies filesystem transitions.
type Service struct {
	q   *db.Queries
	Now func() time.Time
}

// New builds a curator service.
func New(conn *sql.DB) *Service {
	return &Service{q: db.New(conn)}
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Pass is the one-row maintenance clock.
func (s *Service) Pass(ctx context.Context) (db.SkillCuratorPass, error) {
	if err := s.q.EnsureSkillCuratorPass(ctx); err != nil {
		return db.SkillCuratorPass{}, err
	}
	return s.q.GetSkillCuratorPass(ctx)
}

// SavePass writes the maintenance clock.
func (s *Service) SavePass(ctx context.Context, pass db.SkillCuratorPass) error {
	if err := s.q.EnsureSkillCuratorPass(ctx); err != nil {
		return err
	}
	return s.q.UpdateSkillCuratorPass(ctx, db.UpdateSkillCuratorPassParams{
		LastPassAt:    pass.LastPassAt,
		LastMergeAt:   pass.LastMergeAt,
		RunsIdleSince: pass.RunsIdleSince,
	})
}

// RunningTurns reports whether any session run exists.
func (s *Service) RunningTurns(ctx context.Context) (bool, error) {
	n, err := s.q.CountSessionRuns(ctx)
	return n > 0, err
}

// Sync inserts rows for self-improvement skills that do not have one yet.
func (s *Service) Sync(ctx context.Context) error {
	mirror, err := skills.MirrorRoot()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(mirror)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	existing, err := s.q.ListSkillCuratorStates(ctx)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, row := range existing {
		have[row.SkillName] = true
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == ".archive" || !skills.ValidSkillName(name) || have[name] {
			continue
		}
		dir := filepath.Join(mirror, name)
		skill, err := skills.ReadSkill(dir)
		if err != nil || !skills.IsSelfImprovement(skill) {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil {
			continue
		}
		created := info.ModTime().UnixMilli()
		if err := s.q.UpsertSkillCuratorState(ctx, db.UpsertSkillCuratorStateParams{
			SkillName: name,
			Origin:    skills.OriginSelfImprovement,
			Status:    skills.CuratorStatusActive,
			CreatedAt: created,
		}); err != nil {
			return err
		}
	}
	return nil
}

// Rows returns lifecycle rows as transition input.
func (s *Service) Rows(ctx context.Context) ([]skills.CuratorSkill, error) {
	rows, err := s.q.ListSkillCuratorStates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]skills.CuratorSkill, 0, len(rows))
	for _, row := range rows {
		out = append(out, skills.CuratorSkill{
			Name:             row.SkillName,
			Status:           row.Status,
			Pinned:           row.Pinned != 0,
			CreatedAt:        unixMilli(row.CreatedAt),
			LastUsedAt:       unixMilli(row.LastUsedAt),
			ArchivedAt:       unixMilli(row.ArchivedAt),
			DeleteNotifiedAt: unixMilli(row.DeleteNotifiedAt),
		})
	}
	return out, nil
}

// Apply runs filesystem moves for one pass and returns archive and delete names.
func (s *Service) Apply(ctx context.Context, actions []skills.CuratorAction) (archived, deleted []string) {
	now := s.now().UnixMilli()
	for _, action := range actions {
		row, err := s.q.GetSkillCuratorState(ctx, action.Name)
		if err != nil {
			logging.L().Warn("skills.curator.row_missing", zap.String("skill", action.Name), zap.Error(err))
			continue
		}
		switch action.To {
		case skills.CuratorStatusStale:
			row.Status = skills.CuratorStatusStale
		case skills.CuratorStatusArchived:
			if err := skills.ArchiveManagedSkill(action.Name); err != nil {
				logging.L().Warn("skills.curator.archive_failed", zap.String("skill", action.Name), zap.Error(err))
				continue
			}
			row.Status = skills.CuratorStatusArchived
			row.ArchivedAt = now
			row.DeleteNotifiedAt = now
			archived = append(archived, action.Name)
		case "deleted":
			if err := skills.DeleteArchivedSkill(action.Name); err != nil {
				logging.L().Warn("skills.curator.delete_failed", zap.String("skill", action.Name), zap.Error(err))
				continue
			}
			if err := s.q.DeleteSkillCuratorState(ctx, action.Name); err != nil {
				logging.L().Warn("skills.curator.delete_row_failed", zap.String("skill", action.Name), zap.Error(err))
				continue
			}
			deleted = append(deleted, action.Name)
			continue
		default:
			continue
		}
		if err := s.save(ctx, row); err != nil {
			logging.L().Warn("skills.curator.save_failed", zap.String("skill", action.Name), zap.Error(err))
		}
	}
	return archived, deleted
}

// NoteUse records a successful load_skill and restores an archived skill.
func (s *Service) NoteUse(ctx context.Context, name string) error {
	now := s.now().UnixMilli()
	row, err := s.q.GetSkillCuratorState(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return s.q.UpsertSkillCuratorState(ctx, db.UpsertSkillCuratorStateParams{
			SkillName:   name,
			Origin:      skills.OriginSelfImprovement,
			Status:      skills.CuratorStatusActive,
			CreatedAt:   now,
			LastUsedAt:  now,
			UnusedSince: now,
		})
	}
	if err != nil {
		return err
	}
	if row.Status == skills.CuratorStatusArchived {
		if err := skills.RestoreArchivedSkill(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		row.ArchivedAt = 0
	}
	row.Status = skills.CuratorStatusActive
	row.LastUsedAt = now
	row.UnusedSince = now
	return s.save(ctx, row)
}

// SetPinned pins or unpins a self-improvement skill. Unpin restarts the clock.
func (s *Service) SetPinned(ctx context.Context, name string, pinned bool) error {
	row, err := s.q.GetSkillCuratorState(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		now := s.now().UnixMilli()
		row = db.SkillCuratorState{
			SkillName: name,
			Origin:    skills.OriginSelfImprovement,
			Status:    skills.CuratorStatusActive,
			CreatedAt: now,
		}
	} else if err != nil {
		return err
	}
	now := s.now().UnixMilli()
	if pinned {
		row.Pinned = 1
		row.PinnedAt = now
	} else {
		row.Pinned = 0
		row.PinnedAt = 0
		row.Status = skills.CuratorStatusActive
		row.UnusedSince = now
		row.ArchivedAt = 0
	}
	return s.save(ctx, row)
}

// Restore moves an archived skill back and restarts its clock.
func (s *Service) Restore(ctx context.Context, name string) error {
	if err := skills.RestoreArchivedSkill(name); err != nil {
		return err
	}
	return s.NoteUse(ctx, name)
}

// MarkArchived records a skill that the merge runtime already moved.
func (s *Service) MarkArchived(ctx context.Context, name string) error {
	row, err := s.q.GetSkillCuratorState(ctx, name)
	if err != nil {
		return err
	}
	if row.Pinned != 0 {
		return nil
	}
	now := s.now().UnixMilli()
	row.Status = skills.CuratorStatusArchived
	row.ArchivedAt = now
	row.DeleteNotifiedAt = now
	return s.save(ctx, row)
}

// Lookup returns status and pinned for a skill name.
func (s *Service) Lookup(ctx context.Context, name string) (status string, pinned bool) {
	row, err := s.q.GetSkillCuratorState(ctx, name)
	if err != nil {
		return "", false
	}
	return row.Status, row.Pinned != 0
}

// ArchivedNames lists archived curator rows.
func (s *Service) ArchivedNames(ctx context.Context) ([]string, error) {
	rows, err := s.q.ListSkillCuratorStates(ctx)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, row := range rows {
		if row.Status == skills.CuratorStatusArchived {
			names = append(names, row.SkillName)
		}
	}
	return names, nil
}

func (s *Service) save(ctx context.Context, row db.SkillCuratorState) error {
	return s.q.UpsertSkillCuratorState(ctx, db.UpsertSkillCuratorStateParams{
		SkillName:        row.SkillName,
		Origin:           row.Origin,
		Status:           row.Status,
		Pinned:           row.Pinned,
		CreatedAt:        row.CreatedAt,
		LastUsedAt:       row.LastUsedAt,
		UnusedSince:      row.UnusedSince,
		ArchivedAt:       row.ArchivedAt,
		PinnedAt:         row.PinnedAt,
		DeleteNotifiedAt: row.DeleteNotifiedAt,
	})
}

func unixMilli(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
