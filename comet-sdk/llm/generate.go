package llm

import (
	"context"
	"fmt"

	cometsdk "github.com/cometline/comet-sdk"
)

// ─── GenerateText ─────────────────────────────────────────────────────────────

// GenerateTextResult is the result of a pure text generation call.
type GenerateTextResult struct {
	// Text is the complete text response from the model.
	Text string

	// FinishReason is the reason the model stopped ("stop", "max_tokens", etc.).
	FinishReason string

	// Usage reports token consumption for this call.
	Usage cometsdk.TokenUsage
}

// GenerateText sends a request and returns the assembled text response.
//
// This is the simplest way to call an LLM when you only need text output.
// It is intended for use cases without tool calling.
//
// If the model returns tool calls, GenerateText returns [ErrUnexpectedToolCall].
// Use [GenerateMessage] or [Collect] instead if tools are involved.
//
//	result, err := llm.GenerateText(ctx, provider, &cometsdk.Request{
//	    Model:     "gpt-4o",
//	    MaxTokens: 256,
//	    Messages:  []cometsdk.Message{{
//	        Role:    cometsdk.RoleUser,
//	        Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Hello"}},
//	    }},
//	})
//	fmt.Println(result.Text)
func GenerateText(ctx context.Context, p cometsdk.Provider, req *cometsdk.Request) (*GenerateTextResult, error) {
	resp, err := Collect(ctx, p, req)
	if err != nil {
		return nil, err
	}

	if len(resp.ToolCalls) > 0 {
		return nil, ErrUnexpectedToolCall
	}

	return &GenerateTextResult{
		Text:         resp.Text,
		FinishReason: resp.FinishReason,
		Usage:        resp.Usage,
	}, nil
}

// ─── GenerateMessage ──────────────────────────────────────────────────────────

// GenerateMessageResult is the result of a single model turn that may contain
// text, tool calls, or both.
type GenerateMessageResult struct {
	// Message is the fully assembled assistant message. Its Content slice may
	// contain [cometsdk.TextBlock] and/or [cometsdk.ToolCallBlock] entries.
	Message cometsdk.Message

	// ToolCalls contains every tool call the model emitted, extracted from
	// Message.Content for convenience. Empty if no tools were called.
	ToolCalls []cometsdk.ToolCallBlock

	// FinishReason is the reason the model stopped.
	// "stop" means the model finished naturally.
	// "tool_use" means the model wants tool results before continuing.
	// "max_tokens" means the output was truncated.
	FinishReason string

	// Usage reports token consumption for this call.
	Usage cometsdk.TokenUsage
}

// GenerateMessage sends a request and returns the fully assembled assistant
// message, including any tool calls the model emitted.
//
// Unlike [GenerateText], GenerateMessage does not error when the model returns
// tool calls. It is the recommended function for tool-capable interactions
// where the caller handles tool execution.
//
// GenerateMessage does NOT execute tools. Tool calls are returned in
// [GenerateMessageResult.ToolCalls] — the caller decides what to do next.
//
//	result, err := llm.GenerateMessage(ctx, provider, &cometsdk.Request{
//	    Model:     "claude-sonnet-4-5",
//	    MaxTokens: 1024,
//	    Messages:  messages,
//	    Tools:     tools,
//	})
//	if result.FinishReason == "tool_use" {
//	    for _, tc := range result.ToolCalls {
//	        // execute tool, build ToolResultBlock, append to messages, re-call
//	    }
//	}
func GenerateMessage(ctx context.Context, p cometsdk.Provider, req *cometsdk.Request) (*GenerateMessageResult, error) {
	resp, err := Collect(ctx, p, req)
	if err != nil {
		return nil, err
	}

	return &GenerateMessageResult{
		Message:      resp.Message,
		ToolCalls:    resp.ToolCalls,
		FinishReason: resp.FinishReason,
		Usage:        resp.Usage,
	}, nil
}

// ─── Quick helpers ────────────────────────────────────────────────────────────

// QuickText is a shorthand for the most common LLM call: send a single user
// message and get text back. It builds the [cometsdk.Request] for you.
//
//	text, err := llm.QuickText(ctx, provider, "gpt-4o", "What is the capital of France?")
func QuickText(ctx context.Context, p cometsdk.Provider, model string, prompt string) (string, error) {
	req := &cometsdk.Request{
		Model: model,
		Messages: []cometsdk.Message{
			{
				Role:    cometsdk.RoleUser,
				Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
			},
		},
	}

	result, err := GenerateText(ctx, p, req)
	if err != nil {
		return "", fmt.Errorf("QuickText: %w", err)
	}

	return result.Text, nil
}
