package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/jobs"
)

// JobsDeps provides job queue operations to agent tools.
type JobsDeps struct {
	Service              *jobs.Service
	SessionID            string
	SessionWorkspacePath string
	SourcePlatform       string
	SourceChannelID      string
}

type listJobsTool struct{ deps JobsDeps }

func (listJobsTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "list_jobs",
		Description: "List global jobs. By default returns ready todo jobs.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"status":{"type":"string","enum":["todo","ongoing","done"],"description":"Filter by status"},
				"ready_only":{"type":"boolean","description":"When true, only return todo jobs ready to claim"}
			}
		}`),
	}
}

func (t listJobsTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Service == nil {
		return Result{OK: false, Output: "jobs service unavailable"}, nil
	}
	var in struct {
		Status    string `json:"status"`
		ReadyOnly *bool  `json:"ready_only"`
	}
	_ = json.Unmarshal(input, &in)
	readyOnly := true
	if in.ReadyOnly != nil {
		readyOnly = *in.ReadyOnly
	}
	filter := jobs.ListFilter{Status: in.Status, ReadyOnly: readyOnly}
	items, err := t.deps.Service.List(ctx, filter)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if len(items) == 0 {
		return Result{OK: true, Output: "No jobs found."}, nil
	}
	var b strings.Builder
	for _, j := range items {
		fmt.Fprintf(&b, "- %s [status=%s] %s\n", j.ID, j.Status, j.Description)
		if j.DefinitionOfDone != "" {
			fmt.Fprintf(&b, "  DoD: %s\n", j.DefinitionOfDone)
		}
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}

type createJobTool struct {
	deps JobsDeps
}

func (createJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "create_job",
		Description: "Create a global todo job for later execution by any session.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"description":{"type":"string"},
				"definition_of_done":{"type":"string"},
				"workspace_path":{"type":"string"}
			},
			"required":["description"]
		}`),
	}
}

func (t createJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Service == nil {
		return Result{OK: false, Output: "jobs service unavailable"}, nil
	}
	var in struct {
		Description      string `json:"description"`
		DefinitionOfDone string `json:"definition_of_done"`
		WorkspacePath    string `json:"workspace_path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	platform := strings.TrimSpace(t.deps.SourcePlatform)
	if platform == "" {
		platform = jobs.PlatformDesktop
	}
	workspacePath := strings.TrimSpace(in.WorkspacePath)
	if workspacePath == "" {
		workspacePath = strings.TrimSpace(t.deps.SessionWorkspacePath)
	}
	job, err := t.deps.Service.Create(ctx, jobs.CreateInput{
		Description:      in.Description,
		DefinitionOfDone: in.DefinitionOfDone,
		WorkspacePath:    workspacePath,
		CreatedBy:        jobs.CreatedByAgent,
		SourceSessionID:  t.deps.SessionID,
		SourcePlatform:   platform,
		SourceChannelID:  t.deps.SourceChannelID,
	})
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Created job %s", job.ID)}, nil
}

type claimJobTool struct{ deps JobsDeps }

func (claimJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "claim_job",
		Description: "Claim a todo job for the current session.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"job_id":{"type":"string"}},"required":["job_id"]}`),
	}
}

func (t claimJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Service == nil || t.deps.SessionID == "" {
		return Result{OK: false, Output: "jobs service or session unavailable"}, nil
	}
	var in struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	job, err := t.deps.Service.Claim(ctx, strings.TrimSpace(in.JobID), t.deps.SessionID)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	_ = t.deps.Service.Heartbeat(ctx, job.ID, t.deps.SessionID)
	return Result{OK: true, Output: jobs.ExecutionPrompt(job)}, nil
}

type updateJobTool struct{ deps JobsDeps }

func (updateJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "update_job",
		Description: "Update a job. Todo jobs: description/DoD/workspace. Ongoing jobs: progress only. Call with progress after each meaningful milestone while working a claimed job.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"job_id":{"type":"string"},
				"description":{"type":"string"},
				"definition_of_done":{"type":"string"},
				"workspace_path":{"type":"string"},
				"progress":{"type":"string"}
			},
			"required":["job_id"]
		}`),
	}
}

func (t updateJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Service == nil {
		return Result{OK: false, Output: "jobs service unavailable"}, nil
	}
	var in struct {
		JobID            string `json:"job_id"`
		Description      string `json:"description"`
		DefinitionOfDone string `json:"definition_of_done"`
		WorkspacePath    string `json:"workspace_path"`
		Progress         string `json:"progress"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	jobID := strings.TrimSpace(in.JobID)
	if in.Progress != "" {
		job, err := t.deps.Service.UpdateProgress(ctx, jobID, in.Progress, t.deps.SessionID)
		if err != nil {
			return Result{OK: false, Output: err.Error()}, nil
		}
		return Result{OK: true, Output: fmt.Sprintf("Updated progress for job %s", job.ID)}, nil
	}
	current, err := t.deps.Service.Get(ctx, jobID)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	desc := current.Description
	if strings.TrimSpace(in.Description) != "" {
		desc = in.Description
	}
	dod := current.DefinitionOfDone
	if in.DefinitionOfDone != "" {
		dod = in.DefinitionOfDone
	}
	ws := current.WorkspacePath
	if in.WorkspacePath != "" {
		ws = in.WorkspacePath
	}
	job, err := t.deps.Service.UpdateTodo(ctx, jobID, jobs.UpdateTodoInput{
		Description:      desc,
		DefinitionOfDone: dod,
		WorkspacePath:    ws,
	}, t.deps.SessionID)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Updated job %s", job.ID)}, nil
}

type completeJobTool struct{ deps JobsDeps }

func (completeJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "complete_job",
		Description: "Mark an ongoing job assigned to this session as done.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"job_id":{"type":"string"},
				"progress":{"type":"string"}
			},
			"required":["job_id"]
		}`),
	}
}

func (t completeJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Service == nil || t.deps.SessionID == "" {
		return Result{OK: false, Output: "jobs service or session unavailable"}, nil
	}
	var in struct {
		JobID    string `json:"job_id"`
		Progress string `json:"progress"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	job, err := t.deps.Service.Complete(ctx, strings.TrimSpace(in.JobID), t.deps.SessionID, in.Progress)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Completed job %s", job.ID)}, nil
}

type releaseJobTool struct{ deps JobsDeps }

func (releaseJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "release_job",
		Description: "Release an ongoing job back to todo.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"job_id":{"type":"string"},
				"reason":{"type":"string"}
			},
			"required":["job_id"]
		}`),
	}
}

func (t releaseJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Service == nil || t.deps.SessionID == "" {
		return Result{OK: false, Output: "jobs service or session unavailable"}, nil
	}
	var in struct {
		JobID  string `json:"job_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	job, err := t.deps.Service.Release(ctx, strings.TrimSpace(in.JobID), t.deps.SessionID, in.Reason)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("Released job %s (status=%s)", job.ID, job.Status)}, nil
}

type proposeJobTool struct {
	deps JobsDeps
}

func (proposeJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "propose_job",
		Description: "Propose a global todo job for the user to confirm in chat. Does not create the job until the user picks a workspace in the UI.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"description":{"type":"string","description":"What needs to be done"},
				"definition_of_done":{"type":"string","description":"How to know the job is finished"}
			},
			"required":["description"]
		}`),
	}
}

func (t proposeJobTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Description      string `json:"description"`
		DefinitionOfDone string `json:"definition_of_done"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(in.Description) == "" {
		return Result{OK: false, Output: "description is required"}, nil
	}
	payload := map[string]string{
		"status":            "awaiting_workspace",
		"description":       strings.TrimSpace(in.Description),
		"definition_of_done": strings.TrimSpace(in.DefinitionOfDone),
		"default_workspace": strings.TrimSpace(t.deps.SessionWorkspacePath),
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return Result{}, err
	}
	return Result{OK: true, Output: string(out)}, nil
}

// JobPromptIndex returns system prompt guidance for job tools.
func JobPromptIndex(sessionWorkspace, platform string) string {
	ws := strings.TrimSpace(sessionWorkspace)
	wsLine := ""
	if ws != "" {
		wsLine = fmt.Sprintf(" Current session workspace: `%s`.", ws)
	}
	if platform == jobs.PlatformDiscord {
		return fmt.Sprintf(
			"\n\n## Global Jobs\nUse `list_jobs` when the user asks about pending work. When the user wants to create a job, call `propose_job` first — Discord will post a workspace selection message with a dropdown and Confirm/Cancel buttons. Tell the user to use that message (or `/create-job`) to pick a workspace. Use `create_job` directly only when the user already named the workspace in the same message.%s To execute a job, `claim_job` first or use `/jobs`. While working a claimed job, call `update_job` with `progress` after each meaningful milestone. Call `complete_job` when done or `release_job` to abandon.",
			wsLine,
		)
	}
	return fmt.Sprintf(
		"\n\n## Global Jobs\nUse `list_jobs` when the user asks about pending work. When the user wants to create a job, call `propose_job` first so they can confirm the workspace in chat — do not call `create_job` directly unless the user already specified the workspace in the same message. Use `create_job` only after workspace is confirmed or explicitly stated.%s The chat UI pre-fills the current session workspace. To execute a job, `claim_job` first. While working a claimed job, call `update_job` with `progress` after each meaningful milestone so another session can resume. Call `complete_job` when done or `release_job` to abandon.",
		wsLine,
	)
}

// RegisterJobTools adds job tools when deps are configured.
func RegisterJobTools(r *Registry, deps JobsDeps) {
	if deps.Service == nil {
		return
	}
	add := func(t Tool) {
		spec := t.Spec()
		r.byName[spec.Name] = t
		r.order = append(r.order, t)
	}
	add(listJobsTool{deps: deps})
	add(proposeJobTool{deps: deps})
	add(createJobTool{deps: deps})
	add(claimJobTool{deps: deps})
	add(updateJobTool{deps: deps})
	add(completeJobTool{deps: deps})
	add(releaseJobTool{deps: deps})
}
