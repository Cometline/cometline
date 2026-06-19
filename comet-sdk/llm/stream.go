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
// If a streaming or pre-stream error occurred, it is returned here.
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
			go func() {
				for range ch {
				}
			}()
			ms.setResult(nil, ctx.Err())
			return

		case ev, ok := <-ch:
			if !ok {
				ms.setFinalMessage(textBuf, reasoningBuf, toolCalls, finish, usage)
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

			case cometsdk.ErrorEvent:
				streamErr = e.Err
				// Do NOT forward — caller gets the error from Result().

			case cometsdk.DoneEvent:
				// Do NOT forward — caller sees channel close instead.
				if streamErr != nil {
					ms.setResult(nil, streamErr)
					return
				}
				ms.setFinalMessage(textBuf, reasoningBuf, toolCalls, finish, usage)
				return
			}
		}
	}
}

func (ms *MessageStream) setFinalMessage(textBuf, reasoningBuf []byte, toolCalls []cometsdk.ToolCallBlock, finish string, usage cometsdk.TokenUsage) {
	var blocks []cometsdk.Block

	text := string(textBuf)
	if len(text) > 0 {
		blocks = append(blocks, cometsdk.TextBlock{Text: text})
	}
	for _, tc := range toolCalls {
		blocks = append(blocks, tc)
	}

	var reasoningBlocks []cometsdk.Block
	reasoning := string(reasoningBuf)
	if len(reasoning) > 0 {
		reasoningBlocks = append(reasoningBlocks, cometsdk.ReasoningBlock{Text: reasoning})
	}

	ms.setResult(&GenerateMessageResult{
		Message: cometsdk.Message{
			Role:             cometsdk.RoleAssistant,
			Content:          blocks,
			ReasoningContent: reasoningBlocks,
		},
		ToolCalls:    toolCalls,
		FinishReason: finish,
		Usage:        usage,
	}, nil)
}

func (ms *MessageStream) setResult(r *GenerateMessageResult, err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.result = r
	ms.err = err
}
