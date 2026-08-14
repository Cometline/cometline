package llm

import (
	"context"
	"sync"

	cometsdk "github.com/cometline/comet-sdk"
)

// ─── MessageStream ────────────────────────────────────────────────────────────

// MessageStream provides real-time access to all streamed events while
// automatically assembling the final [GenerateMessageResult].
//
// MessageStream forwards all event types (text deltas, tool call
// start/delta/done, step finish, reasoning events) so the caller can render
// them in real time. This is the recommended streaming API for agent-style
// interactions where the UI needs to show tool calls as they happen.
//
// Usage:
//
//	stream := llm.StreamMessage(ctx, provider, req)
//	for ev := range stream.Events() {
//	    switch e := ev.(type) {
//	    case cometsdk.TextDeltaEvent:
//	        fmt.Print(e.Text)
//	    case cometsdk.ToolCallStartEvent:
//	        fmt.Printf("calling %s...\n", e.Name)
//	    case cometsdk.ToolCallDoneEvent:
//	        fmt.Printf("tool %s done\n", e.Name)
//	    case cometsdk.ReasoningContentEvent:
//	        fmt.Print("[reasoning]", e.Text)
//	    }
//	}
//	result, err := stream.Result()
//
// The caller MUST drain Events() before calling Result().
type MessageStream struct {
	events <-chan cometsdk.Event
	done   <-chan struct{}

	mu     sync.Mutex
	result *GenerateMessageResult
	err    error
}

// StreamMessage starts a streaming LLM call and returns a [MessageStream].
//
// The returned MessageStream immediately begins receiving events from the
// provider in a background goroutine. The caller reads events from
// [MessageStream.Events] and retrieves the final assembled result from
// [MessageStream.Result].
//
// StreamMessage does NOT execute tools. Tool calls are forwarded as events
// and included in the final [GenerateMessageResult.ToolCalls].
//
// This is the function CometMind's agent loop should use: stream events
// to the TUI/SSE layer for real-time rendering, then use Result() to get
// the assembled message for persistence and loop decisions.
func StreamMessage(ctx context.Context, p cometsdk.Provider, req *cometsdk.Request) *MessageStream {
	ch, err := p.Stream(ctx, req)

	// Buffer size balances latency vs memory. 32 is generous for typical
	// LLM streaming rates (tens to low hundreds of events per second).
	events := make(chan cometsdk.Event, 32)
	done := make(chan struct{})

	ms := &MessageStream{
		events: events,
		done:   done,
	}

	if err != nil {
		ms.err = err
		close(events)
		close(done)
		return ms
	}

	go ms.run(ctx, ch, events, done)
	return ms
}

// Events returns a channel of [cometsdk.Event]. The channel emits the same
// event types as [cometsdk.Provider.Stream] — TextDeltaEvent,
// ToolCallStartEvent, ToolCallDeltaEvent, ToolCallDoneEvent,
// StepFinishEvent, ReasoningStartEvent, and ReasoningContentEvent.
//
// ErrorEvent and DoneEvent are NOT forwarded to the caller. Errors are
// reported via [Result] and the channel is simply closed when the stream ends.
//
// The channel is closed when the stream ends (success, error, or cancellation).
// The caller must drain this channel before calling [Result].
func (ms *MessageStream) Events() <-chan cometsdk.Event {
	return ms.events
}

// Result blocks until the stream is fully consumed and returns the assembled
// [GenerateMessageResult]. The caller MUST drain [Events] before calling
// Result, otherwise Result will deadlock.
//
// If a streaming or pre-stream error occurred, it is returned here. For a
// mid-stream error, the result is non-nil and contains the output received
// before the failure; pre-stream errors still return a nil result.
func (ms *MessageStream) Result() (*GenerateMessageResult, error) {
	<-ms.done

	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.result, ms.err
}

func (ms *MessageStream) run(ctx context.Context, ch <-chan cometsdk.Event, events chan<- cometsdk.Event, done chan<- struct{}) {
	defer close(events)
	defer close(done)

	var (
		textBuf       []byte
		reasoningBuf  []byte
		toolCalls     []cometsdk.ToolCallBlock
		providerState []cometsdk.ProviderState
		finish        string
		usage         cometsdk.TokenUsage
	)

	for {
		select {
		case <-ctx.Done():
			go func() {
				for range ch {
				}
			}()
			ms.setPartialResult(textBuf, reasoningBuf, toolCalls, providerState, finish, usage, ctx.Err())
			return

		case ev, ok := <-ch:
			if !ok {
				ms.setFinalMessage(textBuf, reasoningBuf, toolCalls, providerState, finish, usage)
				return
			}

			switch e := ev.(type) {
			case cometsdk.TextDeltaEvent:
				textBuf = append(textBuf, e.Text...)
				events <- e

			case cometsdk.ToolCallStartEvent:
				events <- e

			case cometsdk.ToolCallDeltaEvent:
				events <- e

			case cometsdk.ToolCallDoneEvent:
				tc := cometsdk.ToolCallBlock{
					ID:    e.ID,
					Name:  e.Name,
					Input: e.Input,
				}
				toolCalls = append(toolCalls, tc)
				events <- e

			case cometsdk.StepFinishEvent:
				finish = e.FinishReason
				usage = e.Usage
				events <- e

			case cometsdk.ReasoningStartEvent:
				events <- e

			case cometsdk.ReasoningContentEvent:
				reasoningBuf = append(reasoningBuf, e.Text...)
				events <- e

			case cometsdk.ProviderStateEvent:
				providerState = append(providerState, e.State)

			case cometsdk.ErrorEvent:
				// Do NOT forward — caller gets the error from Result(). Preserve
				// everything emitted before the stream failed so agent callers can
				// persist a useful partial response.
				ms.setPartialResult(textBuf, reasoningBuf, toolCalls, providerState, finish, usage, e.Err)
				return

			case cometsdk.DoneEvent:
				// Do NOT forward — caller sees channel close instead.
				ms.setFinalMessage(textBuf, reasoningBuf, toolCalls, providerState, finish, usage)
				return
			}
		}
	}
}

func (ms *MessageStream) setFinalMessage(textBuf, reasoningBuf []byte, toolCalls []cometsdk.ToolCallBlock, providerState []cometsdk.ProviderState, finish string, usage cometsdk.TokenUsage) {
	ms.setResult(buildMessageResult(textBuf, reasoningBuf, toolCalls, providerState, finish, usage), nil)
}

func (ms *MessageStream) setPartialResult(textBuf, reasoningBuf []byte, toolCalls []cometsdk.ToolCallBlock, providerState []cometsdk.ProviderState, finish string, usage cometsdk.TokenUsage, err error) {
	ms.setResult(buildMessageResult(textBuf, reasoningBuf, toolCalls, providerState, finish, usage), err)
}

func buildMessageResult(textBuf, reasoningBuf []byte, toolCalls []cometsdk.ToolCallBlock, providerState []cometsdk.ProviderState, finish string, usage cometsdk.TokenUsage) *GenerateMessageResult {
	text := string(textBuf)
	reasoning := string(reasoningBuf)

	return &GenerateMessageResult{
		Message:      buildAssistantMessage(text, reasoning, toolCalls, providerState),
		ToolCalls:    toolCalls,
		FinishReason: finish,
		Usage:        usage,
	}
}

func (ms *MessageStream) setResult(r *GenerateMessageResult, err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.result = r
	ms.err = err
}
