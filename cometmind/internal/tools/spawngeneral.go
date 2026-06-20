package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/subagent"
)

// SpawnGeneralAgent runs a restricted CometMind agent loop in a background child session.
type SpawnGeneralAgent struct {
	Workspace      Workspace
	Sessions       *session.Service
	Orchestrator   *subagent.Orchestrator
	RunnerFactory  ChildRunnerFactory
	SubagentConfig SubagentToolConfig
}

func (s SpawnGeneralAgent) Spec() ToolSpec {
	return ToolSpec{
		Name: "spawn_general_agent",
		Description: "Spawn a read-only research subagent that runs in parallel. " +
			"Returns immediately with a child_session_id; use wait_subagents to collect results.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"task":{"type":"string","description":"Task for the subagent"},
				"context":{"type":"string","description":"Optional extra constraints"},
				"model_id":{"type":"string","description":"Optional model override"},
				"provider_id":{"type":"string","description":"Optional provider override"}
			},
			"required":["task"]
		}`),
	}
}

func (s SpawnGeneralAgent) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Task       string `json:"task"`
		Context    string `json:"context"`
		ModelID    string `json:"model_id"`
		ProviderID string `json:"provider_id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	task := strings.TrimSpace(in.Task)
	if task == "" {
		return Result{OK: false, Output: "task is required"}, nil
	}
	if s.Sessions == nil || s.Orchestrator == nil || s.RunnerFactory == nil {
		return Result{OK: false, Output: "subagent spawning is not configured"}, nil
	}

	parentID := ToolSessionFrom(ctx)
	if parentID == "" {
		return Result{OK: false, Output: "missing parent session context"}, nil
	}

	parent, err := s.Sessions.GetSession(ctx, parentID)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	child, err := s.Sessions.NewChildSession(ctx, parent, task, "general")
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

		if in.ModelID != "" || in.ProviderID != "" {
		modelID := child.ModelID
		providerID := child.ProviderID
		if in.ModelID != "" {
			modelID = strings.TrimSpace(in.ModelID)
		}
		if in.ProviderID != "" {
			providerID = strings.TrimSpace(in.ProviderID)
		}
		updated, err := s.Sessions.UpdateSessionModel(ctx, child.ID, modelID, providerID)
		if err != nil {
			return Result{OK: false, Output: err.Error()}, nil
		}
		child = updated
	}

	userText := task
	if ctxText := strings.TrimSpace(in.Context); ctxText != "" {
		userText = task + "\n\n" + ctxText
	}
	if _, err := s.Sessions.AppendUserMessage(ctx, child.ID, userText); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	emit := ProgressFrom(ctx)
	if emit != nil {
		emit(event.SubagentStarted(child.ID, task, "cometmind"))
	}
	_ = s.Sessions.UpdateDelegationState(ctx, child.ID, "running", "", "")

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	if err := s.Orchestrator.Register(parentID, child.ID, subagent.KindGeneral, cancel); err != nil {
		cancel()
		return Result{OK: false, Output: err.Error()}, nil
	}

	maxSteps := s.SubagentConfig.GeneralMaxSteps
	if maxSteps <= 0 {
		maxSteps = 1
	}

	go s.runGeneralSubagent(runCtx, parentID, child, emit, maxSteps)

	out := fmt.Sprintf("child_session_id: %s\nstatus: running\nkind: general\nmax_steps: %d",
		child.ID, maxSteps)
	return Result{OK: true, Output: out}, nil
}

func (s SpawnGeneralAgent) runGeneralSubagent(
	runCtx context.Context,
	parentID string,
	child session.Session,
	emit func(event.Event),
	maxSteps int,
) {
	status := "completed"
	summary := ""
	var runErr error

	defer func() {
		if status == "completed" && summary == "" && runErr != nil {
			status = "failed"
			summary = runErr.Error()
		}
		if status == "completed" && summary == "" {
			summary = "subagent finished without assistant text"
		}
		_ = s.Sessions.UpdateDelegationState(context.Background(), child.ID, status, summary, "")
		if status == "completed" || status == "failed" || status == "cancelled" {
			_ = s.Sessions.CompactChildSession(context.Background(), child.ID)
		}
		if emit != nil {
			emit(event.SubagentFinished(child.ID, status, summary))
		}
		s.Orchestrator.Complete(child.ID, subagent.Result{
			Kind:    subagent.KindGeneral,
			Status:  status,
			Summary: summary,
		})
	}()

	runner, err := s.RunnerFactory(child, s.Workspace.Root, maxSteps)
	if err != nil {
		status = "failed"
		runErr = err
		return
	}

	evCh := make(chan event.Event, 64)
	go func() {
		runErr = runner.Run(runCtx, session.AgentTurnFromSession(child), evCh)
		close(evCh)
	}()

	for ev := range evCh {
		if emit == nil {
			continue
		}
		switch ev.Kind {
		case event.KindTurnStatus:
			emit(event.SubagentProgress(child.ID, "status", string(ev.Phase)))
		case event.KindToolCall:
			emit(event.SubagentProgress(child.ID, "tool", ev.Tool))
		case event.KindError:
			emit(event.SubagentProgress(child.ID, "error", ev.Message))
		}
	}

	if runCtx.Err() != nil {
		status = "cancelled"
		summary = runCtx.Err().Error()
		return
	}

	status, summary = s.finalizeGeneralSubagent(context.Background(), child.ID, runErr)
}

func (s SpawnGeneralAgent) finalizeGeneralSubagent(
	ctx context.Context,
	childID string,
	runErr error,
) (status, summary string) {
	status = "completed"
	if runErr != nil {
		status = "failed"
	}

	var parts []string
	if text, err := s.Sessions.LastAssistantText(ctx, childID); err == nil {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 && runErr != nil {
		if toolWork := s.summarizeRecentToolWork(ctx, childID); toolWork != "" {
			parts = append(parts, toolWork)
		}
	}

	if runErr != nil {
		errMsg := runErr.Error()
		stepLimit := strings.Contains(errMsg, "max steps exceeded")
		var footer string
		if stepLimit {
			footer = "Step limit reached — research incomplete."
		} else {
			footer = errMsg
		}
		if len(parts) > 0 {
			parts = append(parts, "("+footer+")")
		} else {
			parts = append(parts, footer)
		}
	}

	summary = strings.TrimSpace(strings.Join(parts, "\n\n"))
	return status, summary
}

func (s SpawnGeneralAgent) summarizeRecentToolWork(ctx context.Context, childID string) string {
	calls, err := s.Sessions.ListToolCallsForSession(ctx, childID)
	if err != nil || len(calls) == 0 {
		return ""
	}
	start := len(calls) - 3
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	b.WriteString("Partial progress from tool calls:")
	for _, tc := range calls[start:] {
		out := strings.TrimSpace(tc.Result)
		if out == "" {
			fmt.Fprintf(&b, "\n- %s", tc.ToolName)
			continue
		}
		if len(out) > 400 {
			out = out[:400] + "…"
		}
		fmt.Fprintf(&b, "\n- %s: %s", tc.ToolName, out)
	}
	return strings.TrimSpace(b.String())
}
