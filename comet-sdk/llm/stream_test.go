package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── StreamMessage Tests ──────────────────────────────────────────────────────

func TestStreamMessage_TextOnly(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Hello"},
			cometsdk.TextDeltaEvent{Text: " world"},
			cometsdk.StepFinishEvent{
				FinishReason: "stop",
				Usage:        cometsdk.TokenUsage{InputTokens: 8, OutputTokens: 4},
			},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())

	var textDeltas int
	var stepFinishes int
	for ev := range stream.Events() {
		switch ev.(type) {
		case cometsdk.TextDeltaEvent:
			textDeltas++
		case cometsdk.StepFinishEvent:
			stepFinishes++
		}
	}

	assert.Equal(t, 2, textDeltas)
	assert.Equal(t, 1, stepFinishes)

	result, err := stream.Result()
	require.NoError(t, err)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Equal(t, cometsdk.RoleAssistant, result.Message.Role)
	assert.Empty(t, result.ToolCalls)
	require.Len(t, result.Message.Content, 1)
	tb, ok := result.Message.Content[0].(cometsdk.TextBlock)
	require.True(t, ok)
	assert.Equal(t, "Hello world", tb.Text)
}

func TestStreamMessage_WithToolCalls(t *testing.T) {
	toolInput := json.RawMessage(`{"path":"main.go"}`)

	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Reading file..."},
			cometsdk.ToolCallStartEvent{ID: "tc_01", Name: "read_file"},
			cometsdk.ToolCallDeltaEvent{ID: "tc_01", Delta: `{"path":`},
			cometsdk.ToolCallDeltaEvent{ID: "tc_01", Delta: `"main.go"}`},
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "read_file", Input: toolInput},
			cometsdk.StepFinishEvent{
				FinishReason: "tool_use",
				Usage:        cometsdk.TokenUsage{InputTokens: 30, OutputTokens: 20},
			},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())

	// Track which event types we receive, in order.
	var eventTypes []string
	for ev := range stream.Events() {
		switch ev.(type) {
		case cometsdk.TextDeltaEvent:
			eventTypes = append(eventTypes, "text_delta")
		case cometsdk.ToolCallStartEvent:
			eventTypes = append(eventTypes, "tool_call_start")
		case cometsdk.ToolCallDeltaEvent:
			eventTypes = append(eventTypes, "tool_call_delta")
		case cometsdk.ToolCallDoneEvent:
			eventTypes = append(eventTypes, "tool_call_done")
		case cometsdk.StepFinishEvent:
			eventTypes = append(eventTypes, "step_finish")
		}
	}

	assert.Equal(t, []string{
		"text_delta",
		"tool_call_start",
		"tool_call_delta",
		"tool_call_delta",
		"tool_call_done",
		"step_finish",
	}, eventTypes)

	result, err := stream.Result()
	require.NoError(t, err)
	assert.Equal(t, "tool_use", result.FinishReason)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "read_file", result.ToolCalls[0].Name)
	assert.JSONEq(t, `{"path":"main.go"}`, string(result.ToolCalls[0].Input))

	// Message should have text + tool call blocks.
	require.Len(t, result.Message.Content, 2)
	_, ok := result.Message.Content[0].(cometsdk.TextBlock)
	assert.True(t, ok)
	_, ok = result.Message.Content[1].(cometsdk.ToolCallBlock)
	assert.True(t, ok)
}

func TestStreamMessage_MultipleToolCalls(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			cometsdk.ToolCallDoneEvent{ID: "tc_02", Name: "read_file", Input: json.RawMessage(`{"path":"b.go"}`)},
			cometsdk.StepFinishEvent{FinishReason: "tool_use"},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())
	for range stream.Events() {
	}

	result, err := stream.Result()
	require.NoError(t, err)
	require.Len(t, result.ToolCalls, 2)
	assert.Equal(t, "tc_01", result.ToolCalls[0].ID)
	assert.Equal(t, "tc_02", result.ToolCalls[1].ID)
}

func TestStreamMessage_PreStreamError(t *testing.T) {
	p := &mockProvider{
		id:  "test",
		err: &cometsdk.RateLimitError{ProviderID: "test"},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())

	var count int
	for range stream.Events() {
		count++
	}
	assert.Equal(t, 0, count)

	result, err := stream.Result()
	assert.Nil(t, result)
	require.Error(t, err)

	var rlErr *cometsdk.RateLimitError
	assert.True(t, errors.As(err, &rlErr))
}

func TestStreamMessage_StreamError(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "partial"},
			cometsdk.ErrorEvent{Err: errors.New("server disconnect")},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())

	// We still receive the partial token via events.
	var tokens []string
	for ev := range stream.Events() {
		if td, ok := ev.(cometsdk.TextDeltaEvent); ok {
			tokens = append(tokens, td.Text)
		}
	}
	assert.Equal(t, []string{"partial"}, tokens)

	// Error surfaces via Result.
	result, err := stream.Result()
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Equal(t, "server disconnect", err.Error())
}

func TestStreamMessage_ContextCancelled(t *testing.T) {
	ch := make(chan cometsdk.Event)
	blockingProvider := &blockingMockProvider{ch: ch}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := llm.StreamMessage(ctx, blockingProvider, simpleTextRequest())
	for range stream.Events() {
	}

	result, err := stream.Result()
	assert.Nil(t, result)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestStreamMessage_EmptyResponse(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.StepFinishEvent{FinishReason: "stop", Usage: cometsdk.TokenUsage{InputTokens: 5}},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())

	var eventCount int
	for range stream.Events() {
		eventCount++
	}
	// StepFinishEvent is forwarded.
	assert.Equal(t, 1, eventCount)

	result, err := stream.Result()
	require.NoError(t, err)
	assert.Empty(t, result.ToolCalls)
	assert.Empty(t, result.Message.Content)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Equal(t, 5, result.Usage.InputTokens)
}

func TestStreamMessage_DoesNotForwardErrorOrDone(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "ok"},
			cometsdk.StepFinishEvent{FinishReason: "stop"},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())

	// We should receive TextDeltaEvent and StepFinishEvent, but NOT DoneEvent.
	var eventTypes []string
	for ev := range stream.Events() {
		switch ev.(type) {
		case cometsdk.TextDeltaEvent:
			eventTypes = append(eventTypes, "text_delta")
		case cometsdk.StepFinishEvent:
			eventTypes = append(eventTypes, "step_finish")
		case cometsdk.DoneEvent:
			eventTypes = append(eventTypes, "done") // should NOT appear
		case cometsdk.ErrorEvent:
			eventTypes = append(eventTypes, "error") // should NOT appear
		}
	}

	assert.Equal(t, []string{"text_delta", "step_finish"}, eventTypes)

	result, err := stream.Result()
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Message.Content[0].(cometsdk.TextBlock).Text)
}

func TestStreamMessage_CacheTokensPropagated(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "cached"},
			cometsdk.StepFinishEvent{
				FinishReason: "stop",
				Usage: cometsdk.TokenUsage{
					InputTokens:  100,
					OutputTokens: 10,
					CacheRead:    80,
					CacheWrite:   20,
				},
			},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())
	for range stream.Events() {
	}

	result, err := stream.Result()
	require.NoError(t, err)
	assert.Equal(t, 80, result.Usage.CacheRead)
	assert.Equal(t, 20, result.Usage.CacheWrite)
}

func TestStreamMessage_ResultIdempotent(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "hello"},
			cometsdk.StepFinishEvent{FinishReason: "stop"},
			cometsdk.DoneEvent{},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, simpleTextRequest())
	for range stream.Events() {
	}

	r1, e1 := stream.Result()
	r2, e2 := stream.Result()

	require.NoError(t, e1)
	require.NoError(t, e2)
	assert.Equal(t, r1.FinishReason, r2.FinishReason)
}
