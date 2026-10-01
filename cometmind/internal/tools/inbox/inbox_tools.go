package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/session"
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

// AddLeaveTool adds leave_inbox_message when an inbox service is configured.
func AddLeaveTool(add func(Tool), deps InboxDeps) {
	if add == nil || deps.Inbox == nil {
		return
	}
	add(leaveInboxMessageTool{deps: deps})
}
