package jobs

import (
	"context"
	"database/sql"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

// StaleOngoing returns ongoing jobs that have not changed since staleAfter.
func (s *Service) StaleOngoing(ctx context.Context, staleAfter time.Duration) ([]Job, error) {
	if staleAfter <= 0 {
		staleAfter = DefaultStaleReviewMinutes * time.Minute
	}
	rows, err := s.q.ListOngoingJobs(ctx)
	if err != nil {
		return nil, err
	}
	cutoff := nowMillis() - staleAfter.Milliseconds()
	out := make([]Job, 0)
	for _, row := range rows {
		job := jobFromRow(row)
		if job.UpdatedAt < cutoff {
			out = append(out, job)
		}
	}
	return out, nil
}

// PurgeArchived hard-deletes archived jobs older than the cutoff.
func (s *Service) PurgeArchived(ctx context.Context, olderThanDays int) (int, error) {
	if olderThanDays <= 0 {
		return 0, nil
	}
	cutoff := nowMillis() - int64(olderThanDays)*24*60*60*1000
	ids, err := s.q.ListArchivedJobsBefore(ctx, sql.NullInt64{Int64: cutoff, Valid: true})
	if err != nil {
		return 0, err
	}
	for _, jobID := range ids {
		if err := s.q.HardDeleteJob(ctx, jobID); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// ArchiveDone archives completed jobs older than the cutoff without deleting them.
func (s *Service) ArchiveDone(ctx context.Context, olderThanDays int) (int, error) {
	if olderThanDays <= 0 {
		return 0, nil
	}
	cutoff := nowMillis() - int64(olderThanDays)*24*60*60*1000
	ids, err := s.q.ListDoneJobsBefore(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	archived := 0
	for _, jobID := range ids {
		ts := nowMillis()
		n, err := s.q.ArchiveJob(ctx, db.ArchiveJobParams{
			ArchivedAt: sql.NullInt64{Int64: ts, Valid: true},
			UpdatedAt:  ts,
			ID:         jobID,
		})
		if err != nil {
			return archived, err
		}
		if n == 0 {
			continue
		}
		archived++
		_ = s.recordEvent(ctx, jobID, EventArchived, "auto-archive", "")
	}
	return archived, nil
}

// SoftDelete marks a job as deleted.
func (s *Service) SoftDelete(ctx context.Context, jobID string) error {
	ts := nowMillis()
	n, err := s.q.SoftDeleteJob(ctx, db.SoftDeleteJobParams{
		DeletedAt: sql.NullInt64{Int64: ts, Valid: true},
		UpdatedAt: ts,
		ID:        jobID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	_ = s.recordEvent(ctx, jobID, EventDeleted, "", "")
	return nil
}

// PurgeDeleted hard-deletes soft-deleted jobs older than the cutoff.
func (s *Service) PurgeDeleted(ctx context.Context, olderThanDays int) (int, error) {
	if olderThanDays <= 0 {
		return 0, nil
	}
	cutoff := nowMillis() - int64(olderThanDays)*24*60*60*1000
	ids, err := s.q.ListDeletedJobsBefore(ctx, sql.NullInt64{Int64: cutoff, Valid: true})
	if err != nil {
		return 0, err
	}
	for _, jobID := range ids {
		if err := s.q.HardDeleteJob(ctx, jobID); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}
