package responsesproto

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

const (
	fixtureProviderID = "codex"
	fixtureModelID    = "gpt-5"
)

// streamFixture runs ParseLoop over a checked-in SSE fixture and returns every
// event it emits, in order.
func streamFixture(t *testing.T, name string, emitToolStart bool) []cometsdk.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures", name))
	require.NoError(t, err)

	body := io.NopCloser(bytes.NewReader(data))
	ch := make(chan cometsdk.Event, 64)
	go ParseLoop(context.Background(), fixtureProviderID, fixtureModelID, emitToolStart, body, ch, slog.New(slog.DiscardHandler), 0)

	var events []cometsdk.Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func TestParseLoop_Fixtures(t *testing.T) {
	t.Parallel()

	done := cometsdk.DoneEvent{}
	tests := []struct {
		name          string
		fixture       string
		suppressStart bool
		want          []cometsdk.Event
	}{
		{
			name:    "text only",
			fixture: "text_only.sse",
			want: []cometsdk.Event{
				cometsdk.TextDeltaEvent{Text: "Hello"},
				cometsdk.TextDeltaEvent{Text: ", world!"},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: cometsdk.TokenUsage{InputTokens: 12, OutputTokens: 4}},
				done,
			},
		},
		{
			name:    "reasoning summary with encrypted state",
			fixture: "reasoning_summary.sse",
			want: []cometsdk.Event{
				cometsdk.ReasoningStartEvent{},
				cometsdk.ReasoningContentEvent{Text: "Checking"},
				cometsdk.ReasoningContentEvent{Text: " the files."},
				cometsdk.ReasoningContentEvent{Text: "Checking the files."},
				cometsdk.ProviderStateEvent{State: cometsdk.ProviderState{ProviderID: fixtureProviderID, ModelID: fixtureModelID, Data: "opaque-state-01"}},
				cometsdk.TextDeltaEvent{Text: "Done."},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: cometsdk.TokenUsage{InputTokens: 20, OutputTokens: 9}},
				done,
			},
		},
		{
			name:    "encrypted reasoning without summary",
			fixture: "reasoning_encrypted_only.sse",
			want: []cometsdk.Event{
				cometsdk.ProviderStateEvent{State: cometsdk.ProviderState{ProviderID: fixtureProviderID, ModelID: fixtureModelID, Data: "opaque-state-02"}},
				cometsdk.TextDeltaEvent{Text: "Hi"},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: cometsdk.TokenUsage{InputTokens: 3, OutputTokens: 1}},
				done,
			},
		},
		{
			name:    "tool call with streamed arguments",
			fixture: "tool_call.sse",
			want: []cometsdk.Event{
				cometsdk.ToolCallStartEvent{ID: "call_01", Name: "read_file"},
				cometsdk.ToolCallDeltaEvent{ID: "call_01", Delta: `{"path":"main.go"}`},
				cometsdk.ToolCallDoneEvent{ID: "call_01", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishToolUse, Usage: cometsdk.TokenUsage{InputTokens: 30, OutputTokens: 8}},
				done,
			},
		},
		{
			name:          "tool call with start suppressed until done",
			fixture:       "tool_call.sse",
			suppressStart: true,
			want: []cometsdk.Event{
				cometsdk.ToolCallStartEvent{ID: "call_01", Name: "read_file"},
				cometsdk.ToolCallDeltaEvent{ID: "call_01", Delta: `{"path":"main.go"}`},
				cometsdk.ToolCallDoneEvent{ID: "call_01", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishToolUse, Usage: cometsdk.TokenUsage{InputTokens: 30, OutputTokens: 8}},
				done,
			},
		},
		{
			name:    "top-level arguments done with string-wrapped arguments",
			fixture: "tool_call_arguments_done.sse",
			want: []cometsdk.Event{
				cometsdk.ToolCallStartEvent{ID: "call_01", Name: "list_dir"},
				cometsdk.ToolCallDeltaEvent{ID: "call_01", Delta: `{"path":"."}`},
				cometsdk.ToolCallDoneEvent{ID: "call_01", Name: "list_dir", Input: json.RawMessage(`{"path":"."}`)},
				done,
			},
		},
		{
			name:    "usage with cached tokens",
			fixture: "usage_cached.sse",
			want: []cometsdk.Event{
				cometsdk.TextDeltaEvent{Text: "ok"},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: cometsdk.TokenUsage{InputTokens: 100, OutputTokens: 5, CacheRead: 80, CacheWrite: 15}},
				done,
			},
		},
		{
			name:    "incomplete at max output tokens",
			fixture: "incomplete_max_tokens.sse",
			want: []cometsdk.Event{
				cometsdk.TextDeltaEvent{Text: "Trunc"},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishMaxTokens, Usage: cometsdk.TokenUsage{InputTokens: 6, OutputTokens: 64}},
				done,
			},
		},
		{
			name:    "EOF without completed event",
			fixture: "eof_without_completed.sse",
			want: []cometsdk.Event{
				cometsdk.TextDeltaEvent{Text: "cut off"},
				cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
				done,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, streamFixture(t, tt.fixture, !tt.suppressStart))
		})
	}
}

func TestParseLoop_ErrorFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fixture    string
		wantBefore []cometsdk.Event
		wantCause  string
	}{
		{
			name:       "error event stops the stream",
			fixture:    "error_event.sse",
			wantBefore: []cometsdk.Event{cometsdk.TextDeltaEvent{Text: "Par"}},
			wantCause:  "responses: upstream overloaded",
		},
		{
			name:      "response failed without message",
			fixture:   "response_failed.sse",
			wantCause: "responses: stream failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			events := streamFixture(t, tt.fixture, true)
			require.NotEmpty(t, events)

			last, ok := events[len(events)-1].(cometsdk.ErrorEvent)
			require.True(t, ok, "last event should be an ErrorEvent, got %T", events[len(events)-1])
			var streamErr *cometsdk.StreamError
			require.True(t, errors.As(last.Err, &streamErr))
			require.Equal(t, fixtureProviderID, streamErr.ProviderID)
			require.EqualError(t, streamErr.Cause, tt.wantCause)

			before := events[:len(events)-1]
			if len(tt.wantBefore) == 0 {
				require.Empty(t, before)
				return
			}
			require.Equal(t, tt.wantBefore, before)
		})
	}
}
