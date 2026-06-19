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

// ─── Mock Provider ────────────────────────────────────────────────────────────

// mockProvider is a test double that emits a predetermined sequence of events.
type mockProvider struct {
	id     string
	events []cometsdk.Event
	err    error // pre-stream error
}

func (m *mockProvider) ID() string { return m.id }

func (m *mockProvider) Stream(_ context.Context, _ *cometsdk.Request) (<-chan cometsdk.Event, error) {
	if m.err != nil {
		return nil, m.err
	}

	ch := make(chan cometsdk.Event, len(m.events))
	for _, e := range m.events {
		ch <- e
	}
	close(ch)
	return ch, nil
}

// simpleTextRequest returns a minimal request for testing.
func simpleTextRequest() *cometsdk.Request {
	return &cometsdk.Request{
		Model:     "test-model",
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hello"}},
			},
		},
	}
}

// ─── Collect Tests ────────────────────────────────────────────────────────────

func TestCollect_TextOnly(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Hello"},
			cometsdk.TextDeltaEvent{Text: ", world!"},
			cometsdk.StepFinishEvent{
				FinishReason: "stop",
				Usage:        cometsdk.TokenUsage{InputTokens: 10, OutputTokens: 5},
			},
			cometsdk.DoneEvent{},
		},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Equal(t, "Hello, world!", resp.Text)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	assert.Empty(t, resp.ToolCalls)

	// Message should be a single TextBlock.
	assert.Equal(t, cometsdk.RoleAssistant, resp.Message.Role)
	require.Len(t, resp.Message.Content, 1)
	tb, ok := resp.Message.Content[0].(cometsdk.TextBlock)
	require.True(t, ok)
	assert.Equal(t, "Hello, world!", tb.Text)
}

func TestCollect_ToolCallOnly(t *testing.T) {
	toolInput := json.RawMessage(`{"city":"Taipei"}`)

	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.ToolCallStartEvent{ID: "tc_01", Name: "get_weather"},
			cometsdk.ToolCallDeltaEvent{ID: "tc_01", Delta: `{"city":`},
			cometsdk.ToolCallDeltaEvent{ID: "tc_01", Delta: `"Taipei"}`},
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "get_weather", Input: toolInput},
			cometsdk.StepFinishEvent{
				FinishReason: "tool_use",
				Usage:        cometsdk.TokenUsage{InputTokens: 20, OutputTokens: 15},
			},
			cometsdk.DoneEvent{},
		},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Empty(t, resp.Text)
	assert.Equal(t, "tool_use", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "tc_01", resp.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Name)
	assert.JSONEq(t, `{"city":"Taipei"}`, string(resp.ToolCalls[0].Input))

	// Message should contain only a ToolCallBlock.
	require.Len(t, resp.Message.Content, 1)
	_, ok := resp.Message.Content[0].(cometsdk.ToolCallBlock)
	assert.True(t, ok)
}

func TestCollect_TextAndToolCalls(t *testing.T) {
	toolInput := json.RawMessage(`{"path":"main.go"}`)

	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Let me read the file."},
			cometsdk.ToolCallStartEvent{ID: "tc_01", Name: "read_file"},
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "read_file", Input: toolInput},
			cometsdk.StepFinishEvent{
				FinishReason: "tool_use",
				Usage:        cometsdk.TokenUsage{InputTokens: 30, OutputTokens: 25},
			},
			cometsdk.DoneEvent{},
		},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Equal(t, "Let me read the file.", resp.Text)
	assert.Equal(t, "tool_use", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)

	// Message should contain both TextBlock and ToolCallBlock.
	require.Len(t, resp.Message.Content, 2)
	_, ok := resp.Message.Content[0].(cometsdk.TextBlock)
	assert.True(t, ok, "first block should be TextBlock")
	_, ok = resp.Message.Content[1].(cometsdk.ToolCallBlock)
	assert.True(t, ok, "second block should be ToolCallBlock")
}

func TestCollect_MultipleToolCalls(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			cometsdk.ToolCallDoneEvent{ID: "tc_02", Name: "read_file", Input: json.RawMessage(`{"path":"b.go"}`)},
			cometsdk.StepFinishEvent{FinishReason: "tool_use", Usage: cometsdk.TokenUsage{InputTokens: 50, OutputTokens: 40}},
			cometsdk.DoneEvent{},
		},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	require.Len(t, resp.ToolCalls, 2)
	assert.Equal(t, "tc_01", resp.ToolCalls[0].ID)
	assert.Equal(t, "tc_02", resp.ToolCalls[1].ID)
	require.Len(t, resp.Message.Content, 2)
}

func TestCollect_PreStreamError(t *testing.T) {
	p := &mockProvider{
		id:  "test",
		err: &cometsdk.AuthError{ProviderID: "test", StatusCode: 401},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	assert.Nil(t, resp)
	require.Error(t, err)

	var authErr *cometsdk.AuthError
	assert.True(t, errors.As(err, &authErr))
}

func TestCollect_StreamError(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "partial"},
			cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: "test", Cause: errors.New("connection reset")}},
			cometsdk.DoneEvent{},
		},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	assert.Nil(t, resp)
	require.Error(t, err)

	var streamErr *cometsdk.StreamError
	assert.True(t, errors.As(err, &streamErr))
}

func TestCollect_ContextCancelled(t *testing.T) {
	// Use a blocking channel so cancellation can be observed.
	ch := make(chan cometsdk.Event)
	blockingProvider := &blockingMockProvider{ch: ch}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	resp, err := llm.Collect(ctx, blockingProvider, simpleTextRequest())
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.Canceled)
}

// blockingMockProvider returns a channel that never sends events.
type blockingMockProvider struct {
	ch chan cometsdk.Event
}

func (b *blockingMockProvider) ID() string { return "blocking" }

func (b *blockingMockProvider) Stream(_ context.Context, _ *cometsdk.Request) (<-chan cometsdk.Event, error) {
	return b.ch, nil
}

// ─── GenerateText Tests ───────────────────────────────────────────────────────

func TestGenerateText_Success(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Paris"},
			cometsdk.StepFinishEvent{
				FinishReason: "stop",
				Usage:        cometsdk.TokenUsage{InputTokens: 8, OutputTokens: 3},
			},
			cometsdk.DoneEvent{},
		},
	}

	result, err := llm.GenerateText(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Equal(t, "Paris", result.Text)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Equal(t, 8, result.Usage.InputTokens)
	assert.Equal(t, 3, result.Usage.OutputTokens)
}

func TestGenerateText_RejectsToolCalls(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Let me check."},
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "get_weather", Input: json.RawMessage(`{}`)},
			cometsdk.StepFinishEvent{FinishReason: "tool_use"},
			cometsdk.DoneEvent{},
		},
	}

	result, err := llm.GenerateText(context.Background(), p, simpleTextRequest())
	assert.Nil(t, result)
	assert.ErrorIs(t, err, llm.ErrUnexpectedToolCall)
}

func TestGenerateText_PropagatesPreStreamError(t *testing.T) {
	p := &mockProvider{
		id:  "test",
		err: &cometsdk.RateLimitError{ProviderID: "test"},
	}

	result, err := llm.GenerateText(context.Background(), p, simpleTextRequest())
	assert.Nil(t, result)
	require.Error(t, err)

	var rlErr *cometsdk.RateLimitError
	assert.True(t, errors.As(err, &rlErr))
}

// ─── GenerateMessage Tests ────────────────────────────────────────────────────

func TestGenerateMessage_TextOnly(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "The answer is 42."},
			cometsdk.StepFinishEvent{FinishReason: "stop", Usage: cometsdk.TokenUsage{InputTokens: 12, OutputTokens: 7}},
			cometsdk.DoneEvent{},
		},
	}

	result, err := llm.GenerateMessage(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Equal(t, cometsdk.RoleAssistant, result.Message.Role)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Empty(t, result.ToolCalls)
	require.Len(t, result.Message.Content, 1)
}

func TestGenerateMessage_WithToolCalls(t *testing.T) {
	toolInput := json.RawMessage(`{"city":"Tokyo"}`)

	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "Checking weather..."},
			cometsdk.ToolCallDoneEvent{ID: "tc_01", Name: "get_weather", Input: toolInput},
			cometsdk.StepFinishEvent{FinishReason: "tool_use", Usage: cometsdk.TokenUsage{InputTokens: 20, OutputTokens: 15}},
			cometsdk.DoneEvent{},
		},
	}

	result, err := llm.GenerateMessage(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Equal(t, "tool_use", result.FinishReason)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "get_weather", result.ToolCalls[0].Name)
	assert.JSONEq(t, `{"city":"Tokyo"}`, string(result.ToolCalls[0].Input))

	// Message should contain text + tool call.
	require.Len(t, result.Message.Content, 2)
}

func TestGenerateMessage_PropagatesStreamError(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.ErrorEvent{Err: errors.New("boom")},
			cometsdk.DoneEvent{},
		},
	}

	result, err := llm.GenerateMessage(context.Background(), p, simpleTextRequest())
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Equal(t, "boom", err.Error())
}

// ─── QuickText Tests ──────────────────────────────────────────────────────────

func TestQuickText_Success(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.TextDeltaEvent{Text: "42"},
			cometsdk.StepFinishEvent{FinishReason: "stop"},
			cometsdk.DoneEvent{},
		},
	}

	text, err := llm.QuickText(context.Background(), p, "test-model", "What is the answer?")
	require.NoError(t, err)
	assert.Equal(t, "42", text)
}

func TestQuickText_Error(t *testing.T) {
	p := &mockProvider{
		id:  "test",
		err: errors.New("network error"),
	}

	text, err := llm.QuickText(context.Background(), p, "test-model", "Hello")
	assert.Empty(t, text)
	require.Error(t, err)
}

// ─── Edge Cases ───────────────────────────────────────────────────────────────

func TestCollect_EmptyResponse(t *testing.T) {
	p := &mockProvider{
		id: "test",
		events: []cometsdk.Event{
			cometsdk.StepFinishEvent{FinishReason: "stop", Usage: cometsdk.TokenUsage{InputTokens: 5, OutputTokens: 0}},
			cometsdk.DoneEvent{},
		},
	}

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Empty(t, resp.Text)
	assert.Empty(t, resp.ToolCalls)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Empty(t, resp.Message.Content)
}

func TestCollect_CacheTokens(t *testing.T) {
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

	resp, err := llm.Collect(context.Background(), p, simpleTextRequest())
	require.NoError(t, err)

	assert.Equal(t, 80, resp.Usage.CacheRead)
	assert.Equal(t, 20, resp.Usage.CacheWrite)
}
