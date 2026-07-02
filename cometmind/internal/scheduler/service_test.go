package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/store"
)

func newSchedulerTestServices(t *testing.T) (*Service, *jobs.Service) {
	t.Helper()
	ctx := context.Background()
	conn, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return NewService(conn), jobs.NewService(conn, nil, nil)
}

func TestCreateListAndUpdateOneShotSchedule(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSchedulerTestServices(t)
	runAt := int64(2_000_000)

	created, err := svc.Create(ctx, CreateInput{
		Description:      "write report",
		DefinitionOfDone: "report committed",
		WorkspacePath:    "/tmp/workspace",
		RunAt:            runAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Description != "write report" || created.NextRunAt != runAt || !created.Enabled {
		t.Fatalf("created=%+v", created)
	}
	if created.RunAt == nil || *created.RunAt != runAt {
		t.Fatalf("run_at=%v", created.RunAt)
	}

	items, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("items=%+v", items)
	}

	updated, err := svc.Update(ctx, created.ID, UpdateInput{
		Description:      "write better report",
		DefinitionOfDone: "better report committed",
		WorkspacePath:    "/tmp/workspace",
		RunAt:            runAt + 1000,
		Enabled:          false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Description != "write better report" || updated.Enabled || updated.NextRunAt != runAt+1000 {
		t.Fatalf("updated=%+v", updated)
	}
}

func TestMaterializeDueCreatesJobAndDisablesSchedule(t *testing.T) {
	ctx := context.Background()
	svc, jobSvc := newSchedulerTestServices(t)

	created, err := svc.Create(ctx, CreateInput{
		Description:      "run delayed task",
		DefinitionOfDone: "task complete",
		WorkspacePath:    "/tmp/project",
		CreatedBy:        CreatedByAgent,
		SourceSessionID:  "sess-1",
		SourcePlatform:   PlatformDesktop,
		SourceChannelID:  "chan-1",
		RunAt:            1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	due, err := svc.DueNow(ctx, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("due before run_at=%+v", due)
	}

	count, err := svc.MaterializeDue(ctx, jobSvc, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}

	fired, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fired.Enabled || fired.LastRunAt == nil || *fired.LastRunAt != 1000 {
		t.Fatalf("fired=%+v", fired)
	}

	jobsList, err := jobSvc.List(ctx, jobs.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobsList) != 1 {
		t.Fatalf("jobs=%+v", jobsList)
	}
	job := jobsList[0]
	if job.Description != "run delayed task" || job.DefinitionOfDone != "task complete" || job.WorkspacePath != "/tmp/project" {
		t.Fatalf("job=%+v", job)
	}
	if job.CreatedBy != CreatedByAgent || job.SourceSessionID != "sess-1" || job.SourcePlatform != PlatformDesktop || job.SourceChannelID != "chan-1" {
		t.Fatalf("job metadata=%+v", job)
	}

	count, err = svc.MaterializeDue(ctx, jobSvc, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("second count=%d", count)
	}
}

func TestCreateRejectsBothCronAndRunAt(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSchedulerTestServices(t)
	if _, err := svc.Create(ctx, CreateInput{Description: "d", CronExpr: "* * * * *", RunAt: 1000}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateRequiresCronOrRunAt(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSchedulerTestServices(t)
	if _, err := svc.Create(ctx, CreateInput{Description: "d"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRecurringScheduleAdvancesAfterMaterialize(t *testing.T) {
	ctx := context.Background()
	svc, jobSvc := newSchedulerTestServices(t)

	created, err := svc.Create(ctx, CreateInput{
		Description: "weekly report",
		CronExpr:    "0 0 * * 0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Enabled {
		t.Fatal("recurring schedule should be enabled")
	}
	if created.CronExpr != "0 0 * * 0" {
		t.Fatalf("cron_expr=%s", created.CronExpr)
	}

	count, err := svc.MaterializeDue(ctx, jobSvc, created.NextRunAt)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count=%d", count)
	}

	after, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Enabled {
		t.Fatal("recurring schedule should remain enabled after firing")
	}
	if after.NextRunAt <= created.NextRunAt {
		t.Fatal("next_run_at should advance after firing")
	}
	if after.LastRunAt == nil {
		t.Fatal("last_run_at should be set after firing")
	}

	jobs, err := jobSvc.List(ctx, jobs.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 materialized job, got %d", len(jobs))
	}
	if jobs[0].Description != "weekly report" {
		t.Fatalf("job description=%s", jobs[0].Description)
	}
}

func TestCreateRejectsInvalidEnums(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSchedulerTestServices(t)
	if _, err := svc.Create(ctx, CreateInput{Description: "d", RunAt: 1000, CreatedBy: "bad"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad created_by, got %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{Description: "d", RunAt: 1000, SourcePlatform: "web"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad source_platform, got %v", err)
	}
}
