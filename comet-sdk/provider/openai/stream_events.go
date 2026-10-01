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
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage"`
}

type openAIChoice struct {
	Index        int               `json:"index"`
	FinishReason string            `json:"finish_reason"`
	Delta        openAIChoiceDelta `json:"delta"`
}

type openAIChoiceDelta struct {
	Role             string                  `json:"role"`
	Content          string                  `json:"content"`
	Reasoning        string                  `json:"reasoning"`
	ReasoningContent string                  `json:"reasoning_content"`
	ReasoningDetails []openAIReasoningDetail `json:"reasoning_details"`
	ToolCalls        []openAIToolDelta       `json:"tool_calls"`
}

type openAIToolDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
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

	return openAIChoiceEvents(delta.Choices[0], state), nil
}

func openAIChoiceEvents(choice openAIChoice, state *streamState) []cometsdk.Event {
	var events []cometsdk.Event
	events = append(events, openAIReasoningEvents(state, choice.Delta)...)
	// Text delta. Some providers embed thinking in content tags when
	// reasoning_split is disabled; split those out before emitting text.
	if choice.Delta.Content != "" {
		events = append(events, state.contentReasoning.push(choice.Delta.Content)...)
	}
	events = append(events, openAIToolEvents(state, choice.Delta.ToolCalls)...)
	if choice.FinishReason != "" {
		events = append(events, openAIFinishEvents(state, choice.FinishReason)...)
	}
	return events
}

func openAIReasoningEvents(state *streamState, delta openAIChoiceDelta) []cometsdk.Event {
	var events []cometsdk.Event
	// Reasoning content delta. Some OpenAI-compatible providers use
	// `reasoning_content` instead of OpenAI's `reasoning` field name.
	reasoning := delta.Reasoning
	if reasoning == "" {
		reasoning = delta.ReasoningContent
	}
	if reasoning != "" {
		events = append(events, appendReasoningText(state, reasoning)...)
		return events
	}
	for _, detail := range delta.ReasoningDetails {
		if detail.Text == "" {
			continue
		}
		piece := reasoningDetailsDelta(state.reasoningDetailText[detail.Index], detail.Text)
		state.reasoningDetailText[detail.Index] = detail.Text
		if piece == "" {
			continue
		}
		events = append(events, appendReasoningText(state, piece)...)
	}
	return events
}

func appendReasoningText(state *streamState, text string) []cometsdk.Event {
	var events []cometsdk.Event
	if state.reasoning == nil {
		state.reasoning = &inProgressReasoning{}
		events = append(events, cometsdk.ReasoningStartEvent{})
	}
	state.reasoning.buffer.WriteString(text)
	return append(events, cometsdk.ReasoningContentEvent{Text: text})
}

func openAIToolEvents(state *streamState, calls []openAIToolDelta) []cometsdk.Event {
	var events []cometsdk.Event
	for _, tc := range calls {
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
	return events
}

func openAIFinishEvents(state *streamState, finishReason string) []cometsdk.Event {
	var events []cometsdk.Event
	// Flush any in-progress tool calls before recording the finish so the
	// caller sees complete tool calls ahead of the StepFinishEvent.
	if finishReason == "tool_calls" {
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
		FinishReason: cometsdk.NormalizeFinishReason(finishReason),
		Usage:        state.pendingUsage,
	}
	return events
}
