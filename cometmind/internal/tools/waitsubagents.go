package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/subagent"
)

// WaitSubagents blocks until selected child subagents finish.
type WaitSubagents struct {
	Sessions       *session.Service
	Orchestrator   *subagent.Orchestrator
	SubagentConfig SubagentToolConfig
}

func (w WaitSubagents) Spec() ToolSpec {
	return ToolSpec{
		Name: "wait_subagents",
		Description: "Wait for one or more parallel subagents to finish and return their summaries. " +
			"Omit child_session_ids to wait for all in-flight children of this session.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"child_session_ids":{
					"type":"array",
					"items":{"type":"string"},
					"description":"Child session IDs to wait for; empty waits for all active children"
				},
				"timeout_seconds":{"type":"integer","description":"Max wait time in seconds (default 1800)"}
			}
		}`),
	}
}

func (w WaitSubagents) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		ChildSessionIDs []string `json:"child_session_ids"`
		TimeoutSeconds  int      `json:"timeout_seconds"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	if w.Sessions == nil || w.Orchestrator == nil {
		return Result{OK: false, Output: "subagent wait is not configured"}, nil
	}

	parentID := ToolSessionFrom(ctx)
	if parentID == "" {
		return Result{OK: false, Output: "missing parent session context"}, nil
	}

	timeout := time.Duration(w.SubagentConfig.WaitTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results, err := w.Orchestrator.Wait(waitCtx, parentID, in.ChildSessionIDs)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	seen := make(map[string]struct{}, len(results))
	var b strings.Builder
	for _, res := range results {
		seen[res.ChildSessionID] = struct{}{}
		writeSubagentResult(&b, res.ChildSessionID, string(res.Kind), res.Status, res.Summary)
	}

	// Include already-finished children not tracked by the orchestrator.
	if len(in.ChildSessionIDs) > 0 {
		for _, id := range in.ChildSessionIDs {
			if _, ok := seen[id]; ok {
				continue
			}
			child, err := w.Sessions.GetSession(ctx, id)
			if err != nil {
				continue
			}
			if child.ParentSessionID != parentID {
				continue
			}
			if !isTerminalDelegation(child.DelegationStatus) {
				continue
			}
			writeSubagentResult(&b, child.ID, child.SubagentKind, child.DelegationStatus, child.OutputSummary)
		}
	}

	if b.Len() == 0 {
		return Result{OK: true, Output: "no subagents to wait for"}, nil
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}

func isTerminalDelegation(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func writeSubagentResult(b *strings.Builder, id, kind, status, summary string) {
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	fmt.Fprintf(b, "child_session_id: %s\nkind: %s\nstatus: %s\n\n%s", id, kind, status, summary)
}
