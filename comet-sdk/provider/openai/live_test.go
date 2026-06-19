//go:build live

package openai_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/provider/openai"
	"github.com/stretchr/testify/require"
)

// newLiveProvider resolves the API key and optional base URL from the environment.
//
// Key resolution order:
//  1. CUSTOM_API_KEY
//  2. OPENAI_API_KEY (fallback)
//
// CUSTOM_BASE_URL overrides the default API endpoint for any provider.
// For OpenAI this replaces https://api.openai.com — useful for pointing at a
// company unified API or any other OpenAI-compatible endpoint.
func newLiveProvider(t *testing.T) cometsdk.Provider {
	t.Helper()

	apiKey := os.Getenv("CUSTOM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		t.Skip("neither CUSTOM_API_KEY nor OPENAI_API_KEY is set")
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	opts := []cometsdk.Option{cometsdk.WithLogger(log)}
	if baseURL := os.Getenv("CUSTOM_BASE_URL"); baseURL != "" {
		opts = append(opts, cometsdk.WithBaseURL(baseURL))
		t.Logf("using custom base URL: %s", baseURL)
	}

	return openai.NewOpenAIProvider(apiKey, opts...)
}

func TestLive_OpenAI_TextStream(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     "gpt-4o",
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is the capital of France?"}},
			},
		},
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

func TestLive_OpenAI_SystemPrompt(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     "gpt-4o",
		System:    "Reply only in Traditional Chinese (zh-TW). Keep answers under 20 words.",
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is the capital of France?"}},
			},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	var text string
	for e := range ch {
		switch ev := e.(type) {
		case cometsdk.TextDeltaEvent:
			text += ev.Text
		case cometsdk.ErrorEvent:
			t.Fatalf("unexpected error: %v", ev.Err)
		}
	}

	t.Logf("response: %s", text)
	require.NotEmpty(t, text)
}

func TestLive_OpenAI_ToolCall(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     "gpt-4o",
		MaxTokens: 256,
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
	}

	ch, err := p.Stream(context.Background(), req)
	require.NoError(t, err)

	var gotToolDone bool

	for e := range ch {
		switch ev := e.(type) {
		case cometsdk.ToolCallStartEvent:
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

	require.True(t, gotToolDone)
}

func TestLive_OpenAI_ContextCancel(t *testing.T) {
	p := newLiveProvider(t)

	ctx, cancel := context.WithCancel(context.Background())

	req := &cometsdk.Request{
		Model:     "gpt-4o",
		MaxTokens: 512,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Count slowly from 1 to 100, one number per line."}},
			},
		},
	}

	ch, err := p.Stream(ctx, req)
	require.NoError(t, err)

	count := 0
	for e := range ch {
		if _, ok := e.(cometsdk.TextDeltaEvent); ok {
			count++
			if count == 3 {
				cancel()
			}
		}
	}

	t.Logf("received %d text deltas before cancel", count)
}
