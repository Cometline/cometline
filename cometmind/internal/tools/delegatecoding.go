package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

// DelegateCodingTask hands coding work to an external ACP agent such as OpenCode.
type DelegateCodingTask struct {
	Workspace Workspace
	Sessions  *session.Service
	ACP       acp.Config
	ACPMgr    *acp.SessionManager
}

func (DelegateCodingTask) Spec() ToolSpec {
	return ToolSpec{
		Name: "delegate_coding_task",
		Description: "Delegate a coding task to an external OpenCode agent over ACP. " +
			"Use for multi-file edits, refactors, and test runs. Returns a summary with verify output.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"task":{"type":"string","description":"Coding task for the subagent"},
				"context":{"type":"string","description":"Optional extra context, file notes, or constraints"},
				"verify_command":{"type":"string","description":"Optional shell command to run after coding, e.g. cd cometmind && go test ./..."},
				"child_session_id":{"type":"string","description":"Resume an existing child session instead of creating a new one"},
				"async":{"type":"boolean","description":"Return immediately after starting delegation without waiting for completion"}
			},
			"required":["task"]
		}`),
	}
}

func (d DelegateCodingTask) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Task           string `json:"task"`
		Context        string `json:"context"`
		VerifyCommand  string `json:"verify_command"`
		ChildSessionID string `json:"child_session_id"`
		Async          bool   `json:"async"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	task := strings.TrimSpace(in.Task)
	if task == "" {
		return Result{OK: false, Output: "task is required"}, nil
	}
	if d.Sessions == nil {
		return Result{OK: false, Output: "delegation is not configured"}, nil
	}

	parentID := ToolSessionFrom(ctx)
	if parentID == "" {
		return Result{OK: false, Output: "missing parent session context"}, nil
	}

	parent, err := d.Sessions.GetSession(ctx, parentID)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	child, resumed, err := d.resolveChild(ctx, parent, task, strings.TrimSpace(in.ChildSessionID))
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	emit := ProgressFrom(ctx)
	if !resumed && emit != nil {
		emit(event.SubagentStarted(child.ID, task, d.ACP.Command))
	}

	_ = d.Sessions.UpdateDelegationState(ctx, child.ID, "running", "", "")

	mgr := d.ACPMgr
	if mgr == nil {
		mgr = acp.NewSessionManager(d.ACP)
	}

	runOpts := acp.RunOptions{
		ChildSessionID: child.ID,
		WorkspaceRoot:  d.Workspace.Root,
		Task:           task,
		Context:        in.Context,
		VerifyCommand:  in.VerifyCommand,
		Interactive:    d.ACP.Interactive,
		OnProgress: func(u acp.ProgressUpdate) {
			if emit == nil {
				return
			}
			text := u.Content
			if text == "" && u.Title != "" {
				text = u.Title
				if u.Status != "" {
					text += " (" + u.Status + ")"
				}
			}
			emit(event.SubagentProgress(child.ID, u.Kind, text))
		},
		OnAwaiting: func(info acp.AwaitingInfo) {
			status := "awaiting_user"
			if info.Kind == "permission" {
				status = "awaiting_permission"
			}
			_ = d.Sessions.UpdateDelegationState(ctx, child.ID, status, "", info.Question)
			if emit == nil {
				return
			}
			opts := make([]event.PermissionOptionWire, 0, len(info.Options))
			for _, opt := range info.Options {
				opts = append(opts, event.PermissionOptionWire{
					ID:   opt.ID,
					Kind: opt.Kind,
					Name: opt.Name,
				})
			}
			emit(event.SubagentAwaitingInput(child.ID, info.Kind, info.Question, opts))
		},
		OnACPSessionID: func(acpSessionID string) {
			_ = d.Sessions.UpdateACPSessionID(ctx, child.ID, acpSessionID)
		},
	}

	if in.Async {
		go d.finishDelegation(context.WithoutCancel(ctx), child.ID, mgr, runOpts, emit)
		out := fmt.Sprintf("child_session_id: %s\nstatus: running\nagent: %s\n\nasync delegation started",
			child.ID, d.ACP.Command)
		return Result{OK: true, Output: out}, nil
	}

	result, runErr := mgr.Run(ctx, runOpts)
	return d.buildResult(ctx, child.ID, result, runErr, emit)
}

func (d DelegateCodingTask) finishDelegation(
	ctx context.Context,
	childID string,
	mgr *acp.SessionManager,
	runOpts acp.RunOptions,
	emit func(event.Event),
) {
	result, runErr := mgr.Run(ctx, runOpts)
	_, _ = d.buildResult(ctx, childID, result, runErr, emit)
}

func (d DelegateCodingTask) buildResult(
	ctx context.Context,
	childID string,
	result acp.TaskResult,
	runErr error,
	emit func(event.Event),
) (Result, error) {
	status := result.Status
	if status == "" {
		status = "failed"
	}
	if runErr != nil && status == "completed" {
		status = "failed"
	}

	summary := result.Summary
	if summary == "" && runErr != nil {
		summary = runErr.Error()
	}
	_ = d.Sessions.UpdateDelegationState(ctx, childID, status, summary, "")

	if emit != nil {
		emit(event.SubagentFinished(childID, status, summary))
	}

	out := fmt.Sprintf("child_session_id: %s\nstatus: %s\nagent: %s\n\n%s",
		childID, status, result.AgentName, summary)
	ok := status == "completed"
	if runErr != nil {
		ok = false
	}
	return Result{OK: ok, Output: out}, nil
}

func (d DelegateCodingTask) resolveChild(
	ctx context.Context,
	parent session.Session,
	task string,
	childSessionID string,
) (session.Session, bool, error) {
	if childSessionID == "" {
		child, err := d.Sessions.NewChildSession(ctx, parent, task)
		return child, false, err
	}

	child, err := d.Sessions.GetSession(ctx, childSessionID)
	if err != nil {
		return session.Session{}, false, err
	}
	if child.ParentSessionID != parent.ID {
		return session.Session{}, false, fmt.Errorf("child session does not belong to parent")
	}
	switch child.DelegationStatus {
	case "running", "awaiting_user", "awaiting_permission":
	default:
		return session.Session{}, false, fmt.Errorf("child session %s cannot be resumed (status=%s)", child.ID, child.DelegationStatus)
	}
	return child, true, nil
}
