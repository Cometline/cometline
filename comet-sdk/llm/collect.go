// Package llm provides higher-level convenience functions on top of the core
// [cometsdk.Provider] streaming interface.
//
// The core SDK exposes a low-level event channel ([cometsdk.Provider.Stream]).
// This package assembles those events into canonical single-turn results so
// callers don't need to write their own event-collection loops.
//
// Design boundary: this package handles single-turn model interaction only.
// It does not execute tools, manage sessions, maintain memory, or run agent
// loops — those responsibilities belong to higher layers (e.g. CometMind).
package llm

import (
	"context"
	"errors"

	cometsdk "github.com/cometline/comet-sdk"
)

// CollectedResponse is the assembled result of a single LLM call.
// It collects all streamed events into a final, easy-to-consume value.
type CollectedResponse struct {
	// Text is the concatenated text output from all TextDeltaEvents.
	// Empty if the model only emitted tool calls.
	Text string

	// ReasoningText is the concatenated reasoning / chain-of-thought output
	// from all ReasoningContentEvents. Empty if the model did not emit
	// any reasoning content.
	ReasoningText string

	// Message is the fully assembled assistant message containing all
	// content blocks (text, tool calls, and/or reasoning) in the order
	// they were emitted.
	Message cometsdk.Message

	// ToolCalls contains every tool call the model emitted, extracted from the
	// message for convenience. Callers can use this directly without walking
	// Message.Content. Empty if the model did not request any tool calls.
	ToolCalls []cometsdk.ToolCallBlock

	// FinishReason is the reason the model stopped generating.
	// Common values: "stop", "tool_use", "max_tokens".
	FinishReason string

	// Usage reports the token consumption for this call.
	Usage cometsdk.TokenUsage
}

// Collect calls [cometsdk.Provider.Stream] and drains the returned event
// channel, assembling all events into a [CollectedResponse].
//
// This is the foundational primitive in the llm package. Both [GenerateText]
// and [GenerateMessage] are built on top of it.
//
// Collect does not execute tool calls. If the model emits tool calls, they
// are available in CollectedResponse.ToolCalls and
// CollectedResponse.Message.Content — the caller decides what to do next.
//
// Errors are returned if:
//   - Provider.Stream returns a pre-stream error (e.g. auth failure, retries exhausted)
//   - An [cometsdk.ErrorEvent] is received during streaming
//   - The context is cancelled
func Collect(ctx context.Context, p cometsdk.Provider, req *cometsdk.Request) (*CollectedResponse, error) {
	ch, err := p.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	return collectFromChannel(ctx, ch)
}

// collectFromChannel drains an event channel into a CollectedResponse.
// Separated from Collect for testability.
func collectFromChannel(ctx context.Context, ch <-chan cometsdk.Event) (*CollectedResponse, error) {
	var (
		textBuf      []byte
		reasoningBuf []byte
		toolCalls    []cometsdk.ToolCallBlock
		finish       string
		usage        cometsdk.TokenUsage
		streamErr    error
	)

	for {
		select {
		case <-ctx.Done():
			// Drain remaining events to avoid goroutine leak in provider.
			go func() {
				for range ch {
				}
			}()
			return nil, ctx.Err()

		case ev, ok := <-ch:
			if !ok {
				// Channel closed without DoneEvent — unusual but handle gracefully.
				return buildResponse(textBuf, reasoningBuf, toolCalls, finish, usage), nil
			}

			switch e := ev.(type) {
			case cometsdk.TextDeltaEvent:
				textBuf = append(textBuf, e.Text...)

			case cometsdk.ToolCallStartEvent:
				// Nothing to collect yet; wait for ToolCallDoneEvent.

			case cometsdk.ToolCallDeltaEvent:
				// Intermediate delta; the provider assembles the complete input
				// and emits ToolCallDoneEvent when ready.

			case cometsdk.ToolCallDoneEvent:
				tc := cometsdk.ToolCallBlock{
					ID:    e.ID,
					Name:  e.Name,
					Input: e.Input,
				}
				toolCalls = append(toolCalls, tc)

			case cometsdk.StepFinishEvent:
				finish = e.FinishReason
				usage = e.Usage

			case cometsdk.ReasoningStartEvent:
				// Reasoning content will arrive via ReasoningContentEvents.

			case cometsdk.ReasoningContentEvent:
				reasoningBuf = append(reasoningBuf, e.Text...)

			case cometsdk.ErrorEvent:
				streamErr = e.Err

			case cometsdk.DoneEvent:
				if streamErr != nil {
					return nil, streamErr
				}
				return buildResponse(textBuf, reasoningBuf, toolCalls, finish, usage), nil
			}
		}
	}
}

// buildResponse assembles the final CollectedResponse from accumulated state.
func buildResponse(
	textBuf []byte,
	reasoningBuf []byte,
	toolCalls []cometsdk.ToolCallBlock,
	finish string,
	usage cometsdk.TokenUsage,
) *CollectedResponse {
	var blocks []cometsdk.Block

	// Add text block if there is any text content.
	text := string(textBuf)
	if len(text) > 0 {
		blocks = append(blocks, cometsdk.TextBlock{Text: text})
	}

	// Add tool call blocks in order.
	for _, tc := range toolCalls {
		blocks = append(blocks, tc)
	}

	// Add reasoning blocks if there is any reasoning content.
	var reasoningBlocks []cometsdk.Block
	reasoning := string(reasoningBuf)
	if len(reasoning) > 0 {
		reasoningBlocks = append(reasoningBlocks, cometsdk.ReasoningBlock{Text: reasoning})
	}

	return &CollectedResponse{
		Text:          text,
		ReasoningText: reasoning,
		Message: cometsdk.Message{
			Role:             cometsdk.RoleAssistant,
			Content:          blocks,
			ReasoningContent: reasoningBlocks,
		},
		ToolCalls:    toolCalls,
		FinishReason: finish,
		Usage:        usage,
	}
}

// ErrUnexpectedToolCall is returned by [GenerateText] when the model emits
// tool calls instead of (or in addition to) text. Use [GenerateMessage] or
// [Collect] if you need to handle tool calls.
var ErrUnexpectedToolCall = errors.New("cometsdk/llm: model returned tool calls; use GenerateMessage or Collect to handle them")
