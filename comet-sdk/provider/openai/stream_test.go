package openai

import (
	"context"
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
	return NewOpenAIProvider("test-key",
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
		Model:    "gpt-4o",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)

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

func TestStream_MultipleToolCalls(t *testing.T) {
	srv := serveFixture(t, "fixtures/tool_calls_multi.sse")
	defer srv.Close()

	p := newTestProvider(t, srv)
	req := &cometsdk.Request{
		Model:    "gpt-4o",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Do things"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)

	var starts []cometsdk.ToolCallStartEvent
	var dones []cometsdk.ToolCallDoneEvent
	var stepFinish *cometsdk.StepFinishEvent

	for _, e := range events {
		switch ev := e.(type) {
		case cometsdk.ToolCallStartEvent:
			starts = append(starts, ev)
		case cometsdk.ToolCallDoneEvent:
			dones = append(dones, ev)
		case cometsdk.StepFinishEvent:
			stepFinish = &ev
		}
	}

	require.Len(t, starts, 2)
	require.Len(t, dones, 2)
	require.NotNil(t, stepFinish)
	require.Equal(t, "tool_use", stepFinish.FinishReason)
	require.Equal(t, 20, stepFinish.Usage.InputTokens)
	require.Equal(t, 15, stepFinish.Usage.OutputTokens)

	names := map[string]bool{}
	for _, d := range dones {
		names[d.Name] = true
	}
	require.True(t, names["read_file"])
	require.True(t, names["list_dir"])
}

func TestStream_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
				w.Write([]byte(": ping\n\n")) //nolint:errcheck
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	p := newTestProvider(t, srv)
	req := &cometsdk.Request{
		Model:    "gpt-4o",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(ctx, req)
	require.NoError(t, err)

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

	p := NewOpenAIProvider("test-key",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(3),
	)
	req := &cometsdk.Request{
		Model:    "gpt-4o",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)
	require.GreaterOrEqual(t, attempts, 2)

	var done bool
	for _, e := range events {
		if _, ok := e.(cometsdk.DoneEvent); ok {
			done = true
		}
	}
	require.True(t, done)
}

// TestStream_ImageFallbackOnUnsupported simulates a non-vision OpenAI-compatible
// model (e.g. DeepSeek): the first request carries an "image_url" content part
// and is rejected with HTTP 400, and the provider must retry once with the image
// downgraded to text — which then succeeds.
func TestStream_ImageFallbackOnUnsupported(t *testing.T) {
	fixtureData, err := os.ReadFile("fixtures/text_response.sse")
	require.NoError(t, err)

	var sawImageURL []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		hasImageURL := strings.Contains(string(body), "image_url")
		sawImageURL = append(sawImageURL, hasImageURL)
		if hasImageURL {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"Failed to deserialize the JSON body into the target type: messages[0]: unknown variant ` + "`image_url`" + `, expected ` + "`text`" + `"}}`)) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData) //nolint:errcheck
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
	)
	req := &cometsdk.Request{
		Model: "deepseek-chat",
		Messages: []cometsdk.Message{{
			Role: cometsdk.RoleUser,
			Content: []cometsdk.Block{
				cometsdk.TextBlock{Text: "What is this?"},
				cometsdk.ImageBlock{MediaType: "image/png", Data: "aGVsbG8="},
			},
		}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)

	// First attempt sent image_url, second (fallback) did not.
	require.Equal(t, []bool{true, false}, sawImageURL)

	var done bool
	for _, e := range events {
		if _, ok := e.(cometsdk.DoneEvent); ok {
			done = true
		}
	}
	require.True(t, done)
}

// TestStream_ReasoningSplitFallbackOnUnsupported retries without reasoning_split
// when a gateway rejects the non-standard field (LiteLLM, Azure, Anthropic, etc.).
func TestStream_ReasoningSplitFallbackOnUnsupported(t *testing.T) {
	fixtureData, err := os.ReadFile("fixtures/text_response.sse")
	require.NoError(t, err)

	var sawReasoningSplit []bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		hasReasoningSplit := strings.Contains(string(body), "reasoning_split")
		sawReasoningSplit = append(sawReasoningSplit, hasReasoningSplit)
		if hasReasoningSplit {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"litellm.BadRequestError: Unknown parameter: 'reasoning_split'"}}`)) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData) //nolint:errcheck
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
	)
	req := &cometsdk.Request{
		Model:    "MiniMax-M3",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)
	require.Equal(t, []bool{true, false}, sawReasoningSplit)

	var done bool
	for _, e := range events {
		if _, ok := e.(cometsdk.DoneEvent); ok {
			done = true
		}
	}
	require.True(t, done)
}

// TestStream_MaxCompletionTokensFallbackOnUnsupported retries with
// max_completion_tokens when a newer OpenAI model rejects max_tokens.
func TestStream_MaxCompletionTokensFallbackOnUnsupported(t *testing.T) {
	fixtureData, err := os.ReadFile("fixtures/text_response.sse")
	require.NoError(t, err)

	var fields []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		payload := string(body)
		switch {
		case strings.Contains(payload, "max_tokens"):
			fields = append(fields, "max_tokens")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":{"message":"Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead."}}`)) //nolint:errcheck
			return
		case strings.Contains(payload, "max_completion_tokens"):
			fields = append(fields, "max_completion_tokens")
		default:
			fields = append(fields, "none")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(fixtureData) //nolint:errcheck
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
	)
	req := &cometsdk.Request{
		Model:     "gpt-5.5",
		MaxTokens: 64,
		Messages:  []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	events := collectEvents(t, ch)
	require.Equal(t, []string{"max_tokens", "max_completion_tokens"}, fields)

	var done bool
	for _, e := range events {
		if _, ok := e.(cometsdk.DoneEvent); ok {
			done = true
		}
	}
	require.True(t, done)
}

// TestStream_NoImageFallbackWithoutImage ensures a 400 that is unrelated to
// images is NOT retried with the image-downgrade path (it just fails).
func TestStream_NoImageFallbackWithoutImage(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"some unrelated bad request"}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key",
		cometsdk.WithBaseURL(srv.URL),
		cometsdk.WithMaxRetries(1),
	)
	req := &cometsdk.Request{
		Model:    "deepseek-chat",
		Messages: []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}}},
	}

	_, err := p.Stream(context.Background(), req)
	require.Error(t, err)
	// Only the initial attempt; no image-downgrade retry (there was no image).
	require.Equal(t, 1, attempts)
}
