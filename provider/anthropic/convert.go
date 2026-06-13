package anthropic

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
)

// ─── Outgoing: SDK Request → Anthropic JSON ───────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Reasoning string          `json:"reasoning,omitempty"`
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

	msgs, err := convertMessages(req.Messages)
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
func convertMessages(msgs []cometsdk.Message) ([]anthropicMessage, error) {
	out := make([]anthropicMessage, 0, len(msgs))
	for _, m := range msgs {
		role, blocks, err := convertMessage(m)
		if err != nil {
			return nil, err
		}
		out = append(out, anthropicMessage{Role: role, Content: blocks})
	}
	return out, nil
}

func convertMessage(m cometsdk.Message) (string, []anthropicBlock, error) {
	switch m.Role {
	case cometsdk.RoleUser:
		blocks, err := convertBlocks(m.Content)
		return "user", blocks, err

	case cometsdk.RoleAssistant:
		var allBlocks []anthropicBlock
		reasoningBlocks, err := convertBlocks(m.ReasoningContent)
		if err != nil {
			return "", nil, err
		}
		allBlocks = append(allBlocks, reasoningBlocks...)
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

		case cometsdk.ReasoningBlock:
			out = append(out, anthropicBlock{Type: "reasoning", Reasoning: v.Text})

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

// ─── Incoming: Anthropic SSE → SDK Events ────────────────────────────────────

// anthropicSSEEvent is the top-level JSON structure of Anthropic SSE data payloads.
type anthropicSSEEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`

	// content_block_start
	ContentBlock *struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`

	// content_block_delta
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
		Reasoning   string `json:"reasoning"`
	} `json:"delta"`

	// message_delta
	Usage *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// streamState tracks in-progress tool calls so we can emit ToolCallDoneEvent.
type streamState struct {
	// toolCallBuffers maps content block index → accumulated input JSON.
	toolCallBuffers map[int]*strings.Builder
	// toolCallMeta maps content block index → (id, name).
	toolCallMeta map[int][2]string
}

func newStreamState() *streamState {
	return &streamState{
		toolCallBuffers: make(map[int]*strings.Builder),
		toolCallMeta:    make(map[int][2]string),
	}
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
		if ev.ContentBlock.Type == "reasoning" {
			return []cometsdk.Event{cometsdk.ReasoningStartEvent{}}, nil
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

		case "reasoning_delta":
			return []cometsdk.Event{cometsdk.ReasoningContentEvent{Text: ev.Delta.Reasoning}}, nil
		}
		return nil, nil

	case "content_block_stop":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse content_block_stop: %w", err)
		}
		buf, ok := state.toolCallBuffers[ev.Index]
		if !ok {
			return nil, nil
		}
		meta := state.toolCallMeta[ev.Index]
		delete(state.toolCallBuffers, ev.Index)
		delete(state.toolCallMeta, ev.Index)
		return []cometsdk.Event{cometsdk.ToolCallDoneEvent{
			ID:    meta[0],
			Name:  meta[1],
			Input: json.RawMessage(buf.String()),
		}}, nil

	case "message_delta":
		var ev anthropicSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("anthropic: parse message_delta: %w", err)
		}
		usage := cometsdk.TokenUsage{}
		if ev.Usage != nil {
			usage.InputTokens = ev.Usage.InputTokens
			usage.OutputTokens = ev.Usage.OutputTokens
			usage.CacheRead = ev.Usage.CacheReadInputTokens
			usage.CacheWrite = ev.Usage.CacheCreationInputTokens
		}
		reason := ""
		if ev.Delta != nil {
			reason = ev.Delta.StopReason
		}
		return []cometsdk.Event{cometsdk.StepFinishEvent{
			FinishReason: cometsdk.NormalizeFinishReason(reason),
			Usage:        usage,
		}}, nil

	case "message_stop":
		return []cometsdk.Event{cometsdk.DoneEvent{}}, nil

	// Ignore ping, message_start, and any unknown event types.
	default:
		return nil, nil
	}
}
