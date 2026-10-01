package openai

import (
	"encoding/json"
	"fmt"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/internal/providerbase"
)

// ─── Outgoing: SDK Request → OpenAI JSON ─────────────────────────────────────

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	Tools               []openAITool    `json:"tools,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Stream              bool            `json:"stream"`
	StreamOptions       *streamOptions  `json:"stream_options,omitempty"`
	ReasoningSplit      *bool           `json:"reasoning_split,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"` // string | []openAIContentPart
	Reasoning  *json.RawMessage `json:"reasoning_content,omitempty"`
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
func toOpenAIRequest(req *cometsdk.Request, disableImageContent bool, enableReasoningSplit bool, useMaxCompletionTokens bool, preserveEmptyReasoningContent ...bool) ([]byte, error) {
	preserveEmptyReasoning := len(preserveEmptyReasoningContent) > 0 && preserveEmptyReasoningContent[0]
	msgs, err := convertMessages(req.System, req.Messages, disableImageContent, preserveEmptyReasoning)
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
	if req.ReasoningEffort != "" {
		or.ReasoningEffort = req.ReasoningEffort
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
func convertMessages(system string, msgs []cometsdk.Message, disableImageContent bool, preserveEmptyReasoningContent bool) ([]openAIMessage, error) {
	var out []openAIMessage

	if system != "" {
		out = append(out, openAIMessage{Role: "system", Content: system})
	}

	for _, m := range msgs {
		converted, err := convertMessage(m, disableImageContent, preserveEmptyReasoningContent)
		if err != nil {
			return nil, err
		}
		out = append(out, converted...)
	}
	return out, nil
}

// convertMessage converts a single SDK message to one or more OpenAI messages.
// Reasoning content is placed in the reasoning_content field of the same message.
func convertMessage(m cometsdk.Message, disableImageContent bool, preserveEmptyReasoningContent bool) ([]openAIMessage, error) {
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
			raw, err := json.Marshal(reasoningParts[0].Text)
			if err != nil {
				return nil, fmt.Errorf("openai: marshal reasoning content: %w", err)
			}
			value := json.RawMessage(raw)
			msg.Reasoning = &value
		} else if len(reasoningParts) > 1 {
			raw, err := json.Marshal(reasoningParts)
			if err != nil {
				return nil, fmt.Errorf("openai: marshal reasoning content: %w", err)
			}
			value := json.RawMessage(raw)
			msg.Reasoning = &value
		} else if preserveEmptyReasoningContent && m.Role == cometsdk.RoleAssistant {
			raw := json.RawMessage(`""`)
			msg.Reasoning = &raw
		}

		if len(textParts) == 1 {
			msg.Content = textParts[0].Text
		} else if len(textParts) > 1 {
			msg.Content = textParts
		} else {
			// OpenAI itself accepts null here, but a number of OpenAI-compatible
			// gateways reject it. Keep every assistant replay structurally valid.
			msg.Content = ""
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
