package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"go.uber.org/zap"
)

// streamAttempt tracks one model stream: what it forwarded, which tool calls
// it started, and how it ended.
type streamAttempt struct {
	started          time.Time
	events           int
	firstEvent       bool
	firstOutput      bool
	completeToolCall bool
	failureCategory  streamFailureCategory
	toolCalls        []cometsdk.ToolCallBlock
	toolIndex        map[string]int
}

// streamStep runs the model stream for one step, retrying recoverable
// failures. On success it returns the result and the tool calls the stream
// started. The bool reports that the turn ended instead; the error is nil
// when the user stopped it.
func (r *Runner) streamStep(ctx context.Context, s *turnState, p *stepRequest) (*llm.GenerateMessageResult, []cometsdk.ToolCallBlock, bool, error) {
	recoveryAttempt := 0
	overflowRecovered := false
	for {
		result, a, err := r.consumeStream(ctx, s, p, recoveryAttempt, overflowRecovered)
		if err == nil {
			logging.L().Info("agent.step.finish", zap.String("session", s.turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", s.turn.ModelID), zap.Int("step", s.steps+1), zap.String("finish_reason", string(result.FinishReason)), zap.Int("tool_calls", len(result.ToolCalls)), zap.Int("input_tokens", result.Usage.InputTokens), zap.Int("output_tokens", result.Usage.OutputTokens), zap.Int("recovery_attempt", recoveryAttempt))
			return result, a.toolCalls, false, nil
		}
		// A cancelled runner context is the explicit /stop path (or a client
		// disconnect). It is normal control flow, not a provider failure:
		// retain any visible partial output, close with done, and do not add an
		// error transcript row or SSE error card.
		if isUserStop(ctx, err) {
			r.persistPartial(ctx, s, result)
			r.logStepStopped(s, a)
			return nil, nil, true, nil
		}
		if !overflowRecovered && isContextOverflowError(err) && !a.completeToolCall && r.Compactor != nil && s.sess.ID != "" {
			overflowRecovered = true
			if r.recoverFromOverflow(ctx, s, p, err) {
				continue
			}
		}
		if ctx.Err() == nil && recoveryAttempt < maxStreamRecoveryAttempts && recoverableStreamFailure(err) && !a.completeToolCall {
			recoveryAttempt++
			if stop, waitErr := r.waitToRetryStream(ctx, s, a, result, err, recoveryAttempt); stop {
				return nil, nil, true, waitErr
			}
			continue
		}
		r.persistPartial(ctx, s, result)
		return nil, nil, true, r.reportStepFailure(s, a, recoveryAttempt, err)
	}
}

func (r *Runner) consumeStream(ctx context.Context, s *turnState, p *stepRequest, recoveryAttempt int, overflowRecovered bool) (*llm.GenerateMessageResult, *streamAttempt, error) {
	turn, req := s.turn, p.req
	a := &streamAttempt{started: time.Now(), toolIndex: map[string]int{}}
	logging.L().Info("llm.stream.start", zap.String("session", turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", turn.ModelID), zap.Int("step", s.steps+1), zap.Int("messages", len(req.Messages)), zap.Int("tools", len(req.Tools)), zap.Int("tool_output_bytes", p.toolOutputBytes), zap.Int("recovery_attempt", recoveryAttempt), zap.Bool("overflow_recovered", overflowRecovered), zap.Int("system_bytes", len(req.System)), zap.Int("max_tokens", req.MaxTokens))
	stream := llm.StreamMessage(ctx, r.Provider, req)
	logging.L().Info("llm.stream.opened", zap.String("session", turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", turn.ModelID), zap.Int("step", s.steps+1), zap.Int("recovery_attempt", recoveryAttempt), zap.Int64("duration_ms", time.Since(a.started).Milliseconds()))
	s.emitStatus(event.PhaseComposingResponse)

	for ev := range stream.Events() {
		a.events++
		if !a.firstEvent {
			a.firstEvent = true
			logging.L().Info("llm.stream.first_event", zap.String("session", turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", turn.ModelID), zap.Int("step", s.steps+1), zap.Int("recovery_attempt", recoveryAttempt), zap.String("event_type", fmt.Sprintf("%T", ev)), zap.Int64("duration_ms", time.Since(a.started).Milliseconds()))
		}
		a.forward(s, ev, p.finalizing)
	}
	result, err := stream.Result()
	a.failureCategory = classifyStreamFailure(err)
	logging.L().Info("llm.stream.events_closed", zap.String("session", turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", turn.ModelID), zap.Int("step", s.steps+1), zap.Int("events", a.events), zap.Bool("first_event", a.firstEvent), zap.Bool("first_output", a.firstOutput), zap.Bool("complete_tool_call", a.completeToolCall), zap.String("failure_category", string(a.failureCategory)), zap.Int("recovery_attempt", recoveryAttempt), zap.Int64("duration_ms", time.Since(a.started).Milliseconds()))
	return result, a, err
}

// forward relays one provider event to the turn stream. Tool calls are
// ignored on a finalizing step, which is sent without tools.
func (a *streamAttempt) forward(s *turnState, ev cometsdk.Event, finalizing bool) {
	switch e := ev.(type) {
	case cometsdk.TextDeltaEvent:
		a.firstOutput = true
		s.ch <- event.TextDelta(e.Text)
	case cometsdk.ReasoningStartEvent:
		s.ch <- event.ReasoningStart()
	case cometsdk.ReasoningContentEvent:
		a.firstOutput = true
		s.ch <- event.ReasoningDelta(e.Text)
	case cometsdk.ToolCallStartEvent:
		if finalizing {
			logging.L().Warn("agent.final_answer.unexpected_tool_call", zap.String("session", s.turn.ID), zap.String("tool_call_id", e.ID), zap.String("tool", e.Name))
			return
		}
		a.firstOutput = true
		a.startToolCall(e.ID, e.Name)
		s.ch <- event.ToolCall(e.ID, e.Name, nil)
	case cometsdk.ToolCallDoneEvent:
		if finalizing {
			logging.L().Warn("agent.final_answer.unexpected_tool_call", zap.String("session", s.turn.ID), zap.String("tool_call_id", e.ID), zap.String("tool", e.Name))
			return
		}
		a.firstOutput = true
		a.completeToolCall = true
		a.finishToolCall(e.ID, e.Name, json.RawMessage(e.Input))
		s.ch <- event.ToolCall(e.ID, e.Name, []byte(e.Input))
	case cometsdk.StepFinishEvent:
		s.ch <- event.StepFinish(e.Usage)
	}
}

func (a *streamAttempt) startToolCall(id, name string) {
	if idx, ok := a.toolIndex[id]; ok {
		a.toolCalls[idx].Name = name
		return
	}
	a.toolIndex[id] = len(a.toolCalls)
	a.toolCalls = append(a.toolCalls, cometsdk.ToolCallBlock{
		ID:    id,
		Name:  name,
		Input: json.RawMessage(`{}`),
	})
}

func (a *streamAttempt) finishToolCall(id, name string, input json.RawMessage) {
	if idx, ok := a.toolIndex[id]; ok {
		a.toolCalls[idx].Name = name
		a.toolCalls[idx].Input = input
		return
	}
	a.toolIndex[id] = len(a.toolCalls)
	a.toolCalls = append(a.toolCalls, cometsdk.ToolCallBlock{
		ID:    id,
		Name:  name,
		Input: input,
	})
}
