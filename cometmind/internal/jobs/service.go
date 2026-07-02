package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
)

// Notifier handles outbound job status notifications.
type Notifier struct {
	settings func() Settings
	handlers []NotificationHandler
}

// NotificationHandler receives job lifecycle events for external delivery.
type NotificationHandler interface {
	OnJobEvent(ctx context.Context, job Job, action string, detail string)
}

// NewNotifier builds a notifier with dynamic settings.
func NewNotifier(settingsFn func() Settings) *Notifier {
	if settingsFn == nil {
		settingsFn = func() Settings { return DefaultSettings() }
	}
	return &Notifier{settings: settingsFn}
}

// Register adds a notification handler.
func (n *Notifier) Register(h NotificationHandler) {
	if n == nil || h == nil {
		return
	}
	n.handlers = append(n.handlers, h)
}

func (n *Notifier) emit(ctx context.Context, job Job, action, detail string) {
	if n == nil {
		return
	}
	cfg := n.settings().Notifications
	if !cfg.Enabled {
		return
	}
	switch action {
	case EventClaimed:
		if !cfg.OnClaimed {
			return
		}
	case EventCompleted:
		if !cfg.OnCompleted {
			return
		}
	case EventBlocked:
		if !cfg.OnBlocked {
			return
		}
	case EventReleased, EventLeaseExpired:
		if !cfg.OnReleased {
			return
		}
	default:
		return
	}
	for _, h := range n.handlers {
		h.OnJobEvent(ctx, job, action, detail)
	}
}

// Service manages the global jobs queue.
type Service struct {
	q        *db.Queries
	settings func() Settings
	notifier *Notifier
}

// Notifier returns the service notifier for registering handlers.
func (s *Service) Notifier() *Notifier {
	if s == nil {
		return nil
	}
	return s.notifier
}

// NewService creates a jobs service.
func NewService(conn *sql.DB, settingsFn func() Settings, notifier *Notifier) *Service {
	if settingsFn == nil {
		settingsFn = func() Settings { return DefaultSettings() }
	}
	return &Service{
		q:        db.New(conn),
		settings: settingsFn,
		notifier: notifier,
	}
}

func (s *Service) Settings() Settings {
	if s == nil {
		return DefaultSettings()
	}
	return s.settings()
}

func (s *Service) leaseDuration() time.Duration {
	mins := s.Settings().LeaseMinutes
	if mins <= 0 {
		mins = DefaultLeaseMinutes
	}
	return time.Duration(mins) * time.Minute
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

func (s *Service) recordEvent(ctx context.Context, jobID, action, detail, actorSessionID string) error {
	var actor sql.NullString
	if strings.TrimSpace(actorSessionID) != "" {
		actor = sql.NullString{String: actorSessionID, Valid: true}
	}
	return s.q.InsertJobEvent(ctx, db.InsertJobEventParams{
		ID:             id.New(),
		JobID:          jobID,
		Action:         action,
		Detail:         detail,
		ActorSessionID: actor,
		CreatedAt:      nowMillis(),
	})
}

func jobFromRow(row db.Job) Job {
	return Job{
		ID:                row.ID,
		Description:       row.Description,
		DefinitionOfDone:  row.DefinitionOfDone,
		Progress:          row.Progress,
		Status:            row.Status,
		WorkspacePath:     nullStringVal(row.WorkspacePath),
		AssignedSessionID: nullStringVal(row.AssignedSessionID),
		LeaseExpiresAt:    nullInt64Ptr(row.LeaseExpiresAt),
		CreatedBy:         row.CreatedBy,
		SourceSessionID:   nullStringVal(row.SourceSessionID),
		SourcePlatform:    row.SourcePlatform,
		SourceChannelID:   nullStringVal(row.SourceChannelID),
		ArchivedAt:        nullInt64Ptr(row.ArchivedAt),
		FailureCount:      row.FailureCount,
		NextRetryAt:       nullInt64Ptr(row.NextRetryAt),
		LastFailureReason: nullStringVal(row.LastFailureReason),
		DeletedAt:         nullInt64Ptr(row.DeletedAt),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func eventFromRow(row db.JobEvent) JobEvent {
	return JobEvent{
		ID:             row.ID,
		JobID:          row.JobID,
		Action:         row.Action,
		Detail:         row.Detail,
		ActorSessionID: nullStringVal(row.ActorSessionID),
		CreatedAt:      row.CreatedAt,
	}
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func nullStringVal(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func optionalNullString(v string) sql.NullString {
	v = strings.TrimSpace(v)
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

// Create inserts a new todo job.
func (s *Service) Create(ctx context.Context, in CreateInput) (Job, error) {
	if strings.TrimSpace(in.Description) == "" {
		return Job{}, fmt.Errorf("description is required")
	}
	createdBy := strings.TrimSpace(in.CreatedBy)
	if createdBy == "" {
		createdBy = CreatedByUser
	}
	ts := nowMillis()
	jobID := id.New()
	row := db.InsertJobParams{
		ID:                jobID,
		Description:       strings.TrimSpace(in.Description),
		DefinitionOfDone:  strings.TrimSpace(in.DefinitionOfDone),
		Progress:          "",
		Status:            StatusTodo,
		WorkspacePath:     optionalNullString(in.WorkspacePath),
		AssignedSessionID: sql.NullString{},
		LeaseExpiresAt:    sql.NullInt64{},
		CreatedBy:         createdBy,
		SourceSessionID:   optionalNullString(in.SourceSessionID),
		SourcePlatform:    strings.TrimSpace(in.SourcePlatform),
		SourceChannelID:   optionalNullString(in.SourceChannelID),
		ArchivedAt:        sql.NullInt64{},
		FailureCount:      0,
		NextRetryAt:       sql.NullInt64{},
		LastFailureReason: sql.NullString{},
		DeletedAt:         sql.NullInt64{},
		CreatedAt:         ts,
		UpdatedAt:         ts,
	}
	if err := s.q.InsertJob(ctx, row); err != nil {
		return Job{}, err
	}
	_ = s.recordEvent(ctx, jobID, EventCreated, "", in.SourceSessionID)
	job, err := s.Get(ctx, jobID)
	return job, err
}

// Get returns one job by id.
func (s *Service) Get(ctx context.Context, jobID string) (Job, error) {
	row, err := s.q.GetJob(ctx, jobID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	return jobFromRow(row), nil
}

// List returns jobs matching the filter.
func (s *Service) List(ctx context.Context, filter ListFilter) ([]Job, error) {
	if filter.ReadyOnly {
		return s.ListReady(ctx)
	}
	var status sql.NullString
	if strings.TrimSpace(filter.Status) != "" {
		status = sql.NullString{String: filter.Status, Valid: true}
	}
	var includeDeleted sql.NullInt64
	if filter.IncludeDeleted {
		includeDeleted = sql.NullInt64{Int64: 1, Valid: true}
	}
	var includeArchived sql.NullInt64
	if filter.IncludeArchived {
		includeArchived = sql.NullInt64{Int64: 1, Valid: true}
	}
	rows, err := s.q.ListJobs(ctx, db.ListJobsParams{
		Status:          status,
		IncludeDeleted:  includeDeleted,
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(rows))
	for _, row := range rows {
		out = append(out, jobFromRow(row))
	}
	return out, nil
}

// Archive hides a completed job from the active board without deleting it.
func (s *Service) Archive(ctx context.Context, jobID string) (Job, error) {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusDone || job.DeletedAt != nil {
		return Job{}, ErrConflict
	}
	if job.ArchivedAt != nil {
		return job, nil
	}
	ts := nowMillis()
	n, err := s.q.ArchiveJob(ctx, db.ArchiveJobParams{
		ArchivedAt: sql.NullInt64{Int64: ts, Valid: true},
		UpdatedAt:  ts,
		ID:         jobID,
	})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	_ = s.recordEvent(ctx, jobID, EventArchived, "", "")
	return s.Get(ctx, jobID)
}

// Unarchive restores an archived job to normal listings.
func (s *Service) Unarchive(ctx context.Context, jobID string) (Job, error) {
	if _, err := s.Get(ctx, jobID); err != nil {
		return Job{}, err
	}
	ts := nowMillis()
	n, err := s.q.UnarchiveJob(ctx, db.UnarchiveJobParams{UpdatedAt: ts, ID: jobID})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	_ = s.recordEvent(ctx, jobID, EventUnarchived, "", "")
	return s.Get(ctx, jobID)
}

// ListReady returns todo jobs that are ready to be claimed.
func (s *Service) ListReady(ctx context.Context) ([]Job, error) {
	_, _ = s.Reconcile(ctx, nil)
	rows, err := s.q.ListReadyJobs(ctx, sql.NullInt64{Int64: nowMillis(), Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(rows))
	for _, row := range rows {
		out = append(out, jobFromRow(row))
	}
	return out, nil
}

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

// ListEvents returns audit events for a job.
func (s *Service) ListEvents(ctx context.Context, jobID string) ([]JobEvent, error) {
	rows, err := s.q.ListJobEvents(ctx, jobID)
	if err != nil {
		return nil, err
	}
	out := make([]JobEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, eventFromRow(row))
	}
	return out, nil
}

// UpdateTodo updates editable fields for a todo job.
func (s *Service) UpdateTodo(ctx context.Context, jobID string, in UpdateTodoInput, actorSessionID string) (Job, error) {
	if strings.TrimSpace(in.Description) == "" {
		return Job{}, fmt.Errorf("description is required")
	}
	ts := nowMillis()
	n, err := s.q.UpdateJobTodoFields(ctx, db.UpdateJobTodoFieldsParams{
		Description:      strings.TrimSpace(in.Description),
		DefinitionOfDone: strings.TrimSpace(in.DefinitionOfDone),
		WorkspacePath:    optionalNullString(in.WorkspacePath),
		UpdatedAt:        ts,
		ID:               jobID,
	})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		if _, err := s.Get(ctx, jobID); err != nil {
			return Job{}, err
		}
		return Job{}, ErrNotEditable
	}
	_ = s.recordEvent(ctx, jobID, EventUpdated, "todo fields", actorSessionID)
	return s.Get(ctx, jobID)
}

// UpdateProgress updates progress for an ongoing job.
func (s *Service) UpdateProgress(ctx context.Context, jobID, progress, sessionID string) (Job, error) {
	job, err := s.Get(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if job.Status != StatusOngoing {
		return Job{}, ErrNotEditable
	}
	if job.AssignedSessionID != sessionID {
		return Job{}, ErrNotAssigned
	}
	ts := nowMillis()
	n, err := s.q.UpdateJobProgress(ctx, db.UpdateJobProgressParams{
		Progress:  progress,
		UpdatedAt: ts,
		ID:        jobID,
	})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrNotEditable
	}
	_ = s.recordEvent(ctx, jobID, EventUpdated, "progress", sessionID)
	return s.Get(ctx, jobID)
}

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

// JobForSession returns the ongoing job assigned to a session, if any.
func (s *Service) JobForSession(ctx context.Context, sessionID string) (Job, bool, error) {
	row, err := s.q.GetJobByAssignedSession(ctx, sql.NullString{String: sessionID, Valid: true})
	if err != nil {
		if err == sql.ErrNoRows {
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

// Reconcile releases orphan or expired ongoing jobs.
func (s *Service) Reconcile(ctx context.Context, isRunning func(sessionID string) bool) (int, error) {
	rows, err := s.q.ListOngoingJobs(ctx)
	if err != nil {
		return 0, err
	}
	now := nowMillis()
	released := 0
	for _, row := range rows {
		job := jobFromRow(row)
		orphan := false
		expired := job.LeaseExpiresAt != nil && *job.LeaseExpiresAt < now
		if isRunning != nil && job.AssignedSessionID != "" {
			orphan = !isRunning(job.AssignedSessionID)
		}
		if !orphan && !expired {
			continue
		}
		action := EventReleased
		reason := "orphan session"
		if expired && !orphan {
			action = EventLeaseExpired
			reason = "lease expired"
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
