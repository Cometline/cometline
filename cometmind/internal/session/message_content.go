package session

import (
	"encoding/json"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

const (
	contentEnvelopePrefix = "cometmind:content:v1\n"
	errorMessagePrefix    = "cometmind:error:v1\n"
)

// ContentBlock is the persisted/API representation of multimodal content.
// User images typically carry base64 Data. Assistant-presented images store
// a media-store ID (no inline bytes) so blobs live under ~/.cometmind/media.
type ContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	ID        string `json:"id,omitempty"`
	Alt       string `json:"alt,omitempty"`
}

// MessageContextRef is a slim UI reference for a web/file/terminal/message context
// attached to a user turn. Content bodies are not stored here — they are
// already inlined into the agent-facing text blocks.
type MessageContextRef struct {
	Kind   string `json:"kind"`
	Title  string `json:"title,omitempty"`
	Source string `json:"source"`
	Role   string `json:"role,omitempty"` // "viewing" for path-only file refs
}

type contentEnvelope struct {
	Blocks      []ContentBlock      `json:"blocks"`
	DisplayText string              `json:"display_text,omitempty"`
	Contexts    []MessageContextRef `json:"contexts,omitempty"`
}

type errorMessageEnvelope struct {
	Message string `json:"message"`
}

// toolResultPayload is stored in messages.content for role=tool_result.
type toolResultPayload struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

func marshalErrorMessageContent(message string) (string, error) {
	env := errorMessageEnvelope{Message: strings.TrimSpace(message)}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return errorMessagePrefix + string(raw), nil
}

// DecodeErrorMessageContent decodes system rows that represent transcript errors.
func DecodeErrorMessageContent(raw string) (string, bool) {
	if !strings.HasPrefix(raw, errorMessagePrefix) {
		return "", false
	}
	var env errorMessageEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, errorMessagePrefix)), &env); err != nil {
		return strings.TrimSpace(raw), true
	}
	msg := strings.TrimSpace(env.Message)
	if msg == "" {
		msg = "The request failed."
	}
	return msg, true
}

func marshalMessageContent(blocks []ContentBlock, displayText string, contexts []MessageContextRef) (string, error) {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" && len(contexts) == 0 && len(blocks) == 1 && blocks[0].Type == "text" {
		return blocks[0].Text, nil
	}
	env := contentEnvelope{Blocks: blocks}
	if displayText != "" {
		env.DisplayText = displayText
	}
	if len(contexts) > 0 {
		env.Contexts = contexts
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return contentEnvelopePrefix + string(raw), nil
}

// ContextsFromStoredContent returns UI context refs persisted on a user message.
func ContextsFromStoredContent(raw string) []MessageContextRef {
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		return nil
	}
	var env contentEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err != nil {
		return nil
	}
	if len(env.Contexts) == 0 {
		return nil
	}
	return env.Contexts
}

// DecodeMessageContent returns content blocks from a persisted message. Plain
// legacy content is treated as a single text block.
func DecodeMessageContent(raw string) ([]ContentBlock, error) {
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		return []ContentBlock{{Type: "text", Text: raw}}, nil
	}
	var env contentEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err != nil {
		return nil, err
	}
	return env.Blocks, nil
}

// PlainTextFromContent extracts agent-facing text from decoded content blocks.
func PlainTextFromContent(blocks []ContentBlock) string {
	var b strings.Builder
	for _, block := range blocks {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// DisplayTextFromStoredContent returns the UI label for a persisted user message.
func DisplayTextFromStoredContent(raw string) string {
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		return raw
	}
	var env contentEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, contentEnvelopePrefix)), &env); err != nil {
		return raw
	}
	if strings.TrimSpace(env.DisplayText) != "" {
		return env.DisplayText
	}
	return PlainTextFromContent(env.Blocks)
}

// TitleTextFromContent picks a short session title from user content.
func TitleTextFromContent(blocks []ContentBlock, displayText string) string {
	if strings.TrimSpace(displayText) != "" {
		return displayText
	}
	return PlainTextFromContent(blocks)
}

type reasoningBlockPayload struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func marshalReasoningContent(blocks []cometsdk.Block) (string, error) {
	// Always emit a JSON array (never "null") so the NOT NULL column and
	// OpenAI-compatible reasoning_content replay stay well-formed.
	payloads := make([]reasoningBlockPayload, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.TextBlock:
			payloads = append(payloads, reasoningBlockPayload{Type: "text", Text: v.Text})
		case cometsdk.ReasoningBlock:
			payloads = append(payloads, reasoningBlockPayload{Type: "reasoning", Text: v.Text})
		}
	}
	raw, err := json.Marshal(payloads)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalReasoningContent(raw string) ([]cometsdk.Block, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "[]" {
		return nil, nil
	}
	var payloads []reasoningBlockPayload
	if err := json.Unmarshal([]byte(raw), &payloads); err != nil {
		return nil, err
	}
	var blocks []cometsdk.Block
	for _, p := range payloads {
		switch p.Type {
		case "text":
			blocks = append(blocks, cometsdk.TextBlock{Text: p.Text})
		case "reasoning":
			blocks = append(blocks, cometsdk.ReasoningBlock{Text: p.Text})
		}
	}
	return blocks, nil
}
