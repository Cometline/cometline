package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/session"
)

// inboxSessionLookup loads a session for workspace provenance.
type inboxSessionLookup interface {
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
}

// InboxDeps wires leave_inbox_message and read-only job helpers.
type InboxDeps struct {
	Inbox     *inbox.Service
	Jobs      *jobs.Service
	Sessions  inboxSessionLookup
	Events    *event.Hub
	SessionID string
}

type leaveInboxMessageTool struct{ deps InboxDeps }

func (leaveInboxMessageTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "leave_inbox_message",
		Description: "Leave a short note for the user in their global inbox (bell icon). " +
			"Use for scheduled/autonomy results or confirmations worth a later glance — not routine chatter. " +
			"The user can reply later (internalized as memory in the background) or dismiss.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"title":{"type":"string","description":"Short headline shown in the inbox list"},
				"body":{"type":"string","description":"A few sentences of detail for the user"},
				"job_id":{"type":"string","description":"Optional related job id for deep link"}
			},
			"required":["title","body"]
		}`),
	}
}

func (t leaveInboxMessageTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Inbox == nil {
		return Result{OK: false, Output: "inbox service unavailable"}, nil
	}
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	workspaceID := ""
	if t.deps.Sessions != nil && strings.TrimSpace(t.deps.SessionID) != "" {
		if sess, err := t.deps.Sessions.GetSession(ctx, t.deps.SessionID); err == nil {
			workspaceID = sess.WorkspaceID
		}
	}
	msg, err := t.deps.Inbox.Create(ctx, inbox.CreateInput{
		Title:       in.Title,
		Body:        in.Body,
		WorkspaceID: workspaceID,
		JobID:       in.JobID,
		SessionID:   t.deps.SessionID,
	})
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if t.deps.Events != nil {
		openCount, _ := t.deps.Inbox.CountOpen(ctx)
		t.deps.Events.Publish(event.InboxMessageCreated(msg.ID, openCount))
	}
	return Result{OK: true, Output: fmt.Sprintf("Left inbox message %s", msg.ID)}, nil
}

type getJobTool struct{ deps InboxDeps }

func (getJobTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "get_job",
		Description: "Read one job by id (description, status, progress, failures).",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"job_id":{"type":"string"}
			},
			"required":["job_id"]
		}`),
	}
}

func (t getJobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if t.deps.Jobs == nil {
		return Result{OK: false, Output: "jobs service unavailable"}, nil
	}
	var in struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	jobID := strings.TrimSpace(in.JobID)
	if jobID == "" {
		return Result{OK: false, Output: "job_id is required"}, nil
	}
	job, err := t.deps.Jobs.Get(ctx, jobID)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "id: %s\nstatus: %s\ndescription: %s\n", job.ID, job.Status, job.Description)
	if job.DefinitionOfDone != "" {
		fmt.Fprintf(&b, "definition_of_done: %s\n", job.DefinitionOfDone)
	}
	if job.Progress != "" {
		fmt.Fprintf(&b, "progress: %s\n", job.Progress)
	}
	if job.LastFailureReason != "" {
		fmt.Fprintf(&b, "last_failure_reason: %s\n", job.LastFailureReason)
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}

// RegisterInboxLeaveTool adds leave_inbox_message to a parent/autonomy registry.
func RegisterInboxLeaveTool(r *Registry, deps InboxDeps) {
	if r == nil || deps.Inbox == nil {
		return
	}
	r.byName["leave_inbox_message"] = leaveInboxMessageTool{deps: deps}
	r.order = append(r.order, leaveInboxMessageTool{deps: deps})
}

// NewInboxProcessRegistry builds the limited tool set for inbox reply internalization.
func NewInboxProcessRegistry(opt RegistryOptions) *Registry {
	ws := Workspace{Root: ""}
	r := &Registry{workspace: ws, byName: make(map[string]Tool)}
	add := func(t Tool) {
		spec := t.Spec()
		r.byName[spec.Name] = t
		r.order = append(r.order, t)
	}
	if opt.Memory != nil {
		add(ListMemories{Memory: opt.Memory})
		add(SearchMemories{Memory: opt.Memory})
		add(CreateMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
		add(UpdateMemory{Memory: opt.Memory, Events: opt.MemoryEvents})
	}
	deps := InboxDeps{Jobs: opt.Jobs}
	if opt.Jobs != nil {
		add(getJobTool{deps: deps})
	}
	return r
}
