package jobs

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

// Claim assigns a todo job to a session.
func (s *Service) Claim(ctx context.Context, jobID, sessionID string) (Job, error) {
	_, _ = s.Reconcile(ctx, nil)
	ts := nowMillis()
	leaseUntil := ts + s.leaseDuration().Milliseconds()
	n, err := s.q.ClaimJob(ctx, db.ClaimJobParams{
		AssignedSessionID: sql.NullString{String: sessionID, Valid: true},
		LeaseExpiresAt:    sql.NullInt64{Int64: leaseUntil, Valid: true},
		UpdatedAt:         ts,
		ID:                jobID,
		NextRetryAt:       sql.NullInt64{Int64: ts, Valid: true},
	})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		if job, getErr := s.Get(ctx, jobID); getErr == nil {
			if job.Status == StatusOngoing {
				return Job{}, ErrAlreadyClaimed
			}
			if job.Status == StatusBlocked {
				return Job{}, ErrConflict
			}
			if job.DeletedAt != nil {
				return Job{}, ErrNotFound
			}
		}
		return Job{}, ErrConflict
	}
	_ = s.recordEvent(ctx, jobID, EventClaimed, "", sessionID)
	job, err := s.Get(ctx, jobID)
	if err == nil && s.notifier != nil {
		s.notifier.emit(ctx, job, EventClaimed, "")
	}
	return job, err
}

// Release returns an ongoing job to todo for an agent handoff.
func (s *Service) Release(ctx context.Context, jobID, sessionID, reason string) (Job, error) {
	return s.ReleaseWithClass(ctx, jobID, sessionID, reason, FailureAgentHandoff)
}

// ReleaseWithClass releases an ongoing job and optionally records worker failure state.
func (s *Service) ReleaseWithClass(ctx context.Context, jobID, sessionID, reason string, class FailureClass) (Job, error) {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusOngoing {
		return Job{}, ErrConflict
	}
	if sessionID != "" && job.AssignedSessionID != sessionID {
		return Job{}, ErrNotAssigned
	}
	ts := nowMillis()
	detail := strings.TrimSpace(reason)
	if class == FailureWorkerError {
		return s.recordWorkerFailure(ctx, job, sessionID, detail, ts)
	}
	n, err := s.q.ReleaseJob(ctx, db.ReleaseJobParams{UpdatedAt: ts, ID: jobID})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	action := EventReleased
	if class == FailureInfra && detail == "lease expired" {
		action = EventLeaseExpired
	}
	_ = s.recordEvent(ctx, jobID, action, detail, sessionID)
	job, err = s.Get(ctx, jobID)
	if err == nil && s.notifier != nil {
		s.notifier.emit(ctx, job, action, detail)
	}
	return job, err
}

func (s *Service) retryCooldown(failureCount int64) time.Duration {
	settings := s.Settings()
	unit := settings.RetryCooldownMinutes
	if unit <= 0 {
		unit = DefaultRetryCooldownMins
	}
	maxMinutes := settings.MaxRetryCooldownMinutes
	if maxMinutes <= 0 {
		maxMinutes = DefaultMaxRetryCooldown
	}
	minutes := int(failureCount) * unit
	if minutes > maxMinutes {
		minutes = maxMinutes
	}
	return time.Duration(minutes) * time.Minute
}

func (s *Service) maxConsecutiveFailures() int64 {
	maxFailures := s.Settings().MaxConsecutiveFailures
	if maxFailures <= 0 {
		maxFailures = DefaultMaxFailures
	}
	return int64(maxFailures)
}

func (s *Service) recordWorkerFailure(ctx context.Context, job Job, sessionID, detail string, ts int64) (Job, error) {
	nextFailureCount := job.FailureCount + 1
	nextStatus := StatusTodo
	action := EventFailed
	var nextRetryAt sql.NullInt64
	if nextFailureCount >= s.maxConsecutiveFailures() {
		nextStatus = StatusBlocked
		action = EventBlocked
	} else {
		nextRetryAt = sql.NullInt64{Int64: ts + s.retryCooldown(nextFailureCount).Milliseconds(), Valid: true}
	}
	n, err := s.q.RecordJobFailure(ctx, db.RecordJobFailureParams{
		NextRetryAt:       nextRetryAt,
		LastFailureReason: optionalNullString(detail),
		Status:            nextStatus,
		UpdatedAt:         ts,
		ID:                job.ID,
	})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	_ = s.recordEvent(ctx, job.ID, action, detail, sessionID)
	job, err = s.Get(ctx, job.ID)
	if err == nil && s.notifier != nil {
		s.notifier.emit(ctx, job, action, detail)
	}
	return job, err
}

// Complete marks an ongoing job as done.
func (s *Service) Complete(ctx context.Context, jobID, sessionID, progress string) (Job, error) {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusOngoing {
		return Job{}, ErrConflict
	}
	if job.AssignedSessionID != sessionID {
		return Job{}, ErrNotAssigned
	}
	if strings.TrimSpace(progress) != "" {
		if _, err := s.UpdateProgress(ctx, jobID, progress, sessionID); err != nil {
			return Job{}, err
		}
	}
	ts := nowMillis()
	n, err := s.q.CompleteJob(ctx, db.CompleteJobParams{UpdatedAt: ts, ID: jobID})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	_ = s.recordEvent(ctx, jobID, EventCompleted, "", sessionID)
	job, err = s.Get(ctx, jobID)
	if err == nil && s.notifier != nil {
		s.notifier.emit(ctx, job, EventCompleted, job.Progress)
	}
	return job, err
}

// Unblock resets retry state for a blocked job so it can be claimed again.
func (s *Service) Unblock(ctx context.Context, jobID string) (Job, error) {
	if _, err := s.Get(ctx, jobID); err != nil {
		return Job{}, err
	}
	ts := nowMillis()
	n, err := s.q.ResetJobFailures(ctx, db.ResetJobFailuresParams{UpdatedAt: ts, ID: jobID})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	_ = s.recordEvent(ctx, jobID, EventReleased, "retry now", "")
	return s.Get(ctx, jobID)
}

// Heartbeat extends the lease for an ongoing job.
func (s *Service) Heartbeat(ctx context.Context, jobID, sessionID string) error {
	ts := nowMillis()
	leaseUntil := ts + s.leaseDuration().Milliseconds()
	n, err := s.q.HeartbeatJob(ctx, db.HeartbeatJobParams{
		LeaseExpiresAt:    sql.NullInt64{Int64: leaseUntil, Valid: true},
		UpdatedAt:         ts,
		ID:                jobID,
		AssignedSessionID: sql.NullString{String: sessionID, Valid: true},
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// JobForSession returns the ongoing job assigned to a session, if any.
func (s *Service) JobForSession(ctx context.Context, sessionID string) (Job, bool, error) {
	row, err := s.q.GetJobByAssignedSession(ctx, sql.NullString{String: sessionID, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Job{}, false, nil
		}
		return Job{}, false, err
	}
	return jobFromRow(row), true, nil
}

// ReleaseForSession releases any ongoing job held by the session.
func (s *Service) ReleaseForSession(ctx context.Context, sessionID, reason string) error {
	job, ok, err := s.JobForSession(ctx, sessionID)
	if err != nil || !ok {
		return err
	}
	_, err = s.Release(ctx, job.ID, sessionID, reason)
	return err
}

// Reconcile releases ongoing jobs whose lease has expired.
//
// isRunning is used only to annotate *why* the lease expired (the assigned
// session is gone vs. a still-live session whose heartbeat failed to renew
// the lease in time) — it does not by itself trigger a release. A job's
// session legitimately has no in-flight HTTP/agent turn between chat
// messages (e.g. waiting on the next user message or the next scheduler
// tick), so treating "no active turn right now" as orphaned would release
// jobs that are still being worked, causing them to bounce back to todo and
// be re-claimed in a loop even though the agent eventually completes them.
// The lease (kept alive by periodic heartbeats while the job is actively
// worked, see internal/jobs/heartbeat.go and internal/autonomy/worker.go)
// is the single source of truth for liveness.
func (s *Service) Reconcile(ctx context.Context, isRunning func(sessionID string) bool) (int, error) {
	rows, err := s.q.ListOngoingJobs(ctx)
	if err != nil {
		return 0, err
	}
	now := nowMillis()
	released := 0
	for _, row := range rows {
		job := jobFromRow(row)
		expired := job.LeaseExpiresAt != nil && *job.LeaseExpiresAt < now
		if !expired {
			continue
		}
		action := EventLeaseExpired
		reason := "lease expired"
		if isRunning != nil && job.AssignedSessionID != "" && !isRunning(job.AssignedSessionID) {
			action = EventReleased
			reason = "orphan session"
		}
		if _, err := s.ReleaseWithClass(ctx, job.ID, "", reason, FailureInfra); err != nil {
			continue
		}
		if action != EventLeaseExpired {
			_ = s.recordEvent(ctx, job.ID, action, reason, job.AssignedSessionID)
		}
		released++
	}
	return released, nil
}
