package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

// Runner executes the persisted agent loop for one user turn (which may span many tool steps).
type Runner struct {
	Provider cometsdk.Provider
	Sessions *session.Service
	Registry *tools.Registry

	WorkspaceRoot string
	MaxSteps      int
	MaxTokens     int
	SystemPrompt  string
}

// Run streams CometMind-native events on ch until the turn completes or ctx is cancelled.
// The caller must receive until the channel closes.
func (r *Runner) Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
	defer func() {
		ch <- event.Event{Kind: event.KindDone}
	}()

	if r.MaxSteps <= 0 {
		r.MaxSteps = 50
	}
	if r.MaxTokens <= 0 {
		r.MaxTokens = 8192
	}

	steps := 0
	for steps < r.MaxSteps {
		msgs, err := r.Sessions.BuildSDKMessages(ctx, turn.ID)
		if err != nil {
			ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: err.Error(), Code: "history"}}
			return err
		}

		req := BuildRequest(turn.ModelID, r.SystemPrompt, msgs, r.Registry.CometSDK(), r.MaxTokens)
		stream := llm.StreamMessage(ctx, r.Provider, req)

		for ev := range stream.Events() {
			switch e := ev.(type) {
			case cometsdk.TextDeltaEvent:
				ch <- event.Event{Kind: event.KindTextDelta, TextDelta: &event.TextDelta{Delta: e.Text}}
			case cometsdk.ReasoningStartEvent:
				ch <- event.Event{Kind: event.KindReasoningStart}
			case cometsdk.ReasoningContentEvent:
				ch <- event.Event{Kind: event.KindReasoningDelta, ReasoningDelta: &event.ReasoningDelta{Text: e.Text}}
			case cometsdk.ToolCallDoneEvent:
				in := []byte(e.Input)
				ch <- event.Event{Kind: event.KindToolCall, ToolCall: &event.ToolCall{ID: e.ID, Tool: e.Name, Input: in}}
			case cometsdk.StepFinishEvent:
				u := e.Usage
				ch <- event.Event{Kind: event.KindStepFinish, Step: &event.StepFinish{Usage: u}}
			}
		}

		result, err := stream.Result()
		if err != nil {
			ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: err.Error(), Code: "llm"}}
			return err
		}

		if err := r.Sessions.SaveTokenUsage(ctx, turn.ID, result.Usage); err != nil {
			ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: err.Error(), Code: "db"}}
			return err
		}

		text := assistantPlainText(result.Message)
		reasoningBlocks := result.Message.ReasoningContent
		_, persistedToolIDs, err := r.Sessions.AppendAssistantStep(ctx, turn.ID, text, reasoningBlocks, result.ToolCalls)
		if err != nil {
			ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: err.Error(), Code: "db"}}
			return err
		}

		switch result.FinishReason {
		case "stop", "max_tokens":
			return nil
		}
		if len(result.ToolCalls) == 0 {
			return nil
		}

		for _, tc := range result.ToolCalls {
			persistedID := persistedToolIDs[tc.ID]
			if persistedID == "" {
				ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: "missing persisted tool call id", Code: "db"}}
				return fmt.Errorf("missing persisted tool call id for %s", tc.ID)
			}
			start := time.Now()
			res, execErr := r.Registry.Execute(ctx, r.WorkspaceRoot, tc.Name, tc.Input)
			dur := time.Since(start).Milliseconds()

			out := res.Output
			isErr := !res.OK
			if execErr != nil {
				isErr = true
				out = fmt.Sprintf("%s\n(execute error: %v)", out, execErr)
			}

			exit := int64PtrFromIntPtr(res.ExitCode)
			if err := r.Sessions.UpdateToolCallResult(ctx, persistedID, out, dur, exit); err != nil {
				ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: err.Error(), Code: "db"}}
				return err
			}
			if _, err := r.Sessions.AppendToolResultMessage(ctx, turn.ID, persistedID, out, isErr); err != nil {
				ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: err.Error(), Code: "db"}}
				return err
			}

			toolErr := ""
			if isErr {
				toolErr = out
			}
			ch <- event.Event{Kind: event.KindToolResult, ToolOut: &event.ToolResult{ID: tc.ID, Tool: tc.Name, Output: out, Err: toolErr}}
		}

		steps++
	}

	ch <- event.Event{Kind: event.KindError, Err: &event.Error{Message: "max steps exceeded", Code: "max_steps"}}
	return fmt.Errorf("max steps exceeded")
}

func int64PtrFromIntPtr(v *int) *int64 {
	if v == nil {
		return nil
	}
	x := int64(*v)
	return &x
}

func assistantPlainText(m cometsdk.Message) string {
	var b strings.Builder
	for _, bl := range m.Content {
		if tb, ok := bl.(cometsdk.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return b.String()
}
