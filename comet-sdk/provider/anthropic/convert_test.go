package anthropic

import (
	"encoding/json"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

// ─── Request conversion tests ─────────────────────────────────────────────────

func TestConvertRequest_Basic(t *testing.T) {
	req := &cometsdk.Request{
		Model:  "claude-sonnet-4-5",
		System: "You are helpful.",
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hello"}},
			},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out anthropicRequest
	require.NoError(t, json.Unmarshal(data, &out))

	require.Equal(t, "claude-sonnet-4-5", out.Model)
	require.Equal(t, "You are helpful.", out.System)
	require.True(t, out.Stream)
	require.Equal(t, 4096, out.MaxTokens) // default
	require.Len(t, out.Messages, 1)
	require.Equal(t, "user", out.Messages[0].Role)
	require.Len(t, out.Messages[0].Content, 1)
	require.Equal(t, "text", out.Messages[0].Content[0].Type)
	require.Equal(t, "Hello", out.Messages[0].Content[0].Text)
}

func TestConvertRequest_ToolCall(t *testing.T) {
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{
				Role: cometsdk.RoleAssistant,
				Content: []cometsdk.Block{
					cometsdk.ToolCallBlock{
						ID:    "toolu_01",
						Name:  "read_file",
						Input: json.RawMessage(`{"path":"main.go"}`),
					},
				},
			},
		},
		Tools: []cometsdk.Tool{
			{
				Name:        "read_file",
				Description: "Read a file",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out anthropicRequest
	require.NoError(t, json.Unmarshal(data, &out))

	require.Len(t, out.Tools, 1)
	require.Equal(t, "read_file", out.Tools[0].Name)

	require.Len(t, out.Messages, 1)
	block := out.Messages[0].Content[0]
	require.Equal(t, "tool_use", block.Type)
	require.Equal(t, "toolu_01", block.ID)
	require.Equal(t, "read_file", block.Name)
}

func TestConvertRequest_EmptyContentFiltered(t *testing.T) {
	// Anthropic rejects messages with empty content arrays.
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{}}, // empty — should be filtered
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out anthropicRequest
	require.NoError(t, json.Unmarshal(data, &out))

	require.Len(t, out.Messages, 1)
	require.Equal(t, "Hi", out.Messages[0].Content[0].Text)
}

func TestConvertRequest_UserImageBlock(t *testing.T) {
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{
				Role: cometsdk.RoleUser,
				Content: []cometsdk.Block{
					cometsdk.TextBlock{Text: "What is in this image?"},
					cometsdk.ImageBlock{MediaType: "image/png", Data: "aGVsbG8="},
				},
			},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out anthropicRequest
	require.NoError(t, json.Unmarshal(data, &out))
	require.Len(t, out.Messages, 1)
	require.Len(t, out.Messages[0].Content, 2)
	image := out.Messages[0].Content[1]
	require.Equal(t, "image", image.Type)
	require.NotNil(t, image.Source)
	require.Equal(t, "base64", image.Source.Type)
	require.Equal(t, "image/png", image.Source.MediaType)
	require.Equal(t, "aGVsbG8=", image.Source.Data)
}

func TestConvertRequest_ToolCallIDSanitised(t *testing.T) {
	// Tool call IDs with invalid chars should be sanitised.
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{
				Role: cometsdk.RoleToolResult,
				Content: []cometsdk.Block{
					cometsdk.ToolResultBlock{
						ToolCallID: "call.with.dots",
						Content:    "result",
					},
				},
			},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out anthropicRequest
	require.NoError(t, json.Unmarshal(data, &out))

	require.Equal(t, "call_with_dots", out.Messages[0].Content[0].ToolUseID)
}

// ─── Event conversion tests ───────────────────────────────────────────────────

func TestConvertEvent_TextDelta(t *testing.T) {
	state := newStreamState()
	events, err := toSDKEvents("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		state)
	require.NoError(t, err)
	require.Len(t, events, 1)
	td, ok := events[0].(cometsdk.TextDeltaEvent)
	require.True(t, ok)
	require.Equal(t, "Hello", td.Text)
}

func TestConvertEvent_ToolCallDone(t *testing.T) {
	state := newStreamState()

	// Simulate: content_block_start (tool_use) → delta × 2 → content_block_stop
	_, err := toSDKEvents("content_block_start",
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_01","name":"read_file"}}`,
		state)
	require.NoError(t, err)

	_, err = toSDKEvents("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"path\":"}}`,
		state)
	require.NoError(t, err)

	_, err = toSDKEvents("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"main.go\"}"}}`,
		state)
	require.NoError(t, err)

	events, err := toSDKEvents("content_block_stop",
		`{"type":"content_block_stop","index":0}`,
		state)
	require.NoError(t, err)
	require.Len(t, events, 1)

	done, ok := events[0].(cometsdk.ToolCallDoneEvent)
	require.True(t, ok)
	require.Equal(t, "toolu_01", done.ID)
	require.Equal(t, "read_file", done.Name)
	require.JSONEq(t, `{"path":"main.go"}`, string(done.Input))
}

func TestConvertEvent_StepFinish(t *testing.T) {
	state := newStreamState()
	events, err := toSDKEvents("message_delta",
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":2,"cache_creation_input_tokens":1}}`,
		state)
	require.NoError(t, err)
	require.Len(t, events, 1)

	sf, ok := events[0].(cometsdk.StepFinishEvent)
	require.True(t, ok)
	require.Equal(t, "stop", sf.FinishReason)
	require.Equal(t, 10, sf.Usage.InputTokens)
	require.Equal(t, 5, sf.Usage.OutputTokens)
	require.Equal(t, 2, sf.Usage.CacheRead)
	require.Equal(t, 1, sf.Usage.CacheWrite)
}

// ─── Options passthrough tests ────────────────────────────────────────────────

func TestConvertRequest_OptionsPassthrough(t *testing.T) {
	// top_k, top_p, thinking are Anthropic-specific params.
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}},
		},
		Options: map[string]any{
			"anthropic": map[string]any{
				"top_k": 40,
				"top_p": 0.9,
				"thinking": map[string]any{
					"type":          "enabled",
					"budget_tokens": 5000,
				},
			},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	require.Equal(t, float64(40), out["top_k"])
	require.Equal(t, 0.9, out["top_p"])
	require.NotNil(t, out["thinking"])

	// SDK-managed fields must not be overridden.
	require.Equal(t, "claude-sonnet-4-5", out["model"])
	require.Equal(t, true, out["stream"])
}

func TestConvertRequest_OptionsDoNotOverrideSDKFields(t *testing.T) {
	// Caller tries to override model and stream — SDK fields must win.
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}},
		},
		Options: map[string]any{
			"anthropic": map[string]any{
				"model":  "claude-haiku", // should be ignored
				"stream": false,          // should be ignored
			},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	require.Equal(t, "claude-sonnet-4-5", out["model"])
	require.Equal(t, true, out["stream"])
}

func TestConvertRequest_OptionsNil(t *testing.T) {
	// No Options set — should behave identically to before.
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}},
		},
	}

	data, err := toAnthropicRequest(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))

	require.Equal(t, "claude-sonnet-4-5", out["model"])
	require.Equal(t, true, out["stream"])
}

func TestConvertRequest_OptionsWrongType(t *testing.T) {
	// Options["anthropic"] is not map[string]any — should return an error.
	req := &cometsdk.Request{
		Model: "claude-sonnet-4-5",
		Messages: []cometsdk.Message{
			{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hi"}}},
		},
		Options: map[string]any{
			"anthropic": "this-is-wrong",
		},
	}

	_, err := toAnthropicRequest(req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "map[string]any")
}
