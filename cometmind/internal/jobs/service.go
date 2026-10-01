package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/id"
)

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
		settingsFn = DefaultSettings
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
		ScheduledJobID:    optionalNullString(in.ScheduledJobID),
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

// HasOpenJobForScheduledJob reports whether a scheduled job already has an
// outstanding (todo/ongoing/blocked, non-deleted) materialized job. Used by
// the scheduler to avoid materializing a duplicate job for a recurring
// schedule while a previous firing's job is still unresolved.
func (s *Service) HasOpenJobForScheduledJob(ctx context.Context, scheduledJobID string) (bool, error) {
	scheduledJobID = strings.TrimSpace(scheduledJobID)
	if scheduledJobID == "" {
		return false, nil
	}
	return s.q.HasOpenJobForScheduledJob(ctx, optionalNullString(scheduledJobID))
}

// Get returns one job by id.
func (s *Service) Get(ctx context.Context, jobID string) (Job, error) {
	row, err := s.q.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
