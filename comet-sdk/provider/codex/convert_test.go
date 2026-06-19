package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

func TestConvertRequest_ResponsesShape(t *testing.T) {
	temp := 0.2
	req := &cometsdk.Request{
		Model:       "gpt-5.4",
		System:      "Be helpful.",
		MaxTokens:   123,
		Temperature: &temp,
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{
				cometsdk.TextBlock{Text: "Describe this"},
				cometsdk.ImageBlock{MediaType: "image/png", Data: "aGVsbG8="},
			}},
			{Role: cometsdk.RoleAssistant, Content: []cometsdk.Block{
				cometsdk.TextBlock{Text: "Calling a tool"},
				cometsdk.ToolCallBlock{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)},
			}},
			{Role: cometsdk.RoleToolResult, Content: []cometsdk.Block{
				cometsdk.ToolResultBlock{ToolCallID: "call_1", Content: "file contents"},
			}},
		},
		Tools: []cometsdk.Tool{{Name: "read_file", Description: "Read a file", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}

	data, err := toCodexRequest(req, false)
	require.NoError(t, err)

	var out codexRequest
	require.NoError(t, json.Unmarshal(data, &out))
	require.Equal(t, "gpt-5.4", out.Model)
	require.Equal(t, "Be helpful.", out.Instructions)
	require.True(t, out.Stream)
	require.False(t, out.Store)
	require.Equal(t, 123, out.MaxOutputTokens)
	require.Len(t, out.Input, 4)
	require.Equal(t, "user", out.Input[0].Role)
	require.Equal(t, "input_image", out.Input[0].Content[1].Type)
	require.Equal(t, "function_call", out.Input[2].Type)
	require.Equal(t, "function_call_output", out.Input[3].Type)
	require.Len(t, out.Tools, 1)
	require.False(t, out.Tools[0].Strict)

	data, err = toCodexRequest(req, true)
	require.NoError(t, err)
	require.NotContains(t, string(data), "max_output_tokens")
}

func TestConvertEvent_TextToolAndCompleted(t *testing.T) {
	state := &codexStreamState{}

	events, err := toSDKEvents(`{"type":"response.output_text.delta","delta":"hello"}`, state)
	require.NoError(t, err)
	require.Equal(t, cometsdk.TextDeltaEvent{Text: "hello"}, events[0])

	events, err = toSDKEvents(`{"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_1","name":"read_file","arguments":{"path":"main.go"}}}`, state)
	require.NoError(t, err)
	require.Len(t, events, 3)
	require.Equal(t, cometsdk.ToolCallStartEvent{ID: "call_1", Name: "read_file"}, events[0])
	require.Equal(t, cometsdk.ToolCallDeltaEvent{ID: "call_1", Delta: `{"path":"main.go"}`}, events[1])
	require.Equal(t, cometsdk.ToolCallDoneEvent{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"main.go"}`)}, events[2])

	events, err = toSDKEvents(`{"type":"response.completed","response":{"usage":{"input_tokens":7,"output_tokens":3}}}`, state)
	require.NoError(t, err)
	require.Equal(t, cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishToolUse, Usage: cometsdk.TokenUsage{InputTokens: 7, OutputTokens: 3}}, events[0])
	require.Equal(t, cometsdk.DoneEvent{}, events[1])
}

func TestJWTExpiry(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))

	got, ok := jwtExpiry(header + "." + payload + ".")
	require.True(t, ok)
	require.Equal(t, exp, got.Unix())
}

func TestStream_MaxOutputTokensFallbackOnUnsupported(t *testing.T) {
	t.Setenv("CODEX_HOME", t.TempDir())
	accessToken := writeTestAuthFile(t, codexAuthPath())

	var fields []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/responses", r.URL.Path)
		require.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		if strings.Contains(payload, "max_output_tokens") {
			fields = append(fields, "max_output_tokens")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"Unsupported parameter: max_output_tokens"}}`)) //nolint:errcheck
			return
		}
		fields = append(fields, "none")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")) //nolint:errcheck
		w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{}}}\n\n")) //nolint:errcheck
	}))
	defer srv.Close()

	p := NewCodexProvider(
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
	)
	req := &cometsdk.Request{
		Model:     "gpt-5.4",
		MaxTokens: 64,
		Messages:  []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)
	events := collectEvents(t, ch)
	require.Equal(t, []string{"max_output_tokens", "none"}, fields)
	require.Contains(t, events, cometsdk.TextDeltaEvent{Text: "hello"})
	require.Contains(t, events, cometsdk.DoneEvent{})
}

func writeTestAuthFile(t *testing.T, path string) string {
	t.Helper()
	exp := time.Now().Add(time.Hour).Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	accessToken := header + "." + payload + "."
	data := fmt.Sprintf(`{"auth_mode":"chatgpt","tokens":{"access_token":"%s","refresh_token":"test-refresh"}}`, accessToken)
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	return accessToken
}

func collectEvents(t *testing.T, ch <-chan cometsdk.Event) []cometsdk.Event {
	t.Helper()
	var events []cometsdk.Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}
