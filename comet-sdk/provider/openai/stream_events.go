package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

// ─── Incoming: OpenAI SSE → SDK Events ───────────────────────────────────────

type openAIReasoningDetail struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// openAIDelta is the JSON structure of each OpenAI SSE data line.
type openAIDelta struct {
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Role             string                  `json:"role"`
			Content          string                  `json:"content"`
			Reasoning        string                  `json:"reasoning"`
			ReasoningContent string                  `json:"reasoning_content"`
			ReasoningDetails []openAIReasoningDetail `json:"reasoning_details"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage"`
}

type openAIUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails *struct {
		CachedTokens        int `json:"cached_tokens"`
		CacheWriteTokens    int `json:"cache_write_tokens"`
		CacheCreationTokens int `json:"cache_creation_tokens"`
	} `json:"prompt_tokens_details"`
}

func tokenUsageFrom(u *openAIUsage) cometsdk.TokenUsage {
	if u == nil {
		return cometsdk.TokenUsage{}
	}
	usage := cometsdk.TokenUsage{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
	}
	if d := u.PromptTokensDetails; d != nil {
		usage.CacheRead = d.CachedTokens
		if d.CacheWriteTokens > 0 {
			usage.CacheWrite = d.CacheWriteTokens
		} else {
			usage.CacheWrite = d.CacheCreationTokens
		}
	}
	return usage
}

func applyUsage(state *streamState, u *openAIUsage) {
	if u == nil {
		return
	}
	state.pendingUsage = tokenUsageFrom(u)
	if state.pendingFinish != nil {
		state.pendingFinish.Usage = state.pendingUsage
	}
}

// inProgressReasoning tracks reasoning content being streamed.
type inProgressReasoning struct {
	buffer strings.Builder
}

// inProgressToolCall tracks a tool call being assembled from streaming deltas.
type inProgressToolCall struct {
	id        string
	name      string
	argBuffer strings.Builder
}

// streamState maintains per-stream mutable state for the OpenAI parser.
type streamState struct {
	inProgress          map[int]*inProgressToolCall
	reasoning           *inProgressReasoning
	contentReasoning    contentReasoningSplitter
	reasoningDetailText map[int]string
	pendingUsage        cometsdk.TokenUsage
	pendingFinish       *cometsdk.StepFinishEvent
}

func newStreamState() *streamState {
	return &streamState{
		inProgress:          make(map[int]*inProgressToolCall),
		reasoningDetailText: make(map[int]string),
	}
}

// flushPendingStreamEvents emits a buffered step finish and terminal DoneEvent
// when the provider closes the SSE body without sending data: [DONE].
func flushPendingStreamEvents(state *streamState) []cometsdk.Event {
	var events []cometsdk.Event
	if state.pendingFinish != nil {
		events = append(events, *state.pendingFinish)
		state.pendingFinish = nil
	}
	events = append(events, cometsdk.DoneEvent{})
	return events
}

// toSDKEvents converts one OpenAI SSE data payload to zero or more SDK events.
func toSDKEvents(data string, state *streamState) ([]cometsdk.Event, error) {
	if data == "[DONE]" {
		var events []cometsdk.Event
		if state.pendingFinish != nil {
			events = append(events, *state.pendingFinish)
			state.pendingFinish = nil
		}
		events = append(events, cometsdk.DoneEvent{})
		return events, nil
	}

	var delta openAIDelta
	if err := json.Unmarshal([]byte(data), &delta); err != nil {
		return nil, fmt.Errorf("openai: parse delta: %w", err)
	}

	// Official OpenAI usage chunks use choices:[]; some gateways (TrendAI)
	// send usage on a later chunk that still has a non-empty empty-delta choice.
	applyUsage(state, delta.Usage)
	if len(delta.Choices) == 0 {
		return nil, nil
	}

	choice := delta.Choices[0]
	var events []cometsdk.Event

	// Reasoning content delta. Some OpenAI-compatible providers use
	// `reasoning_content` instead of OpenAI's `reasoning` field name.
	reasoning := choice.Delta.Reasoning
	if reasoning == "" {
		reasoning = choice.Delta.ReasoningContent
	}
	if reasoning != "" {
		if state.reasoning == nil {
			state.reasoning = &inProgressReasoning{}
			events = append(events, cometsdk.ReasoningStartEvent{})
		}
		state.reasoning.buffer.WriteString(reasoning)
		events = append(events, cometsdk.ReasoningContentEvent{
			Text: reasoning,
		})
	} else {
		for _, detail := range choice.Delta.ReasoningDetails {
			if detail.Text == "" {
				continue
			}
			delta := reasoningDetailsDelta(state.reasoningDetailText[detail.Index], detail.Text)
			state.reasoningDetailText[detail.Index] = detail.Text
			if delta == "" {
				continue
			}
			if state.reasoning == nil {
				state.reasoning = &inProgressReasoning{}
				events = append(events, cometsdk.ReasoningStartEvent{})
			}
			state.reasoning.buffer.WriteString(delta)
			events = append(events, cometsdk.ReasoningContentEvent{Text: delta})
		}
	}

	// Text delta. Some providers embed thinking in content tags when
	// reasoning_split is disabled; split those out before emitting text.
	if choice.Delta.Content != "" {
		events = append(events, state.contentReasoning.push(choice.Delta.Content)...)
	}

	// Tool call deltas.
	for _, tc := range choice.Delta.ToolCalls {
		idx := tc.Index
		if tc.Function.Name != "" {
			state.inProgress[idx] = &inProgressToolCall{
				id:   tc.ID,
				name: tc.Function.Name,
			}
			events = append(events, cometsdk.ToolCallStartEvent{
				ID:   tc.ID,
				Name: tc.Function.Name,
			})
		}
		if tc.Function.Arguments != "" {
			ip, ok := state.inProgress[idx]
			if ok {
				ip.argBuffer.WriteString(tc.Function.Arguments)
				events = append(events, cometsdk.ToolCallDeltaEvent{
					ID:    ip.id,
					Delta: tc.Function.Arguments,
				})
			}
		}
	}

	if choice.FinishReason != "" {
		// Flush any in-progress tool calls before recording the finish so the
		// caller sees complete tool calls ahead of the StepFinishEvent.
		if choice.FinishReason == "tool_calls" {
			for _, ip := range state.inProgress {
				events = append(events, cometsdk.ToolCallDoneEvent{
					ID:    ip.id,
					Name:  ip.name,
					Input: json.RawMessage(ip.argBuffer.String()),
				})
			}
			state.inProgress = make(map[int]*inProgressToolCall)
		}
		state.pendingFinish = &cometsdk.StepFinishEvent{
			FinishReason: cometsdk.NormalizeFinishReason(choice.FinishReason),
			Usage:        state.pendingUsage,
		}
	}

	return events, nil
}
