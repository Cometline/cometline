package anthropic

import (
	"encoding/json"
	"fmt"
	"regexp"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/internal/providerbase"
)

// ─── Outgoing: SDK Request → Anthropic JSON ───────────────────────────────────

type anthropicRequest struct {
	Model        string                 `json:"model"`
	MaxTokens    int                    `json:"max_tokens"`
	System       string                 `json:"system,omitempty"`
	Messages     []anthropicMessage     `json:"messages"`
	Tools        []anthropicTool        `json:"tools,omitempty"`
	OutputConfig *anthropicOutputConfig `json:"output_config,omitempty"`
	Stream       bool                   `json:"stream"`
}

type anthropicOutputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *anthropicImage `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	Data      string          `json:"data,omitempty"`
}

// thinkingReplayState is the opaque ProviderState.Data payload used to echo
// Anthropic thinking / redacted_thinking blocks on later turns. Reasoning
// plaintext never lives here — that stays in Message.ReasoningContent for UI.
type thinkingReplayState struct {
	Blocks []anthropicBlock `json:"blocks"`
}

type anthropicImage struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// toolIDRe matches valid Anthropic tool call ID characters.
var toolIDRe = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

// sanitiseToolCallID strips any characters not in [a-zA-Z0-9_-].
func sanitiseToolCallID(id string) string {
	return toolIDRe.ReplaceAllString(id, "_")
}

// toAnthropicRequest converts a cometsdk.Request to Anthropic API JSON.
// Any keys in req.Options["anthropic"] are merged into the final payload, allowing
// callers to pass provider-specific fields such as thinking, cache_control,
// top_k, top_p, etc. without requiring changes to this package.
// SDK-managed fields (model, messages, stream, max_tokens) take precedence
// and cannot be overridden via Options.
func toAnthropicRequest(req *cometsdk.Request) ([]byte, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Anthropic requires max_tokens; use a safe default.
	}

	msgs, err := convertMessages(req.Messages, req.Model)
	if err != nil {
		return nil, err
	}
	msgs = filterEmptyContent(msgs)

	ar := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    req.System,
		Messages:  msgs,
		Stream:    true,
	}
	if req.ReasoningEffort != "" {
		ar.OutputConfig = &anthropicOutputConfig{Effort: req.ReasoningEffort}
	}

	for _, t := range req.Tools {
		ar.Tools = append(ar.Tools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	return providerbase.MarshalWithOptions(ar, req.Options, "anthropic")
}

// convertMessages converts SDK messages to Anthropic message structs.
func convertMessages(msgs []cometsdk.Message, modelID string) ([]anthropicMessage, error) {
	out := make([]anthropicMessage, 0, len(msgs))
	for _, m := range msgs {
		role, blocks, err := convertMessage(m, modelID)
		if err != nil {
			return nil, err
		}
		out = append(out, anthropicMessage{Role: role, Content: blocks})
	}
	return out, nil
}

func convertMessage(m cometsdk.Message, modelID string) (string, []anthropicBlock, error) {
	switch m.Role {
	case cometsdk.RoleUser:
		blocks, err := convertBlocks(m.Content)
		return "user", blocks, err

	case cometsdk.RoleAssistant:
		// Plaintext CoT is display-only. Anthropic only accepts thinking blocks
		// that carry the original signature, which we replay from ProviderState.
		allBlocks := replayThinkingBlocks(m.ProviderState, modelID)
		contentBlocks, err := convertBlocks(m.Content)
		if err != nil {
			return "", nil, err
		}
		allBlocks = append(allBlocks, contentBlocks...)
		return "assistant", allBlocks, nil

	case cometsdk.RoleToolResult:
		// Tool results are sent as a "user" turn in Anthropic's API.
		blocks := make([]anthropicBlock, 0, len(m.Content))
		for _, b := range m.Content {
			tr, ok := b.(cometsdk.ToolResultBlock)
			if !ok {
				return "", nil, fmt.Errorf("anthropic: RoleToolResult message contains non-ToolResultBlock")
			}
			blocks = append(blocks, anthropicBlock{
				Type:      "tool_result",
				ToolUseID: sanitiseToolCallID(tr.ToolCallID),
				Content:   tr.Content,
				IsError:   tr.IsError,
			})
		}
		return "user", blocks, nil

	default:
		return "", nil, fmt.Errorf("anthropic: unknown role %q", m.Role)
	}
}

func convertBlocks(blocks []cometsdk.Block) ([]anthropicBlock, error) {
	out := make([]anthropicBlock, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.TextBlock:
			out = append(out, anthropicBlock{Type: "text", Text: v.Text})

		case cometsdk.ImageBlock:
			out = append(out, anthropicBlock{
				Type: "image",
				Source: &anthropicImage{
					Type:      "base64",
					MediaType: v.MediaType,
					Data:      v.Data,
				},
			})

		case cometsdk.ReasoningBlock:
			// Display-only CoT. Never emit type:"reasoning" (illegal) and never
			// invent a thinking block without Anthropic's signature.
			continue

		case cometsdk.ToolCallBlock:
			input := v.Input
			if len(input) == 0 {
				input = json.RawMessage(`{}`)
			}
			out = append(out, anthropicBlock{
				Type:  "tool_use",
				ID:    sanitiseToolCallID(v.ID),
				Name:  v.Name,
				Input: input,
			})

		case cometsdk.ToolResultBlock:
			out = append(out, anthropicBlock{
				Type:      "tool_result",
				ToolUseID: sanitiseToolCallID(v.ToolCallID),
				Content:   v.Content,
				IsError:   v.IsError,
			})

		default:
			return nil, fmt.Errorf("anthropic: unsupported block type %T", b)
		}
	}
	return out, nil
}

// filterEmptyContent removes messages whose content slice is empty.
// Anthropic rejects requests with empty content arrays.
func filterEmptyContent(msgs []anthropicMessage) []anthropicMessage {
	out := msgs[:0]
	for _, m := range msgs {
		if len(m.Content) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func replayThinkingBlocks(states []cometsdk.ProviderState, modelID string) []anthropicBlock {
	if modelID == "" {
		return nil
	}
	var out []anthropicBlock
	for _, state := range states {
		if state.ProviderID != "" && state.ProviderID != providerID {
			continue
		}
		if state.ModelID != "" && state.ModelID != modelID {
			continue
		}
		if state.Data == "" {
			continue
		}
		var payload thinkingReplayState
		if err := json.Unmarshal([]byte(state.Data), &payload); err != nil {
			continue
		}
		for _, block := range payload.Blocks {
			if replayableThinkingBlock(block) {
				out = append(out, block)
			}
		}
	}
	return out
}

func replayableThinkingBlock(block anthropicBlock) bool {
	switch block.Type {
	case "thinking":
		return block.Signature != ""
	case "redacted_thinking":
		return block.Data != ""
	default:
		return false
	}
}

func marshalThinkingReplayState(blocks []anthropicBlock) string {
	replay := make([]anthropicBlock, 0, len(blocks))
	for _, block := range blocks {
		if replayableThinkingBlock(block) {
			replay = append(replay, block)
		}
	}
	if len(replay) == 0 {
		return ""
	}
	raw, err := json.Marshal(thinkingReplayState{Blocks: replay})
	if err != nil {
		return ""
	}
	return string(raw)
}

func thinkingReplayEvent(blocks []anthropicBlock) []cometsdk.Event {
	data := marshalThinkingReplayState(blocks)
	if data == "" {
		return nil
	}
	return []cometsdk.Event{cometsdk.ProviderStateEvent{State: cometsdk.ProviderState{
		ProviderID: providerID,
		Data:       data,
	}}}
}
