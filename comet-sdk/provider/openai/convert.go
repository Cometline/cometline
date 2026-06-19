package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
)

// ─── Outgoing: SDK Request → OpenAI JSON ─────────────────────────────────────

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	Tools               []openAITool    `json:"tools,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Stream              bool            `json:"stream"`
	StreamOptions       *streamOptions  `json:"stream_options,omitempty"`
	ReasoningSplit      *bool           `json:"reasoning_split,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`                     // string | []openAIContentPart | null
	Reasoning  any              `json:"reasoning_content,omitempty"` // string | []openAIContentPart
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	Name       string           `json:"name,omitempty"`
}

type openAIContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openAIImageURL `json:"image_url,omitempty"`
}

type openAIImageURL struct {
	URL string `json:"url"`
}

type openAIToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string        `json:"type"`
	Function openAIToolDef `json:"function"`
}

type openAIToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// toOpenAIRequest converts a cometsdk.Request to OpenAI Chat Completions JSON.
// Any keys in req.Options["openai"] are merged into the final payload, allowing
// callers to pass provider-specific fields such as top_p, presence_penalty,
// frequency_penalty, seed, etc. without requiring changes to this package.
// SDK-managed fields (model, messages, stream, stream_options) take precedence
// and cannot be overridden via Options.
func toOpenAIRequest(req *cometsdk.Request, disableImageContent bool, enableReasoningSplit bool, useMaxCompletionTokens bool) ([]byte, error) {
	msgs, err := convertMessages(req.System, req.Messages, disableImageContent)
	if err != nil {
		return nil, err
	}

	or := openAIRequest{
		Model:         req.Model,
		Messages:      msgs,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	if enableReasoningSplit {
		reasoningSplit := true
		or.ReasoningSplit = &reasoningSplit
	}

	if req.MaxTokens > 0 {
		if useMaxCompletionTokens {
			or.MaxCompletionTokens = req.MaxTokens
		} else {
			or.MaxTokens = req.MaxTokens
		}
	}

	for _, t := range req.Tools {
		or.Tools = append(or.Tools, openAITool{
			Type: "function",
			Function: openAIToolDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	return providerbase.MarshalWithOptions(or, req.Options, "openai")
}

// convertMessages prepends a system message if provided, then converts all messages.
func convertMessages(system string, msgs []cometsdk.Message, disableImageContent bool) ([]openAIMessage, error) {
	var out []openAIMessage

	if system != "" {
		out = append(out, openAIMessage{Role: "system", Content: system})
	}

	for _, m := range msgs {
		converted, err := convertMessage(m, disableImageContent)
		if err != nil {
			return nil, err
		}
		out = append(out, converted...)
	}
	return out, nil
}

// convertMessage converts a single SDK message to one or more OpenAI messages.
// Reasoning content is placed in the reasoning_content field of the same message.
func convertMessage(m cometsdk.Message, disableImageContent bool) ([]openAIMessage, error) {
	switch m.Role {
	case cometsdk.RoleUser:
		parts, err := contentParts(m.Content, disableImageContent)
		if err != nil {
			return nil, err
		}
		return []openAIMessage{{Role: "user", Content: parts}}, nil

	case cometsdk.RoleAssistant:
		var textParts []openAIContentPart
		var reasoningParts []openAIContentPart
		var toolCalls []openAIToolCall

		for _, b := range m.Content {
			switch v := b.(type) {
			case cometsdk.TextBlock:
				textParts = append(textParts, openAIContentPart{Type: "text", Text: v.Text})
			case cometsdk.ToolCallBlock:
				args := string(v.Input)
				if args == "" {
					args = "{}"
				}
				toolCalls = append(toolCalls, openAIToolCall{
					ID:   v.ID,
					Type: "function",
					Function: openAIFunction{
						Name:      v.Name,
						Arguments: args,
					},
				})
			default:
				return nil, fmt.Errorf("openai: unsupported block type %T in assistant message", b)
			}
		}

		for _, b := range m.ReasoningContent {
			switch v := b.(type) {
			case cometsdk.TextBlock:
				reasoningParts = append(reasoningParts, openAIContentPart{Type: "text", Text: v.Text})
			case cometsdk.ReasoningBlock:
				reasoningParts = append(reasoningParts, openAIContentPart{Type: "text", Text: v.Text})
			default:
				return nil, fmt.Errorf("openai: unsupported reasoning block type %T", b)
			}
		}

		msg := openAIMessage{Role: "assistant"}

		if len(reasoningParts) == 1 {
			msg.Reasoning = reasoningParts[0].Text
		} else if len(reasoningParts) > 1 {
			msg.Reasoning = reasoningParts
		}

		if len(textParts) == 1 {
			msg.Content = textParts[0].Text
		} else if len(textParts) > 1 {
			msg.Content = textParts
		} else if len(textParts) == 0 && len(toolCalls) > 0 {
			// content: null for tool-call-only responses (OpenAI gateway requires this)
			msg.Content = nil
		}
		msg.ToolCalls = toolCalls

		return []openAIMessage{msg}, nil

	case cometsdk.RoleToolResult:
		var out []openAIMessage
		for _, b := range m.Content {
			tr, ok := b.(cometsdk.ToolResultBlock)
			if !ok {
				return nil, fmt.Errorf("openai: RoleToolResult message contains non-ToolResultBlock")
			}
			out = append(out, openAIMessage{
				Role:       "tool",
				ToolCallID: tr.ToolCallID,
				Content:    tr.Content,
			})
		}
		return out, nil

	default:
		return nil, fmt.Errorf("openai: unknown role %q", m.Role)
	}
}

// imagePlaceholderText is substituted for an image block when the target model
// cannot accept image input. The provider downgrades to this on a reactive
// retry after the endpoint rejects image content (see the image fallback in
// Stream); it keeps the turn structurally valid so replayed history does not
// break the conversation.
const imagePlaceholderText = "[image omitted: this model does not support image input]"

func contentParts(blocks []cometsdk.Block, disableImageContent bool) (any, error) {
	if len(blocks) == 1 {
		if tb, ok := blocks[0].(cometsdk.TextBlock); ok {
			return tb.Text, nil
		}
	}
	parts := make([]openAIContentPart, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.TextBlock:
			parts = append(parts, openAIContentPart{Type: "text", Text: v.Text})
		case cometsdk.ImageBlock:
			if disableImageContent {
				// Downgrade to a text placeholder so non-vision models (e.g.
				// DeepSeek) don't reject the request with HTTP 400 on the
				// "image_url" content part.
				parts = append(parts, openAIContentPart{Type: "text", Text: imagePlaceholderText})
				continue
			}
			parts = append(parts, openAIContentPart{
				Type:     "image_url",
				ImageURL: &openAIImageURL{URL: fmt.Sprintf("data:%s;base64,%s", v.MediaType, v.Data)},
			})
		default:
			return nil, fmt.Errorf("openai: unsupported block type %T in user message", b)
		}
	}
	return parts, nil
}

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
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
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

	if len(delta.Choices) == 0 && delta.Usage != nil {
		state.pendingUsage = cometsdk.TokenUsage{
			InputTokens:  delta.Usage.PromptTokens,
			OutputTokens: delta.Usage.CompletionTokens,
		}
		if state.pendingFinish != nil {
			state.pendingFinish.Usage = state.pendingUsage
		}
		return nil, nil
	}

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
		}
	}

	return events, nil
}
