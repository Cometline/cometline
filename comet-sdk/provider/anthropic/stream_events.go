package anthropic

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

// ─── Incoming: Anthropic SSE → SDK Events ────────────────────────────────────

// anthropicSSEEvent is the top-level JSON structure of Anthropic SSE data payloads.
type anthropicSSEEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`

	// content_block_start
	ContentBlock *struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Name      string `json:"name"`
		Thinking  string `json:"thinking"`
		Signature string `json:"signature"`
		Data      string `json:"data"`
	} `json:"content_block"`

	// content_block_delta / message_delta
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
		Thinking    string `json:"thinking"`
		Signature   string `json:"signature"`
	} `json:"delta"`

	// message_start
	Message *struct {
		Usage *anthropicUsage `json:"usage"`
	} `json:"message"`

	// message_delta
	Usage *anthropicUsage `json:"usage"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

func tokenUsageFrom(u *anthropicUsage) cometsdk.TokenUsage {
	if u == nil {
		return cometsdk.TokenUsage{}
	}
	return cometsdk.TokenUsage{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		CacheRead:    u.CacheReadInputTokens,
		CacheWrite:   u.CacheCreationInputTokens,
	}
}

// mergeUsage overlays incoming onto base. Official Anthropic streams put
// input/cache counts on message_start and only output_tokens on message_delta.
func mergeUsage(base, incoming cometsdk.TokenUsage) cometsdk.TokenUsage {
	if incoming.InputTokens > 0 {
		base.InputTokens = incoming.InputTokens
	}
	if incoming.OutputTokens > 0 {
		base.OutputTokens = incoming.OutputTokens
	}
	if incoming.CacheRead > 0 {
		base.CacheRead = incoming.CacheRead
	}
	if incoming.CacheWrite > 0 {
		base.CacheWrite = incoming.CacheWrite
	}
	return base
}

// streamState tracks in-progress tool calls and thinking blocks.
type streamState struct {
	// toolCallBuffers maps content block index → accumulated input JSON.
	toolCallBuffers map[int]*strings.Builder
	// toolCallMeta maps content block index → (id, name).
	toolCallMeta map[int][2]string
	// thinkingBlocks maps content block index → accumulated thinking / redacted_thinking.
	thinkingBlocks map[int]*anthropicBlock
	pendingUsage   cometsdk.TokenUsage
	emittedFinish  bool
}

func newStreamState() *streamState {
	return &streamState{
		toolCallBuffers: make(map[int]*strings.Builder),
		toolCallMeta:    make(map[int][2]string),
		thinkingBlocks:  make(map[int]*anthropicBlock),
	}
}

func (s *streamState) completedThinkingBlocks() []anthropicBlock {
	if len(s.thinkingBlocks) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(s.thinkingBlocks))
	for idx := range s.thinkingBlocks {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	out := make([]anthropicBlock, 0, len(indexes))
	for _, idx := range indexes {
		if block := s.thinkingBlocks[idx]; block != nil {
			out = append(out, *block)
		}
	}
	return out
}

// toSDKEvents converts a raw Anthropic SSE event (by type name + JSON data)
// into zero or more SDK events. state is updated in-place for tool call assembly.
func toSDKEvents(eventType, data string, state *streamState) ([]cometsdk.Event, error) {
	switch eventType {
	case "content_block_start":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse content_block_start: %w", err)
		}
		if ev.ContentBlock == nil {
			return nil, nil
		}
		if ev.ContentBlock.Type == "tool_use" {
			state.toolCallBuffers[ev.Index] = &strings.Builder{}
			state.toolCallMeta[ev.Index] = [2]string{ev.ContentBlock.ID, ev.ContentBlock.Name}
			return []cometsdk.Event{cometsdk.ToolCallStartEvent{
				ID:   ev.ContentBlock.ID,
				Name: ev.ContentBlock.Name,
			}}, nil
		}
		if ev.ContentBlock.Type == "thinking" {
			state.thinkingBlocks[ev.Index] = &anthropicBlock{
				Type:      "thinking",
				Thinking:  ev.ContentBlock.Thinking,
				Signature: ev.ContentBlock.Signature,
			}
			return []cometsdk.Event{cometsdk.ReasoningStartEvent{}}, nil
		}
		if ev.ContentBlock.Type == "redacted_thinking" {
			state.thinkingBlocks[ev.Index] = &anthropicBlock{
				Type: "redacted_thinking",
				Data: ev.ContentBlock.Data,
			}
			return nil, nil
		}
		return nil, nil

	case "content_block_delta":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse content_block_delta: %w", err)
		}
		if ev.Delta == nil {
			return nil, nil
		}
		switch ev.Delta.Type {
		case "text_delta":
			return []cometsdk.Event{cometsdk.TextDeltaEvent{Text: ev.Delta.Text}}, nil

		case "input_json_delta":
			buf, ok := state.toolCallBuffers[ev.Index]
			if !ok {
				return nil, nil
			}
			buf.WriteString(ev.Delta.PartialJSON)
			meta := state.toolCallMeta[ev.Index]
			return []cometsdk.Event{cometsdk.ToolCallDeltaEvent{
				ID:    meta[0],
				Delta: ev.Delta.PartialJSON,
			}}, nil

		case "thinking_delta":
			if block, ok := state.thinkingBlocks[ev.Index]; ok && block.Type == "thinking" {
				block.Thinking += ev.Delta.Thinking
			}
			if ev.Delta.Thinking == "" {
				return nil, nil
			}
			return []cometsdk.Event{cometsdk.ReasoningContentEvent{Text: ev.Delta.Thinking}}, nil

		case "signature_delta":
			if block, ok := state.thinkingBlocks[ev.Index]; ok && block.Type == "thinking" {
				block.Signature += ev.Delta.Signature
			}
			return nil, nil
		}
		return nil, nil

	case "content_block_stop":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse content_block_stop: %w", err)
		}
		if buf, ok := state.toolCallBuffers[ev.Index]; ok {
			meta := state.toolCallMeta[ev.Index]
			delete(state.toolCallBuffers, ev.Index)
			delete(state.toolCallMeta, ev.Index)
			return []cometsdk.Event{cometsdk.ToolCallDoneEvent{
				ID:    meta[0],
				Name:  meta[1],
				Input: json.RawMessage(buf.String()),
			}}, nil
		}
		return nil, nil

	case "message_start":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse message_start: %w", err)
		}
		if ev.Message != nil {
			state.pendingUsage = mergeUsage(state.pendingUsage, tokenUsageFrom(ev.Message.Usage))
		}
		return nil, nil

	case "message_delta":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse message_delta: %w", err)
		}
		state.pendingUsage = mergeUsage(state.pendingUsage, tokenUsageFrom(ev.Usage))
		reason := ""
		if ev.Delta != nil {
			reason = ev.Delta.StopReason
		}
		state.emittedFinish = true
		return []cometsdk.Event{cometsdk.StepFinishEvent{
			FinishReason: cometsdk.NormalizeFinishReason(reason),
			Usage:        state.pendingUsage,
		}}, nil

	case "message_stop":
		var events []cometsdk.Event
		events = append(events, thinkingReplayEvent(state.completedThinkingBlocks())...)
		if !state.emittedFinish {
			events = append(events, cometsdk.StepFinishEvent{Usage: state.pendingUsage})
			state.emittedFinish = true
		}
		return append(events, cometsdk.DoneEvent{}), nil

	// Ignore ping and any unknown event types.
	default:
		return nil, nil
	}
}
