package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/subagent"
)

// DelegateCodingTask hands coding work to an external ACP agent such as OpenCode.
type DelegateCodingTask struct {
	Workspace    Workspace
	Sessions     *session.Service
	ACP          acp.Config
	ACPMgr       *acp.SessionManager
	Orchestrator *subagent.Orchestrator
}

const delegationCancelledByUser = "delegation cancelled by user"

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
				"async":{"type":"boolean","description":"Return immediately after starting delegation without waiting for completion"}
			},
			"required":["task"]
		}`),
	}
}

func (d DelegateCodingTask) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Task          string `json:"task"`
		Context       string `json:"context"`
		VerifyCommand string `json:"verify_command"`
		Async         bool   `json:"async"`
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

	child, err := d.Sessions.NewChildSession(ctx, parent, task, "acp")
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	emit := ProgressFrom(ctx)
	if emit != nil {
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
		OnACPSessionID: func(acpSessionID string) {
			_ = d.Sessions.UpdateACPSessionID(ctx, child.ID, acpSessionID)
		},
	}

	if in.Async {
		runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		if d.Orchestrator != nil {
			if err := d.Orchestrator.Register(parentID, child.ID, subagent.KindACP, cancel); err != nil {
				cancel()
				return Result{OK: false, Output: err.Error()}, nil
			}
		}
		go d.finishDelegation(runCtx, parentID, child.ID, mgr, runOpts, emit)
		out := fmt.Sprintf("child_session_id: %s\nstatus: running\nagent: %s\n\nasync delegation started",
			child.ID, d.ACP.Command)
		return Result{OK: true, Output: out}, nil
	}

	result, runErr := mgr.Run(ctx, runOpts)
	res, _, _ := d.buildResult(ctx, parentID, child.ID, result, runErr, emit)
	return res, nil
}

func (d DelegateCodingTask) finishDelegation(
	ctx context.Context,
	parentID, childID string,
	mgr *acp.SessionManager,
	runOpts acp.RunOptions,
	emit func(event.Event),
) {
	result, runErr := mgr.Run(ctx, runOpts)
	_, status, summary := d.buildResult(ctx, parentID, childID, result, runErr, emit)
	if d.Orchestrator != nil {
		if ctx.Err() != nil {
			status = "cancelled"
			if summary == "" {
				summary = delegationCancelledByUser
			}
		}
		d.Orchestrator.Complete(childID, subagent.Result{
			Kind:    subagent.KindACP,
			Status:  status,
			Summary: summary,
		})
	}
}

func (d DelegateCodingTask) buildResult(
	ctx context.Context,
	parentID, childID string,
	result acp.TaskResult,
	runErr error,
	emit func(event.Event),
) (Result, string, string) {
	status, summary := normalizeDelegationOutcome(result, runErr)
	_ = d.Sessions.UpdateDelegationState(ctx, childID, status, summary, "")

	if emit != nil {
		emit(event.SubagentFinished(childID, status, summary))
	}

	out := fmt.Sprintf("child_session_id: %s\nstatus: %s\nagent: %s",
		childID, status, result.AgentName)
	if status == "cancelled" {
		out += "\ncancelled_by: user"
	}
	if summary != "" {
		out += "\n\n" + summary
	}
	ok := status == "completed"
	_ = parentID
	return Result{OK: ok, Output: out}, status, summary
}

func normalizeDelegationOutcome(result acp.TaskResult, runErr error) (status, summary string) {
	status = result.Status
	if status == "" {
		status = "failed"
	}
	summary = strings.TrimSpace(result.Summary)

	if runErr != nil && status == "completed" {
		status = "failed"
	}
	if errors.Is(runErr, context.Canceled) || status == "cancelled" {
		status = "cancelled"
		if summary == "" || summary == context.Canceled.Error() {
			summary = delegationCancelledByUser
		}
	}
	if summary == "" && runErr != nil && status != "cancelled" {
		summary = runErr.Error()
	}
	return status, summary
}
