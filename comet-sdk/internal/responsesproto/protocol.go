// Package responsesproto implements the OpenAI Responses wire protocol shared
// by the Codex and API-key (OpenCode Go) providers: request conversion and the
// streaming event parser. Provider-specific concerns (auth, headers, Responses
// Lite behavior) stay in the provider packages.
package responsesproto

import (
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
)

// RequestOptions controls protocol-level request shaping shared by providers.
type RequestOptions struct {
	// ProviderKey is the key under which req.Options provider-specific
	// overrides are merged (e.g. "codex" or "openai").
	ProviderKey string
	// DisableMaxOutputTokens omits max_output_tokens from the request.
	DisableMaxOutputTokens bool
	// DisableReasoningSummary omits the reasoning summary request.
	DisableReasoningSummary bool
	// ReplayEncryptedState replays persisted encrypted reasoning state.
	ReplayEncryptedState bool
	// IncludeEncryptedReasoning requests encrypted reasoning state in
	// responses so stateless multi-turn reasoning (store: false) works.
	IncludeEncryptedReasoning bool
	// Store persists the response server-side when true.
	Store bool
}

// Request is the OpenAI Responses request body.
type Request struct {
	Model           string      `json:"model"`
	Input           []InputItem `json:"input"`
	Instructions    string      `json:"instructions,omitempty"`
	Tools           []Tool      `json:"tools,omitempty"`
	Reasoning       *Reasoning  `json:"reasoning,omitempty"`
	MaxOutputTokens int         `json:"max_output_tokens,omitempty"`
	Temperature     *float64    `json:"temperature,omitempty"`
	Store           bool        `json:"store"`
	Stream          bool        `json:"stream"`
	Include         []string    `json:"include,omitempty"`
}

// Reasoning requests displayable reasoning summaries and optional effort.
type Reasoning struct {
	Summary string `json:"summary,omitempty"`
	Effort  string `json:"effort,omitempty"`
}

// InputItem is one input entry: a role message, a reasoning item, a function
// call, or a function call output.
type InputItem struct {
	Type             string        `json:"type,omitempty"`
	Role             string        `json:"role,omitempty"`
	Content          []ContentPart `json:"content,omitempty"`
	Summary          []ContentPart `json:"summary,omitempty"`
	CallID           string        `json:"call_id,omitempty"`
	Name             string        `json:"name,omitempty"`
	Args             string        `json:"arguments,omitempty"`
	Output           string        `json:"output,omitempty"`
	EncryptedContent string        `json:"encrypted_content,omitempty"`
}

// MarshalJSON keeps encrypted reasoning items valid for the Responses API.
// A bare {type:reasoning, encrypted_content:...} is rejected with
// Missing required parameter: 'input[N].summary', and omitempty would drop an
// empty summary slice, so encrypted items always emit an explicit summary array.
func (c InputItem) MarshalJSON() ([]byte, error) {
	type wire struct {
		Type             string        `json:"type,omitempty"`
		Role             string        `json:"role,omitempty"`
		Content          []ContentPart `json:"content,omitempty"`
		Summary          []ContentPart `json:"summary,omitempty"`
		CallID           string        `json:"call_id,omitempty"`
		Name             string        `json:"name,omitempty"`
		Args             string        `json:"arguments,omitempty"`
		Output           string        `json:"output,omitempty"`
		EncryptedContent string        `json:"encrypted_content,omitempty"`
	}
	if c.EncryptedContent == "" {
		return json.Marshal(wire(c))
	}
	summary := c.Summary
	if summary == nil {
		summary = []ContentPart{}
	}
	return json.Marshal(struct {
		Type             string        `json:"type,omitempty"`
		Role             string        `json:"role,omitempty"`
		Content          []ContentPart `json:"content,omitempty"`
		Summary          []ContentPart `json:"summary"`
		CallID           string        `json:"call_id,omitempty"`
		Name             string        `json:"name,omitempty"`
		Args             string        `json:"arguments,omitempty"`
		Output           string        `json:"output,omitempty"`
		EncryptedContent string        `json:"encrypted_content,omitempty"`
	}{
		Type:             c.Type,
		Role:             c.Role,
		Content:          c.Content,
		Summary:          summary,
		CallID:           c.CallID,
		Name:             c.Name,
		Args:             c.Args,
		Output:           c.Output,
		EncryptedContent: c.EncryptedContent,
	})
}

// ContentPart is a text or image part inside an input item.
type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// Tool is a function tool declaration.
type Tool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

// BuildRequest converts a cometsdk.Request to the Responses wire format.
func BuildRequest(req *cometsdk.Request, opts RequestOptions) ([]byte, error) {
	input, err := convertMessages(req.Messages, req.Model, opts.ReplayEncryptedState)
	if err != nil {
		return nil, err
	}
	out := Request{
		Model:        req.Model,
		Input:        input,
		Instructions: req.System,
		Store:        opts.Store,
		Stream:       true,
		Temperature:  req.Temperature,
	}
	if !opts.DisableReasoningSummary || req.ReasoningEffort != "" {
		// Ask for a displayable summary when the model supports it. Providers
		// retry without this field if a model rejects it.
		reasoning := &Reasoning{}
		if !opts.DisableReasoningSummary {
			reasoning.Summary = "auto"
		}
		reasoning.Effort = req.ReasoningEffort
		out.Reasoning = reasoning
	}
	if req.MaxTokens > 0 && !opts.DisableMaxOutputTokens {
		out.MaxOutputTokens = req.MaxTokens
	}
	if opts.IncludeEncryptedReasoning {
		out.Include = []string{"reasoning.encrypted_content"}
	}
	for _, t := range req.Tools {
		params := t.Parameters
		if len(strings.TrimSpace(string(params))) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Tools = append(out.Tools, Tool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
			Strict:      false,
		})
	}
	return providerbase.MarshalWithOptions(out, req.Options, opts.ProviderKey)
}

func convertMessages(messages []cometsdk.Message, modelID string, replayEncryptedState bool) ([]InputItem, error) {
	var out []InputItem
	toolNames := make(map[string]string)
	for _, m := range messages {
		converted, err := convertMessage(m, toolNames, modelID, replayEncryptedState)
		if err != nil {
			return nil, err
		}
		out = append(out, converted...)
	}
	return out, nil
}

func convertMessage(m cometsdk.Message, toolNames map[string]string, modelID string, replayEncryptedState bool) ([]InputItem, error) {
	switch m.Role {
	case cometsdk.RoleUser:
		parts, err := inputContentParts(m.Content)
		if err != nil {
			return nil, err
		}
		return []InputItem{{Role: "user", Content: parts}}, nil
	case cometsdk.RoleAssistant:
		var out []InputItem
		reasoningSummary := reasoningSummaryParts(m.ReasoningContent)
		var encrypted string
		if replayEncryptedState {
			for _, state := range m.ProviderState {
				if state.ModelID == modelID && state.Data != "" {
					// One assistant message stores at most one opaque state per model.
					encrypted = state.Data
				}
			}
		}
		// Responses requires reasoning input items to carry summary. Merge
		// opaque encrypted state with the displayable summary into a single
		// item so we never emit encrypted_content without summary.
		if encrypted != "" || len(reasoningSummary) > 0 {
			out = append(out, InputItem{
				Type:             "reasoning",
				EncryptedContent: encrypted,
				Summary:          reasoningSummary,
			})
		}
		var textParts []ContentPart
		var toolCalls []InputItem
		for _, b := range m.Content {
			switch v := b.(type) {
			case cometsdk.TextBlock:
				textParts = append(textParts, ContentPart{Type: "output_text", Text: v.Text})
			case cometsdk.ToolCallBlock:
				args := v.Input
				if len(strings.TrimSpace(string(args))) == 0 {
					args = json.RawMessage(`{}`)
				}
				toolNames[v.ID] = v.Name
				toolCalls = append(toolCalls, InputItem{Type: "function_call", CallID: v.ID, Name: v.Name, Args: string(args)})
			default:
				return nil, fmt.Errorf("responses: unsupported block type %T in assistant message", b)
			}
		}
		if len(textParts) > 0 {
			out = append(out, InputItem{Role: "assistant", Content: textParts})
		}
		out = append(out, toolCalls...)
		return out, nil
	case cometsdk.RoleToolResult:
		var out []InputItem
		for _, b := range m.Content {
			tr, ok := b.(cometsdk.ToolResultBlock)
			if !ok {
				return nil, fmt.Errorf("responses: RoleToolResult message contains non-ToolResultBlock")
			}
			out = append(out, InputItem{
				Type:   "function_call_output",
				CallID: tr.ToolCallID,
				Name:   toolNames[tr.ToolCallID],
				Output: toolResultOutput(tr.Content),
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("responses: unknown role %q", m.Role)
	}
}

// toolResultOutput ensures function_call_output always carries the output
// field the API requires. Empty tool results must not be omitted via
// omitempty or the API returns HTTP 400.
func toolResultOutput(content string) string {
	if content == "" {
		return "(no output)"
	}
	return content
}

func reasoningSummaryParts(blocks []cometsdk.Block) []ContentPart {
	parts := make([]ContentPart, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.ReasoningBlock:
			if text := strings.TrimSpace(v.Text); text != "" {
				parts = append(parts, ContentPart{Type: "summary_text", Text: text})
			}
		case cometsdk.TextBlock:
			if text := strings.TrimSpace(v.Text); text != "" {
				parts = append(parts, ContentPart{Type: "summary_text", Text: text})
			}
		}
	}
	return parts
}

func inputContentParts(blocks []cometsdk.Block) ([]ContentPart, error) {
	parts := make([]ContentPart, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.TextBlock:
			parts = append(parts, ContentPart{Type: "input_text", Text: v.Text})
		case cometsdk.ImageBlock:
			parts = append(parts, ContentPart{Type: "input_image", ImageURL: fmt.Sprintf("data:%s;base64,%s", v.MediaType, v.Data)})
		default:
			return nil, fmt.Errorf("responses: unsupported block type %T in user message", b)
		}
	}
	return parts, nil
}
