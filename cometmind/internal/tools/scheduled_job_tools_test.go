package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/scheduler"
	_ "modernc.org/sqlite"
)

func testSchedulerService(t *testing.T) *scheduler.Service {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return scheduler.NewService(conn)
}

func TestParseScheduledRunAt(t *testing.T) {
	t.Parallel()
	ms, err := parseScheduledRunAt(1_700_000_000_000, "")
	if err != nil || ms != 1_700_000_000_000 {
		t.Fatalf("run_at only: ms=%d err=%v", ms, err)
	}
	iso := "2026-07-10T15:00:00+08:00"
	ms, err = parseScheduledRunAt(0, iso)
	if err != nil {
		t.Fatal(err)
	}
	want, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t.Fatal(err)
	}
	if ms != want.UnixMilli() {
		t.Fatalf("iso ms=%d want %d", ms, want.UnixMilli())
	}
	if _, err := parseScheduledRunAt(1, iso); err == nil {
		t.Fatal("expected error when both set")
	}
	if _, err := parseScheduledRunAt(0, "not-a-time"); err == nil {
		t.Fatal("expected invalid iso error")
	}
	ms, err = parseScheduledRunAt(0, "")
	if err != nil || ms != 0 {
		t.Fatalf("empty: ms=%d err=%v", ms, err)
	}
}

func TestCreateScheduledJobTool_Cron(t *testing.T) {
	svc := testSchedulerService(t)
	tool := createScheduledJobTool{deps: JobsDeps{
		Scheduler:            svc,
		SessionID:            "sess-1",
		SessionWorkspacePath: "/default/ws",
		SourcePlatform:       jobs.PlatformDesktop,
	}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{
		"description":"health check",
		"definition_of_done":"report written",
		"cron_expr":"0 9 * * 1-5"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "Created scheduled job") {
		t.Fatalf("output=%q", res.Output)
	}
	if !strings.Contains(res.Output, `cron "0 9 * * 1-5"`) {
		t.Fatalf("missing cron in output: %q", res.Output)
	}
	if !strings.Contains(res.Output, "workspace: /default/ws") {
		t.Fatalf("missing default workspace: %q", res.Output)
	}
	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d want 1", len(items))
	}
	if items[0].CreatedBy != scheduler.CreatedByAgent {
		t.Fatalf("created_by=%q", items[0].CreatedBy)
	}
	if items[0].CronExpr != "0 9 * * 1-5" {
		t.Fatalf("cron=%q", items[0].CronExpr)
	}
	if items[0].WorkspacePath != "/default/ws" {
		t.Fatalf("workspace=%q", items[0].WorkspacePath)
	}
}

func TestCreateScheduledJobTool_OneShotISO(t *testing.T) {
	svc := testSchedulerService(t)
	tool := createScheduledJobTool{deps: JobsDeps{Scheduler: svc, SessionID: "s1"}}
	iso := "2030-01-15T12:00:00Z"
	body, _ := json.Marshal(map[string]any{
		"description": "summarize logs",
		"run_at_iso":  iso,
	})
	res, err := tool.Execute(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RunAt == nil {
		t.Fatalf("items=%+v", items)
	}
	want, _ := time.Parse(time.RFC3339, iso)
	if *items[0].RunAt != want.UnixMilli() {
		t.Fatalf("run_at=%d want %d", *items[0].RunAt, want.UnixMilli())
	}
}

func TestCreateScheduledJobTool_RejectBothScheduleKinds(t *testing.T) {
	svc := testSchedulerService(t)
	tool := createScheduledJobTool{deps: JobsDeps{Scheduler: svc}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{
		"description":"bad",
		"cron_expr":"0 9 * * *",
		"run_at":2000000
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("expected failure when both cron and run_at set")
	}
}

func TestCreateScheduledJobTool_RequiresSchedule(t *testing.T) {
	svc := testSchedulerService(t)
	tool := createScheduledJobTool{deps: JobsDeps{Scheduler: svc}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"description":"no schedule"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("expected failure without schedule")
	}
}

func TestListScheduledJobsTool(t *testing.T) {
	svc := testSchedulerService(t)
	if _, err := svc.Create(context.Background(), scheduler.CreateInput{
		Description: "nightly",
		CronExpr:    "0 2 * * *",
	}); err != nil {
		t.Fatal(err)
	}
	tool := listScheduledJobsTool{deps: JobsDeps{Scheduler: svc}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "nightly") || !strings.Contains(res.Output, "cron") {
		t.Fatalf("output=%q", res.Output)
	}
}

func TestUpdateScheduledJobTool_PartialAndScheduleSwitch(t *testing.T) {
	svc := testSchedulerService(t)
	created, err := svc.Create(context.Background(), scheduler.CreateInput{
		Description:      "nightly",
		DefinitionOfDone: "done",
		WorkspacePath:    "/ws",
		CronExpr:         "0 2 * * *",
		CreatedBy:        scheduler.CreatedByAgent,
	})
	if err != nil {
		t.Fatal(err)
	}
	tool := updateScheduledJobTool{deps: JobsDeps{Scheduler: svc}}

	// Partial: description + pause
	res, err := tool.Execute(context.Background(), json.RawMessage(fmt.Sprintf(`{
		"scheduled_job_id":%q,
		"description":"nightly health",
		"enabled":false
	}`, created.ID)))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "nightly health" || got.Enabled || got.CronExpr != "0 2 * * *" {
		t.Fatalf("partial update=%+v", got)
	}

	// Switch cron → one-shot
	iso := "2031-06-01T10:00:00Z"
	body, _ := json.Marshal(map[string]any{
		"scheduled_job_id": created.ID,
		"run_at_iso":       iso,
		"enabled":          true,
	})
	res, err = tool.Execute(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	got, err = svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse(time.RFC3339, iso)
	if got.CronExpr != "" || got.RunAt == nil || *got.RunAt != want.UnixMilli() || !got.Enabled {
		t.Fatalf("schedule switch=%+v", got)
	}
}

func TestUpdateScheduledJobTool_RequiresID(t *testing.T) {
	svc := testSchedulerService(t)
	tool := updateScheduledJobTool{deps: JobsDeps{Scheduler: svc}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"description":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("expected failure without scheduled_job_id")
	}
}

func TestDeleteScheduledJobTool(t *testing.T) {
	svc := testSchedulerService(t)
	created, err := svc.Create(context.Background(), scheduler.CreateInput{
		Description: "to delete",
		CronExpr:    "0 3 * * *",
	})
	if err != nil {
		t.Fatal(err)
	}
	tool := deleteScheduledJobTool{deps: JobsDeps{Scheduler: svc}}
	res, err := tool.Execute(context.Background(), json.RawMessage(fmt.Sprintf(`{"scheduled_job_id":%q}`, created.ID)))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "Deleted") {
		t.Fatalf("res=%+v", res)
	}
	if _, err := svc.Get(context.Background(), created.ID); err == nil {
		t.Fatal("expected not found after delete")
	}
	// second delete fails
	res, err = tool.Execute(context.Background(), json.RawMessage(fmt.Sprintf(`{"scheduled_job_id":%q}`, created.ID)))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("expected delete of missing id to fail")
	}
}

func TestJobPromptIndexMentionsScheduledJobs(t *testing.T) {
	idx := JobPromptIndex("/tmp/ws", "")
	if !strings.Contains(idx, "create_scheduled_job") {
		t.Fatalf("expected create_scheduled_job guidance, got %q", idx)
	}
	if !strings.Contains(idx, "list_scheduled_jobs") {
		t.Fatalf("expected list_scheduled_jobs guidance, got %q", idx)
	}
	if !strings.Contains(idx, "update_scheduled_job") || !strings.Contains(idx, "delete_scheduled_job") {
		t.Fatalf("expected update/delete guidance, got %q", idx)
	}
	if !strings.Contains(idx, "Jobs vs scheduled jobs") {
		t.Fatalf("expected disambiguation section, got %q", idx)
	}
}

func TestCreateJobToolDescriptionDisambiguates(t *testing.T) {
	spec := createJobTool{}.Spec()
	if !strings.Contains(spec.Description, "create_scheduled_job") {
		t.Fatalf("create_job should point to create_scheduled_job: %q", spec.Description)
	}
	if !strings.Contains(spec.Description, "immediate") {
		t.Fatalf("create_job should say immediate: %q", spec.Description)
	}
}
