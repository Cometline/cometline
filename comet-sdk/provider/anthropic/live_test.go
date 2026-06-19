//go:build live

package anthropic_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/anthropic"
	"github.com/stretchr/testify/require"
)

// newLiveProvider resolves the API key and optional base URL from the environment.
//
// Key resolution order:
//  1. CUSTOM_API_KEY  — company unified API key (supports both Anthropic + OpenAI formats)
//  2. ANTHROPIC_API_KEY — fallback for direct Anthropic access
//
// CUSTOM_BASE_URL overrides the default API endpoint (https://api.anthropic.com).
// Use this to point at a company unified API that proxies Anthropic's /v1/messages.
func newLiveProvider(t *testing.T) cometsdk.Provider {
	t.Helper()

	apiKey := os.Getenv("CUSTOM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		t.Skip("neither CUSTOM_API_KEY nor ANTHROPIC_API_KEY is set")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	opts := []cometsdk.Option{cometsdk.WithLogger(log)}
	if baseURL := os.Getenv("CUSTOM_BASE_URL"); baseURL != "" {
		opts = append(opts, cometsdk.WithBaseURL(baseURL))
		// A unified gateway expects Bearer auth, not Anthropic's native X-API-Key.
		opts = append(opts, cometsdk.WithBearerAuth())
		t.Logf("using custom base URL: %s (Bearer auth)", baseURL)
	}

	return anthropic.NewAnthropicProvider(apiKey, opts...)
}

func TestLive_Anthropic_TextStream(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:  "claude-sonnet-4-5",
		System: "You are a concise assistant. Reply in one sentence.",
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is the capital of France?"}},
			},
		},
		MaxTokens: 64,
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	var text string
	var gotStep, gotDone bool

	for e := range ch {
		switch ev := e.(type) {
		case cometsdk.TextDeltaEvent:
			text += ev.Text
		case cometsdk.StepFinishEvent:
			gotStep = true
			t.Logf("finish_reason=%s input=%d output=%d",
				ev.FinishReason, ev.Usage.InputTokens, ev.Usage.OutputTokens)
		case cometsdk.DoneEvent:
			gotDone = true
		case cometsdk.ErrorEvent:
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}

	t.Logf("response: %s", text)
	require.NotEmpty(t, text)
	require.True(t, gotStep)
	require.True(t, gotDone)
}

func TestLive_Anthropic_ToolCall(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:  "claude-sonnet-4-5",
		System: "Use the get_weather tool to answer weather questions.",
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What's the weather in Taipei?"}},
			},
		},
		Tools: []cometsdk.Tool{
			{
				Name:        "get_weather",
				Description: "Get current weather for a city",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"city": {"type": "string", "description": "City name"}
					},
					"required": ["city"]
				}`),
			},
		},
		MaxTokens: 256,
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	var gotToolStart, gotToolDone bool

	for e := range ch {
		switch ev := e.(type) {
		case cometsdk.ToolCallStartEvent:
			gotToolStart = true
			t.Logf("tool start: id=%s name=%s", ev.ID, ev.Name)
		case cometsdk.ToolCallDoneEvent:
			gotToolDone = true
			t.Logf("tool done: id=%s name=%s input=%s", ev.ID, ev.Name, ev.Input)
			require.Equal(t, "get_weather", ev.Name)
			require.True(t, json.Valid(ev.Input))
		case cometsdk.ErrorEvent:
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}

	require.True(t, gotToolStart)
	require.True(t, gotToolDone)
}

func TestLive_Anthropic_ContextCancel(t *testing.T) {
	p := newLiveProvider(t)

	ctx, cancel := context.WithCancel(context.Background())

	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Count slowly from 1 to 100, one number per line."}},
			},
		},
		MaxTokens: 512,
	}

	ch, err := p.Stream(ctx, req)
	require.NoError(t, err)

	count := 0
	for e := range ch {
		if _, ok := e.(cometsdk.TextDeltaEvent); ok {
			count++
			if count == 3 {
				cancel() // cancel mid-stream
			}
		}
	}

	// Channel must be closed cleanly — no goroutine leak.
	t.Logf("received %d text deltas before cancel", count)
}
