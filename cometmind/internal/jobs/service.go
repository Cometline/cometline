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
		ID:               row.ID,
		Description:      row.Description,
		DefinitionOfDone: row.DefinitionOfDone,
		Progress:         row.Progress,
		Status:           row.Status,
		Priority:         int(row.Priority),
		ScheduledAt:      nullInt64Ptr(row.ScheduledAt),
		DueAt:            nullInt64Ptr(row.DueAt),
		WorkspacePath:    nullStringVal(row.WorkspacePath),
		AssignedSessionID: nullStringVal(row.AssignedSessionID),
		LeaseExpiresAt:   nullInt64Ptr(row.LeaseExpiresAt),
		CreatedBy:        row.CreatedBy,
		SourceSessionID:  nullStringVal(row.SourceSessionID),
		SourcePlatform:   row.SourcePlatform,
		SourceChannelID:  nullStringVal(row.SourceChannelID),
		DeletedAt:        nullInt64Ptr(row.DeletedAt),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
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

func optionalNullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
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
		ID:               jobID,
		Description:      strings.TrimSpace(in.Description),
		DefinitionOfDone: strings.TrimSpace(in.DefinitionOfDone),
		Progress:         "",
		Status:           StatusTodo,
		Priority:         int64(in.Priority),
		ScheduledAt:      optionalNullInt64(in.ScheduledAt),
		DueAt:            optionalNullInt64(in.DueAt),
		WorkspacePath:    optionalNullString(in.WorkspacePath),
		AssignedSessionID: sql.NullString{},
		LeaseExpiresAt:   sql.NullInt64{},
		CreatedBy:        createdBy,
		SourceSessionID:  optionalNullString(in.SourceSessionID),
		SourcePlatform:   strings.TrimSpace(in.SourcePlatform),
		SourceChannelID:  optionalNullString(in.SourceChannelID),
		DeletedAt:        sql.NullInt64{},
		CreatedAt:        ts,
		UpdatedAt:        ts,
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
	rows, err := s.q.ListJobs(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(rows))
	for _, row := range rows {
		if filter.IncludeDeleted {
			out = append(out, jobFromRow(row))
			continue
		}
		if row.DeletedAt.Valid {
			continue
		}
		out = append(out, jobFromRow(row))
	}
	return out, nil
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
		Priority:         int64(in.Priority),
		ScheduledAt:      optionalNullInt64(in.ScheduledAt),
		DueAt:            optionalNullInt64(in.DueAt),
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
	})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		if job, getErr := s.Get(ctx, jobID); getErr == nil {
			if job.Status == StatusOngoing {
				return Job{}, ErrAlreadyClaimed
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

// Release returns an ongoing job to todo.
func (s *Service) Release(ctx context.Context, jobID, sessionID, reason string) (Job, error) {
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
	n, err := s.q.ReleaseJob(ctx, db.ReleaseJobParams{UpdatedAt: ts, ID: jobID})
	if err != nil {
		return Job{}, err
	}
	if n == 0 {
		return Job{}, ErrConflict
	}
	action := EventReleased
	detail := strings.TrimSpace(reason)
	_ = s.recordEvent(ctx, jobID, action, detail, sessionID)
	job, err = s.Get(ctx, jobID)
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

// Heartbeat extends the lease for an ongoing job.
func (s *Service) Heartbeat(ctx context.Context, jobID, sessionID string) error {
	ts := nowMillis()
	leaseUntil := ts + s.leaseDuration().Milliseconds()
	n, err := s.q.HeartbeatJob(ctx, db.HeartbeatJobParams{
		LeaseExpiresAt: sql.NullInt64{Int64: leaseUntil, Valid: true},
		UpdatedAt:      ts,
		ID:             jobID,
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
		if _, err := s.Release(ctx, job.ID, "", reason); err != nil {
			continue
		}
		_ = s.recordEvent(ctx, job.ID, action, reason, job.AssignedSessionID)
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

// ExecutionPrompt builds the agent prompt for running a claimed job.
func ExecutionPrompt(job Job) string {
	dod := strings.TrimSpace(job.DefinitionOfDone)
	if dod == "" {
		dod = "(none specified)"
	}
	return fmt.Sprintf(
		"Please work on: %s\n\nDefinition of done: %s\n\nUpdate progress with `update_job` as you go. When finished, call `complete_job` with a final progress summary.\n\n(Use job_id %q when calling job tools.)",
		job.Description, dod, job.ID,
	)
}
