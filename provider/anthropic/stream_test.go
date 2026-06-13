package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

// serveFixture starts a test server that streams the given SSE fixture file.
func serveFixture(t *testing.T, fixturePath string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data) //nolint:errcheck
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
}

func newTestProvider(t *testing.T, server *httptest.Server) cometsdk.Provider {
	t.Helper()
	return NewAnthropicProvider("test-key",
		cometsdk.WithBaseURL(server.URL),
		cometsdk.WithMaxRetries(1),
	)
}

func collectEvents(t *testing.T, ch <-chan cometsdk.Event) []cometsdk.Event {
	t.Helper()
	var events []cometsdk.Event
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func TestStream_TextOnly(t *testing.T) {
	srv := serveFixture(t, "fixtures/text_response.sse")
	defer srv.Close()

	p := newTestProvider(t, srv)
	req := &cometsdk.Request{
		Model:    "claude-sonnet-4-5",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)

	// Expect: TextDeltaEvent × 2, StepFinishEvent, DoneEvent
	var texts []string
	var stepFinish *cometsdk.StepFinishEvent
	var done bool

	for _, e := range events {
		switch ev := e.(type) {
		case cometsdk.TextDeltaEvent:
			texts = append(texts, ev.Text)
		case cometsdk.StepFinishEvent:
			stepFinish = &ev
		case cometsdk.DoneEvent:
			done = true
		}
	}

	require.Equal(t, []string{"Hello", ", world!"}, texts)
	require.NotNil(t, stepFinish)
	require.Equal(t, "stop", stepFinish.FinishReason)
	require.Equal(t, 10, stepFinish.Usage.InputTokens)
	require.Equal(t, 5, stepFinish.Usage.OutputTokens)
	require.True(t, done)
}

func TestStream_ToolCall(t *testing.T) {
	srv := serveFixture(t, "fixtures/tool_call.sse")
	defer srv.Close()

	p := newTestProvider(t, srv)
	req := &cometsdk.Request{
		Model:    "claude-sonnet-4-5",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Read main.go"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)

	var started *cometsdk.ToolCallStartEvent
	var done *cometsdk.ToolCallDoneEvent
	var stepFinish *cometsdk.StepFinishEvent

	for _, e := range events {
		switch ev := e.(type) {
		case cometsdk.ToolCallStartEvent:
			started = &ev
		case cometsdk.ToolCallDoneEvent:
			done = &ev
		case cometsdk.StepFinishEvent:
			stepFinish = &ev
		}
	}

	require.NotNil(t, started)
	require.Equal(t, "toolu_01", started.ID)
	require.Equal(t, "read_file", started.Name)

	require.NotNil(t, done)
	require.Equal(t, "toolu_01", done.ID)
	require.Equal(t, "read_file", done.Name)
	require.JSONEq(t, `{"path":"main.go"}`, string(done.Input))

	require.NotNil(t, stepFinish)
	require.Equal(t, "tool_use", stepFinish.FinishReason)
}

func TestStream_ContextCancelled(t *testing.T) {
	// Server that streams slowly, one byte at a time.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
				w.Write([]byte(":ping\n\n")) //nolint:errcheck
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	p := newTestProvider(t, srv)
	req := &cometsdk.Request{
		Model:    "claude-sonnet-4-5",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(ctx, req)
	require.NoError(t, err)

	// Cancel after a short delay.
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	var gotError bool
	for e := range ch {
		if _, ok := e.(cometsdk.ErrorEvent); ok {
			gotError = true
		}
	}

	// Channel must be closed (range ended) — no goroutine leak.
	require.True(t, gotError, "expected ErrorEvent after cancellation")
}

func TestStream_RateLimitRetry(t *testing.T) {
	attempts := 0
	fixtureData, err := os.ReadFile("fixtures/text_response.sse")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData) //nolint:errcheck
	}))
	defer srv.Close()

	p := NewAnthropicProvider("test-key",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(3),
	)
	req := &cometsdk.Request{
		Model:    "claude-sonnet-4-5",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)
	require.GreaterOrEqual(t, attempts, 2, "expected at least one retry")

	var done bool
	for _, e := range events {
		if _, ok := e.(cometsdk.DoneEvent); ok {
			done = true
		}
	}
	require.True(t, done)
}
