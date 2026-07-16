package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/sse"
	"github.com/stretchr/testify/require"
)

func TestWriteWebSocketSSEEventPreservesMultilineJSON(t *testing.T) {
	payload := []byte("{\n  \"type\": \"response.output_text.delta\",\n  \"delta\": \"hello\"\n}")
	var buffer bytes.Buffer
	require.NoError(t, writeWebSocketSSEEvent(&buffer, payload))

	scanner := sse.NewScanner(&buffer)
	require.True(t, scanner.Next())
	require.Equal(t, string(payload), scanner.Event().Data)
}

func TestStream_UsesSSEOnly(t *testing.T) {
	codeHome := t.TempDir()
	t.Setenv("CODEX_HOME", codeHome)
	accessToken := writeTestAuthFile(t, codexAuthPath())
	require.NoError(t, os.WriteFile(filepath.Join(codeHome, "installation_id"), []byte("installation-test\n"), 0o600))

	var request map[string]any
	var requestHeaders http.Header
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		requestCount++
		requestHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{}}}\n\n"))
	}))
	defer srv.Close()

	p := NewCodexProvider(
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
		cometsdk.WithResponseTransport(cometsdk.ResponseTransportWebSocket),
	)
	req := &cometsdk.Request{
		Model:    "gpt-5.6-luna",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)
	events := collectEvents(t, ch)

	require.Equal(t, 1, requestCount)
	require.Equal(t, accessToken, strings.TrimPrefix(requestHeaders.Get("Authorization"), "Bearer "))
	require.Equal(t, "installation-test", requestHeaders.Get("x-codex-installation-id"))
	require.Empty(t, requestHeaders.Get("OpenAI-Beta"))
	require.Equal(t, "true", requestHeaders.Get(responsesLiteHeader))
	require.NotContains(t, request, "type")
	require.Equal(t, "gpt-5.6-luna", request["model"])
	require.Equal(t, false, request["store"])
	require.Equal(t, true, request["stream"])
	require.Equal(t, false, request["parallel_tool_calls"])
	require.Equal(t, map[string]any{"context": "all_turns"}, request["reasoning"])
	require.Contains(t, events, cometsdk.TextDeltaEvent{Text: "hello"})
	require.Contains(t, events, cometsdk.DoneEvent{})
}

func TestCodexWebSocketRequest_NonLunaUsesStandardResponsesFields(t *testing.T) {
	for _, model := range []string{"gpt-5.4", "gpt-5.4-mini", "gpt-5.6-sol", "gpt-5.6-terra"} {
		t.Run(model, func(t *testing.T) {
			req := &cometsdk.Request{
				Model:    model,
				Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
			}

			data, err := toCodexWebSocketRequest(req)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(data, &payload))
			require.Equal(t, true, payload["parallel_tool_calls"])
			require.NotContains(t, payload, "reasoning")
			require.NotContains(t, payload, "include")
			require.False(t, codexUsesResponsesLite(req.Model))
		})
	}
}
