//go:build live

package llm_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/comet-sdk/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLiveProvider resolves the API key and optional base URL from the environment.
//
// Key resolution order:
//  1. CUSTOM_API_KEY
//  2. OPENAI_API_KEY (fallback)
//
// CUSTOM_BASE_URL overrides the default API endpoint.
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

func liveModel() string {
	if m := os.Getenv("LIVE_TEST_MODEL"); m != "" {
		return m
	}
	return "gpt-4o"
}

// ─── Collect ──────────────────────────────────────────────────────────────────

func TestLive_Collect_TextOnly(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is 2 + 2? Reply with just the number."}},
			},
		},
	}

	resp, err := llm.Collect(context.Background(), p, req)
	require.NoError(t, err)

	t.Logf("text: %q", resp.Text)
	t.Logf("finish_reason=%s input=%d output=%d", resp.FinishReason, resp.Usage.InputTokens, resp.Usage.OutputTokens)

	assert.NotEmpty(t, resp.Text)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Empty(t, resp.ToolCalls)
	assert.Equal(t, cometsdk.RoleAssistant, resp.Message.Role)
	// Note: some OpenAI-compatible gateways return zero usage in streaming mode.
	t.Logf("usage: input=%d output=%d", resp.Usage.InputTokens, resp.Usage.OutputTokens)
}

func TestLive_Collect_WithToolCall(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
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

	resp, err := llm.Collect(context.Background(), p, req)
	require.NoError(t, err)

	t.Logf("finish_reason=%s tool_calls=%d", resp.FinishReason, len(resp.ToolCalls))

	require.NotEmpty(t, resp.ToolCalls)
	assert.Equal(t, "get_weather", resp.ToolCalls[0].Name)
	assert.True(t, json.Valid(resp.ToolCalls[0].Input))
	t.Logf("tool input: %s", resp.ToolCalls[0].Input)
}

// ─── GenerateText ─────────────────────────────────────────────────────────────

func TestLive_GenerateText(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is the capital of France? Reply in one word."}},
			},
		},
	}

	result, err := llm.GenerateText(context.Background(), p, req)
	require.NoError(t, err)

	t.Logf("text: %q", result.Text)
	assert.NotEmpty(t, result.Text)
	assert.Equal(t, "stop", result.FinishReason)
	// Note: some OpenAI-compatible gateways return zero usage in streaming mode.
	t.Logf("usage: input=%d output=%d", result.Usage.InputTokens, result.Usage.OutputTokens)
}

// ─── GenerateMessage ──────────────────────────────────────────────────────────

func TestLive_GenerateMessage_TextOnly(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Say hello in Japanese. One sentence only."}},
			},
		},
	}

	result, err := llm.GenerateMessage(context.Background(), p, req)
	require.NoError(t, err)

	t.Logf("text in message: %v", result.Message.Content)
	assert.Equal(t, cometsdk.RoleAssistant, result.Message.Role)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Empty(t, result.ToolCalls)
	require.NotEmpty(t, result.Message.Content)
}

func TestLive_GenerateMessage_WithToolCall(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
		MaxTokens: 256,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Read the file main.go"}},
			},
		},
		Tools: []cometsdk.Tool{
			{
				Name:        "read_file",
				Description: "Read the contents of a file",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "File path to read"}
					},
					"required": ["path"]
				}`),
			},
		},
	}

	result, err := llm.GenerateMessage(context.Background(), p, req)
	require.NoError(t, err)

	t.Logf("finish_reason=%s tool_calls=%d", result.FinishReason, len(result.ToolCalls))
	require.NotEmpty(t, result.ToolCalls)
	assert.Equal(t, "read_file", result.ToolCalls[0].Name)
	t.Logf("tool input: %s", result.ToolCalls[0].Input)
}

// ─── QuickText ────────────────────────────────────────────────────────────────

func TestLive_QuickText(t *testing.T) {
	p := newLiveProvider(t)

	text, err := llm.QuickText(context.Background(), p, liveModel(), "What is 1 + 1? Reply with just the number.")
	require.NoError(t, err)

	t.Logf("text: %q", text)
	assert.NotEmpty(t, text)
}

// ─── StreamMessage ────────────────────────────────────────────────────────────

func TestLive_StreamMessage_TextOnly(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
		MaxTokens: 64,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What is the capital of Japan? One word."}},
			},
		},
	}

	stream := llm.StreamMessage(context.Background(), p, req)

	var textDeltas, stepFinishes int
	for ev := range stream.Events() {
		switch ev.(type) {
		case cometsdk.TextDeltaEvent:
			textDeltas++
		case cometsdk.StepFinishEvent:
			stepFinishes++
		}
	}

	t.Logf("text_deltas=%d step_finishes=%d", textDeltas, stepFinishes)
	assert.Greater(t, textDeltas, 0)
	assert.Equal(t, 1, stepFinishes)

	result, err := stream.Result()
	require.NoError(t, err)
	assert.Equal(t, "stop", result.FinishReason)
	assert.Equal(t, cometsdk.RoleAssistant, result.Message.Role)
	assert.Empty(t, result.ToolCalls)
	t.Logf("final text: %v", result.Message.Content)
}

func TestLive_StreamMessage_WithToolCall(t *testing.T) {
	p := newLiveProvider(t)

	req := &cometsdk.Request{
		Model:     liveModel(),
		MaxTokens: 256,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "What's the weather in Tokyo?"}},
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

	stream := llm.StreamMessage(context.Background(), p, req)

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

	t.Logf("event sequence: %v", eventTypes)

	result, err := stream.Result()
	require.NoError(t, err)

	require.NotEmpty(t, result.ToolCalls)
	assert.Equal(t, "get_weather", result.ToolCalls[0].Name)
	assert.True(t, json.Valid(result.ToolCalls[0].Input))
	t.Logf("finish_reason=%s tool_input=%s", result.FinishReason, result.ToolCalls[0].Input)
}
