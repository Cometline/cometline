package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/scheduler"
)

type listScheduledJobsTool struct{ deps JobsDeps }

func (listScheduledJobsTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "list_scheduled_jobs",
		Description: "List deferred/recurring scheduled jobs (cron or one-shot). Use this to inspect existing schedules; use list_jobs for the immediate work queue.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (t listScheduledJobsTool) Execute(ctx context.Context, _ json.RawMessage) (Result, error) {
	if t.deps.Scheduler == nil {
		return Result{OK: false, Output: "scheduler service unavailable"}, nil
	}
	items, err := t.deps.Scheduler.List(ctx)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if len(items) == 0 {
		return Result{OK: true, Output: "No scheduled jobs found."}, nil
	}
	var b strings.Builder
	for _, j := range items {
		fmt.Fprintf(&b, "- %s [enabled=%v] %s\n", j.ID, j.Enabled, j.Description)
		if j.DefinitionOfDone != "" {
			fmt.Fprintf(&b, "  DoD: %s\n", j.DefinitionOfDone)
		}
		if j.CronExpr != "" {
			fmt.Fprintf(&b, "  schedule: cron %q\n", j.CronExpr)
		} else if j.RunAt != nil {
			fmt.Fprintf(&b, "  schedule: one-shot run_at=%d (%s)\n", *j.RunAt, formatMillis(*j.RunAt))
		}
		fmt.Fprintf(&b, "  next_run_at=%d (%s)\n", j.NextRunAt, formatMillis(j.NextRunAt))
		if j.WorkspacePath != "" {
			fmt.Fprintf(&b, "  workspace: %s\n", j.WorkspacePath)
		}
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}

type createScheduledJobTool struct{ deps JobsDeps }

func (createScheduledJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "create_scheduled_job",
		Description: "Create a scheduled job (one-shot or recurring). Use for delayed or cron work — not create_job. " +
			"Schedule only via cron_expr or run_at/run_at_iso (mutually exclusive). " +
			"description is the work to do when it fires; definition_of_done is success criteria, not the schedule. " +
			"When due, CometMind materializes a normal queue job automatically. Prefer run_at_iso with timezone offset (e.g. 2026-07-10T15:00:00+08:00).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"description":{"type":"string","description":"Work to perform when the schedule fires (not the schedule itself)"},
				"definition_of_done":{"type":"string","description":"Success criteria for the work, not time/cron"},
				"workspace_path":{"type":"string"},
				"cron_expr":{"type":"string","description":"Standard 5-field cron (minute hour day month weekday). Mutually exclusive with run_at/run_at_iso."},
				"run_at":{"type":"integer","description":"One-shot Unix time in milliseconds. Mutually exclusive with cron_expr and run_at_iso."},
				"run_at_iso":{"type":"string","description":"One-shot RFC3339/RFC3339Nano timestamp (prefer with offset). Mutually exclusive with cron_expr and run_at."}
			},
			"required":["description"]
		}`),
	}
}

func (t createScheduledJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Scheduler == nil {
		return Result{OK: false, Output: "scheduler service unavailable"}, nil
	}
	var in struct {
		Description      string `json:"description"`
		DefinitionOfDone string `json:"definition_of_done"`
		WorkspacePath    string `json:"workspace_path"`
		CronExpr         string `json:"cron_expr"`
		RunAt            int64  `json:"run_at"`
		RunAtISO         string `json:"run_at_iso"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	cronExpr := strings.TrimSpace(in.CronExpr)
	runAt, err := parseScheduledRunAt(in.RunAt, in.RunAtISO)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if cronExpr != "" && runAt > 0 {
		return Result{OK: false, Output: "provide either cron_expr or run_at/run_at_iso, not both"}, nil
	}
	if cronExpr == "" && runAt <= 0 {
		return Result{OK: false, Output: "either cron_expr or run_at/run_at_iso is required"}, nil
	}

	platform := strings.TrimSpace(t.deps.SourcePlatform)
	if platform == "" {
		platform = jobs.PlatformDesktop
	}
	workspacePath := strings.TrimSpace(in.WorkspacePath)
	if workspacePath == "" {
		workspacePath = strings.TrimSpace(t.deps.SessionWorkspacePath)
	}

	item, err := t.deps.Scheduler.Create(ctx, scheduler.CreateInput{
		Description:      in.Description,
		DefinitionOfDone: in.DefinitionOfDone,
		WorkspacePath:    workspacePath,
		CreatedBy:        scheduler.CreatedByAgent,
		SourceSessionID:  t.deps.SessionID,
		SourcePlatform:   platform,
		SourceChannelID:  t.deps.SourceChannelID,
		CronExpr:         cronExpr,
		RunAt:            runAt,
	})
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	return Result{OK: true, Output: formatScheduledJobResult("Created", item)}, nil
}

type updateScheduledJobTool struct{ deps JobsDeps }

func (updateScheduledJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "update_scheduled_job",
		Description: "Update an existing scheduled job (description, DoD, workspace, schedule, or enabled). " +
			"Omit fields you do not want to change. To change the schedule, set either cron_expr or run_at/run_at_iso (not both). " +
			"Set enabled=false to pause without deleting. Use list_scheduled_jobs to find the scheduled_job_id.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"scheduled_job_id":{"type":"string","description":"ID of the scheduled job to update"},
				"description":{"type":"string","description":"Work to perform when the schedule fires"},
				"definition_of_done":{"type":"string","description":"Success criteria for the work, not time/cron"},
				"workspace_path":{"type":"string"},
				"cron_expr":{"type":"string","description":"Standard 5-field cron. Mutually exclusive with run_at/run_at_iso."},
				"run_at":{"type":"integer","description":"One-shot Unix milliseconds. Mutually exclusive with cron_expr and run_at_iso."},
				"run_at_iso":{"type":"string","description":"One-shot RFC3339 timestamp. Mutually exclusive with cron_expr and run_at."},
				"enabled":{"type":"boolean","description":"false pauses the schedule; true re-enables it"}
			},
			"required":["scheduled_job_id"]
		}`),
	}
}

func (t updateScheduledJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Scheduler == nil {
		return Result{OK: false, Output: "scheduler service unavailable"}, nil
	}
	var in struct {
		ScheduledJobID   string `json:"scheduled_job_id"`
		Description      string `json:"description"`
		DefinitionOfDone string `json:"definition_of_done"`
		WorkspacePath    string `json:"workspace_path"`
		CronExpr         string `json:"cron_expr"`
		RunAt            int64  `json:"run_at"`
		RunAtISO         string `json:"run_at_iso"`
		Enabled          *bool  `json:"enabled"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	id := strings.TrimSpace(in.ScheduledJobID)
	if id == "" {
		return Result{OK: false, Output: "scheduled_job_id is required"}, nil
	}
	current, err := t.deps.Scheduler.Get(ctx, id)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	description := firstNonEmpty(in.Description, current.Description)
	dod := firstNonEmpty(in.DefinitionOfDone, current.DefinitionOfDone)
	workspace := firstNonEmpty(in.WorkspacePath, current.WorkspacePath)
	enabled := current.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	cronExpr := current.CronExpr
	var runAt int64
	if current.CronExpr == "" && current.RunAt != nil {
		runAt = *current.RunAt
	}
	newCron := strings.TrimSpace(in.CronExpr)
	newRunAt, err := parseScheduledRunAt(in.RunAt, in.RunAtISO)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if newCron != "" && newRunAt > 0 {
		return Result{OK: false, Output: "provide either cron_expr or run_at/run_at_iso, not both"}, nil
	}
	if newCron != "" || newRunAt > 0 {
		cronExpr = newCron
		runAt = newRunAt
	}

	item, err := t.deps.Scheduler.Update(ctx, id, scheduler.UpdateInput{
		Description:      description,
		DefinitionOfDone: dod,
		WorkspacePath:    workspace,
		CronExpr:         cronExpr,
		RunAt:            runAt,
		Enabled:          enabled,
	})
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: formatScheduledJobResult("Updated", item)}, nil
}

type deleteScheduledJobTool struct{ deps JobsDeps }

func (deleteScheduledJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "delete_scheduled_job",
		Description: "Permanently delete a scheduled job by id. Does not delete already-materialized queue jobs. Use list_scheduled_jobs to find the id; use update_scheduled_job with enabled=false to pause instead of delete.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"scheduled_job_id":{"type":"string","description":"ID of the scheduled job to delete"}
			},
			"required":["scheduled_job_id"]
		}`),
	}
}

func (t deleteScheduledJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Scheduler == nil {
		return Result{OK: false, Output: "scheduler service unavailable"}, nil
	}
	var in struct {
		ScheduledJobID string `json:"scheduled_job_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	id := strings.TrimSpace(in.ScheduledJobID)
	if id == "" {
		return Result{OK: false, Output: "scheduled_job_id is required"}, nil
	}
	if err := t.deps.Scheduler.Delete(ctx, id); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Deleted scheduled job %s", id)}, nil
}

func formatScheduledJobResult(verb string, item scheduler.ScheduledJob) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s scheduled job %s\n", verb, item.ID)
	fmt.Fprintf(&b, "description: %s\n", item.Description)
	if item.DefinitionOfDone != "" {
		fmt.Fprintf(&b, "definition_of_done: %s\n", item.DefinitionOfDone)
	}
	if item.CronExpr != "" {
		fmt.Fprintf(&b, "schedule: cron %q\n", item.CronExpr)
	} else if item.RunAt != nil {
		fmt.Fprintf(&b, "schedule: one-shot run_at=%d (%s)\n", *item.RunAt, formatMillis(*item.RunAt))
	}
	fmt.Fprintf(&b, "next_run_at: %d (%s)\n", item.NextRunAt, formatMillis(item.NextRunAt))
	if item.WorkspacePath != "" {
		fmt.Fprintf(&b, "workspace: %s\n", item.WorkspacePath)
	}
	fmt.Fprintf(&b, "enabled: %v", item.Enabled)
	return b.String()
}

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// parseScheduledRunAt resolves a one-shot time from unix milliseconds and/or ISO8601.
// Returns 0 when neither is set. Rejects both set or invalid ISO.
func parseScheduledRunAt(runAtMs int64, runAtISO string) (int64, error) {
	runAtISO = strings.TrimSpace(runAtISO)
	if runAtMs > 0 && runAtISO != "" {
		return 0, fmt.Errorf("provide either run_at or run_at_iso, not both")
	}
	if runAtISO == "" {
		return runAtMs, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, runAtISO); err == nil {
		return t.UnixMilli(), nil
	}
	if t, err := time.Parse(time.RFC3339, runAtISO); err == nil {
		return t.UnixMilli(), nil
	}
	return 0, fmt.Errorf("invalid run_at_iso %q: use RFC3339 with offset (e.g. 2026-07-10T15:00:00+08:00)", runAtISO)
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "n/a"
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
