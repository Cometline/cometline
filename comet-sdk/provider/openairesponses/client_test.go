package openairesponses

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

func collectEvents(t *testing.T, ch <-chan cometsdk.Event) []cometsdk.Event {
	t.Helper()
	var events []cometsdk.Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func newTestProvider(t *testing.T, srv *httptest.Server) cometsdk.Provider {
	t.Helper()
	return NewOpenAIResponsesProvider("test-key", "opencode-go",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
	)
}

func TestStream_RequestShape(t *testing.T) {
	var gotPath, gotAuth string
	var body map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"))
	}))
	defer srv.Close()

	temp := 0.3
	p := newTestProvider(t, srv)
	req := &cometsdk.Request{
		Model:       "gpt-5.6-luna",
		System:      "Be concise.",
		MaxTokens:   512,
		Temperature: &temp,
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hello"}}},
		},
		Tools: []cometsdk.Tool{{Name: "web_search", Description: "Search", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}
	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)
	_ = collectEvents(t, ch)

	require.Equal(t, "/v1/responses", gotPath)
	require.Equal(t, "Bearer test-key", gotAuth)
	require.Equal(t, "gpt-5.6-luna", body["model"])
	require.Equal(t, "Be concise.", body["instructions"])
	require.Equal(t, false, body["store"])
	require.Equal(t, true, body["stream"])
	require.Equal(t, float64(512), body["max_output_tokens"])
	require.Equal(t, 0.3, body["temperature"])
	require.Equal(t, []any{"reasoning.encrypted_content"}, body["include"])

	input, ok := body["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 1)
	user := input[0].(map[string]any)
	require.Equal(t, "user", user["role"])
	parts := user["content"].([]any)
	require.Equal(t, map[string]any{"type": "input_text", "text": "hello"}, parts[0])

	tools, ok := body["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	tool := tools[0].(map[string]any)
	require.Equal(t, "function", tool["type"])
	require.Equal(t, "web_search", tool["name"])
}

func TestStream_TextAndToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"fc_1\",\"call_id\":\"call_1\",\"type\":\"function_call\",\"name\":\"web_search\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_1\",\"delta\":\"{\\\"query\\\":\\\"heptabase\\\"}\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"fc_1\",\"call_id\":\"call_1\",\"type\":\"function_call\",\"name\":\"web_search\",\"arguments\":{\"query\":\"heptabase\"}}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":7,\"output_tokens\":3}}}\n\n"))
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(context.Background(), &cometsdk.Request{
		Model:    "gpt-5.6-luna",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	})
	require.NoError(t, err)
	events := collectEvents(t, ch)

	var texts []string
	var starts []cometsdk.ToolCallStartEvent
	var dones []cometsdk.ToolCallDoneEvent
	var stepFinish *cometsdk.StepFinishEvent
	for _, e := range events {
		switch ev := e.(type) {
		case cometsdk.TextDeltaEvent:
			texts = append(texts, ev.Text)
		case cometsdk.ToolCallStartEvent:
			starts = append(starts, ev)
		case cometsdk.ToolCallDoneEvent:
			dones = append(dones, ev)
		case cometsdk.StepFinishEvent:
			stepFinish = &ev
		}
	}

	require.Equal(t, []string{"Hello"}, texts)
	require.Len(t, starts, 1)
	require.Equal(t, "call_1", starts[0].ID)
	require.Equal(t, "web_search", starts[0].Name)
	require.Len(t, dones, 1)
	require.Equal(t, `{"query":"heptabase"}`, string(dones[0].Input))
	require.NotNil(t, stepFinish)
	require.Equal(t, cometsdk.FinishToolUse, stepFinish.FinishReason)
	require.Equal(t, 7, stepFinish.Usage.InputTokens)
	require.Equal(t, 3, stepFinish.Usage.OutputTokens)
}

func TestStream_IncompleteMaxOutputTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.incomplete\",\"response\":{\"incomplete_details\":{\"reason\":\"max_output_tokens\"},\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"))
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(context.Background(), &cometsdk.Request{
		Model:     "gpt-5.6-luna",
		MaxTokens: 64,
		Messages:  []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	})
	require.NoError(t, err)
	events := collectEvents(t, ch)

	var stepFinish *cometsdk.StepFinishEvent
	var done bool
	for _, e := range events {
		switch ev := e.(type) {
		case cometsdk.StepFinishEvent:
			stepFinish = &ev
		case cometsdk.DoneEvent:
			done = true
		}
	}
	require.NotNil(t, stepFinish)
	require.Equal(t, cometsdk.FinishMaxTokens, stepFinish.FinishReason)
	require.Equal(t, 2, stepFinish.Usage.OutputTokens)
	require.True(t, done)
}

func TestStream_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.failed\",\"error\":{\"message\":\"model overloaded\"}}\n\n"))
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(context.Background(), &cometsdk.Request{
		Model:    "gpt-5.6-luna",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	})
	require.NoError(t, err)
	events := collectEvents(t, ch)

	var errMsg string
	for _, e := range events {
		if ee, ok := e.(cometsdk.ErrorEvent); ok {
			errMsg = ee.Err.Error()
		}
	}
	require.Contains(t, errMsg, "model overloaded")
}

func TestStream_EncryptedReasoningReplayAndState(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"reasoning\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"plan\"}],\"encrypted_content\":\"opaque-state\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{}}}\n\n"))
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(context.Background(), &cometsdk.Request{
		Model: "gpt-5.6-luna",
		Messages: []cometsdk.Message{{
			Role: cometsdk.RoleAssistant,
			ProviderState: []cometsdk.ProviderState{{
				ProviderID: "opencode-go",
				ModelID:    "gpt-5.6-luna",
				Data:       "opaque-state",
			}},
			ReasoningContent: []cometsdk.Block{cometsdk.ReasoningBlock{Text: "Earlier plan"}},
		}},
	})
	require.NoError(t, err)
	events := collectEvents(t, ch)

	input := body["input"].([]any)
	require.Len(t, input, 1)
	item := input[0].(map[string]any)
	require.Equal(t, "reasoning", item["type"])
	require.Equal(t, "opaque-state", item["encrypted_content"])
	summary, ok := item["summary"].([]any)
	require.True(t, ok, "summary must be present for encrypted reasoning replay")
	require.Len(t, summary, 1)

	var stateEvent *cometsdk.ProviderStateEvent
	for _, e := range events {
		if se, ok := e.(cometsdk.ProviderStateEvent); ok {
			stateEvent = &se
		}
	}
	require.NotNil(t, stateEvent)
	require.Equal(t, "opencode-go", stateEvent.State.ProviderID)
	require.Equal(t, "gpt-5.6-luna", stateEvent.State.ModelID)
	require.Equal(t, "opaque-state", stateEvent.State.Data)
}

func TestStream_MaxOutputTokensFallbackOnUnsupported(t *testing.T) {
	var fields []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		payload := string(raw)
		if strings.Contains(payload, "max_output_tokens") {
			fields = append(fields, "max_output_tokens")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: max_output_tokens"}}`))
			return
		}
		fields = append(fields, "none")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{}}}\n\n"))
	}))
	defer srv.Close()

	p := newTestProvider(t, srv)
	ch, err := p.Stream(context.Background(), &cometsdk.Request{
		Model:     "gpt-5.6-luna",
		MaxTokens: 64,
		Messages:  []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	})
	require.NoError(t, err)
	events := collectEvents(t, ch)
	require.Equal(t, []string{"max_output_tokens", "none"}, fields)
	require.Contains(t, events, cometsdk.DoneEvent{})
}
