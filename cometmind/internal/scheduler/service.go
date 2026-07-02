package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/id"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/robfig/cron/v3"
)

var (
	ErrNotFound     = errors.New("scheduled job not found")
	ErrConflict     = errors.New("scheduled job conflict")
	ErrInvalidInput = errors.New("invalid scheduled job")
)

type Service struct {
	q *db.Queries
}

func NewService(conn *sql.DB) *Service {
	return &Service{q: db.New(conn)}
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
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

func scheduledJobFromRow(row db.ScheduledJob) ScheduledJob {
	return ScheduledJob{
		ID:               row.ID,
		Description:      row.Description,
		DefinitionOfDone: row.DefinitionOfDone,
		WorkspacePath:    nullStringVal(row.WorkspacePath),
		CreatedBy:        row.CreatedBy,
		SourceSessionID:  nullStringVal(row.SourceSessionID),
		SourcePlatform:   row.SourcePlatform,
		SourceChannelID:  nullStringVal(row.SourceChannelID),
		CronExpr:         nullStringVal(row.CronExpr),
		RunAt:            nullInt64Ptr(row.RunAt),
		NextRunAt:        row.NextRunAt,
		LastRunAt:        nullInt64Ptr(row.LastRunAt),
		Enabled:          row.Enabled != 0,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func validateSchedule(description, cronExpr string, runAt int64) error {
	if strings.TrimSpace(description) == "" {
		return fmt.Errorf("%w: description is required", ErrInvalidInput)
	}
	cronExpr = strings.TrimSpace(cronExpr)
	if cronExpr != "" && runAt > 0 {
		return fmt.Errorf("%w: provide either cron_expr or run_at, not both", ErrInvalidInput)
	}
	if cronExpr == "" && runAt <= 0 {
		return fmt.Errorf("%w: either cron_expr or run_at is required", ErrInvalidInput)
	}
	if cronExpr != "" {
		if _, err := cron.ParseStandard(cronExpr); err != nil {
			return fmt.Errorf("%w: invalid cron_expr: %v", ErrInvalidInput, err)
		}
	}
	return nil
}

func nextCronRun(cronExpr string, from time.Time) (int64, error) {
	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid cron_expr: %v", ErrInvalidInput, err)
	}
	return sched.Next(from).UnixMilli(), nil
}

func validateCreatedBy(v string) error {
	switch v {
	case CreatedByUser, CreatedByAgent:
		return nil
	default:
		return fmt.Errorf("%w: created_by must be 'user' or 'agent'", ErrInvalidInput)
	}
}

func validateSourcePlatform(v string) error {
	switch v {
	case "", PlatformDesktop, PlatformDiscord:
		return nil
	default:
		return fmt.Errorf("%w: source_platform must be '', 'desktop', or 'discord'", ErrInvalidInput)
	}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (ScheduledJob, error) {
	if err := validateSchedule(in.Description, in.CronExpr, in.RunAt); err != nil {
		return ScheduledJob{}, err
	}
	createdBy := strings.TrimSpace(in.CreatedBy)
	if createdBy == "" {
		createdBy = CreatedByUser
	}
	if err := validateCreatedBy(createdBy); err != nil {
		return ScheduledJob{}, err
	}
	sourcePlatform := strings.TrimSpace(in.SourcePlatform)
	if err := validateSourcePlatform(sourcePlatform); err != nil {
		return ScheduledJob{}, err
	}
	cronExpr := strings.TrimSpace(in.CronExpr)
	var runAtVal sql.NullInt64
	var nextRunAt int64
	if cronExpr != "" {
		nr, err := nextCronRun(cronExpr, time.Now())
		if err != nil {
			return ScheduledJob{}, err
		}
		nextRunAt = nr
	} else {
		runAtVal = sql.NullInt64{Int64: in.RunAt, Valid: true}
		nextRunAt = in.RunAt
	}
	ts := nowMillis()
	scheduleID := id.New()
	if err := s.q.InsertScheduledJob(ctx, db.InsertScheduledJobParams{
		ID:               scheduleID,
		Description:      strings.TrimSpace(in.Description),
		DefinitionOfDone: strings.TrimSpace(in.DefinitionOfDone),
		WorkspacePath:    optionalNullString(in.WorkspacePath),
		CreatedBy:        createdBy,
		SourceSessionID:  optionalNullString(in.SourceSessionID),
		SourcePlatform:   sourcePlatform,
		SourceChannelID:  optionalNullString(in.SourceChannelID),
		CronExpr:         optionalNullString(cronExpr),
		RunAt:            runAtVal,
		NextRunAt:        nextRunAt,
		LastRunAt:        sql.NullInt64{},
		Enabled:          1,
		CreatedAt:        ts,
		UpdatedAt:        ts,
	}); err != nil {
		return ScheduledJob{}, err
	}
	return s.Get(ctx, scheduleID)
}

func (s *Service) Get(ctx context.Context, scheduleID string) (ScheduledJob, error) {
	row, err := s.q.GetScheduledJob(ctx, scheduleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ScheduledJob{}, ErrNotFound
		}
		return ScheduledJob{}, err
	}
	return scheduledJobFromRow(row), nil
}

func (s *Service) List(ctx context.Context) ([]ScheduledJob, error) {
	rows, err := s.q.ListScheduledJobs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ScheduledJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduledJobFromRow(row))
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, scheduleID string, in UpdateInput) (ScheduledJob, error) {
	if err := validateSchedule(in.Description, "", in.RunAt); err != nil {
		return ScheduledJob{}, err
	}
	enabled := int64(0)
	if in.Enabled {
		enabled = 1
	}
	n, err := s.q.UpdateScheduledJob(ctx, db.UpdateScheduledJobParams{
		Description:      strings.TrimSpace(in.Description),
		DefinitionOfDone: strings.TrimSpace(in.DefinitionOfDone),
		WorkspacePath:    optionalNullString(in.WorkspacePath),
		RunAt:            sql.NullInt64{Int64: in.RunAt, Valid: true},
		NextRunAt:        in.RunAt,
		Enabled:          enabled,
		UpdatedAt:        nowMillis(),
		ID:               scheduleID,
	})
	if err != nil {
		return ScheduledJob{}, err
	}
	if n == 0 {
		return ScheduledJob{}, ErrNotFound
	}
	return s.Get(ctx, scheduleID)
}

func (s *Service) Delete(ctx context.Context, scheduleID string) error {
	n, err := s.q.DeleteScheduledJob(ctx, scheduleID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) DueNow(ctx context.Context, atMillis int64) ([]ScheduledJob, error) {
	if atMillis <= 0 {
		atMillis = nowMillis()
	}
	rows, err := s.q.ListDueScheduledJobs(ctx, atMillis)
	if err != nil {
		return nil, err
	}
	out := make([]ScheduledJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduledJobFromRow(row))
	}
	return out, nil
}

func (s *Service) MarkFired(ctx context.Context, scheduleID string, firedAt int64) error {
	if firedAt <= 0 {
		firedAt = nowMillis()
	}
	n, err := s.q.MarkScheduledJobFired(ctx, db.MarkScheduledJobFiredParams{
		LastRunAt: sql.NullInt64{Int64: firedAt, Valid: true},
		UpdatedAt: firedAt,
		ID:        scheduleID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Service) AdvanceRecurring(ctx context.Context, scheduleID string, firedAt, nextRunAt int64) error {
	if firedAt <= 0 {
		firedAt = nowMillis()
	}
	n, err := s.q.AdvanceScheduledJob(ctx, db.AdvanceScheduledJobParams{
		LastRunAt: sql.NullInt64{Int64: firedAt, Valid: true},
		NextRunAt: nextRunAt,
		UpdatedAt: firedAt,
		ID:        scheduleID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Service) MaterializeDue(ctx context.Context, jobSvc *jobs.Service, atMillis int64) (int, error) {
	if jobSvc == nil {
		return 0, fmt.Errorf("jobs service is required")
	}
	if atMillis <= 0 {
		atMillis = nowMillis()
	}
	due, err := s.DueNow(ctx, atMillis)
	if err != nil {
		return 0, err
	}
	created := 0
	for _, item := range due {
		if item.CronExpr != "" {
			nextRun, err := nextCronRun(item.CronExpr, time.UnixMilli(atMillis))
			if err != nil {
				return created, err
			}
			if err := s.AdvanceRecurring(ctx, item.ID, atMillis, nextRun); err != nil {
				if err == ErrConflict {
					continue
				}
				return created, err
			}
		} else {
			if err := s.MarkFired(ctx, item.ID, atMillis); err != nil {
				if err == ErrConflict {
					continue
				}
				return created, err
			}
		}
		if _, err := jobSvc.Create(ctx, jobs.CreateInput{
			Description:      item.Description,
			DefinitionOfDone: item.DefinitionOfDone,
			WorkspacePath:    item.WorkspacePath,
			CreatedBy:        item.CreatedBy,
			SourceSessionID:  item.SourceSessionID,
			SourcePlatform:   item.SourcePlatform,
			SourceChannelID:  item.SourceChannelID,
		}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
