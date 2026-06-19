package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
)

type codexRequest struct {
	Model           string       `json:"model"`
	Input           []codexInput `json:"input"`
	Instructions    string       `json:"instructions,omitempty"`
	Tools           []codexTool  `json:"tools,omitempty"`
	MaxOutputTokens int          `json:"max_output_tokens,omitempty"`
	Temperature     *float64     `json:"temperature,omitempty"`
	Store           bool         `json:"store"`
	Stream          bool         `json:"stream"`
}

type codexInput struct {
	Type    string             `json:"type,omitempty"`
	Role    string             `json:"role,omitempty"`
	Content []codexContentPart `json:"content,omitempty"`
	CallID  string             `json:"call_id,omitempty"`
	Name    string             `json:"name,omitempty"`
	Args    json.RawMessage    `json:"arguments,omitempty"`
	Output  string             `json:"output,omitempty"`
}

type codexContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type codexTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

func toCodexRequest(req *cometsdk.Request, disableMaxOutputTokens bool) ([]byte, error) {
	input, err := convertMessages(req.Messages)
	if err != nil {
		return nil, err
	}
	out := codexRequest{
		Model:        req.Model,
		Input:        input,
		Instructions: req.System,
		Store:        false,
		Stream:       true,
		Temperature:  req.Temperature,
	}
	if req.MaxTokens > 0 && !disableMaxOutputTokens {
		out.MaxOutputTokens = req.MaxTokens
	}
	for _, t := range req.Tools {
		params := t.Parameters
		if len(strings.TrimSpace(string(params))) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Tools = append(out.Tools, codexTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
			Strict:      false,
		})
	}
	return providerbase.MarshalWithOptions(out, req.Options, providerID)
}

func convertMessages(messages []cometsdk.Message) ([]codexInput, error) {
	var out []codexInput
	for _, m := range messages {
		converted, err := convertMessage(m)
		if err != nil {
			return nil, err
		}
		out = append(out, converted...)
	}
	return out, nil
}

func convertMessage(m cometsdk.Message) ([]codexInput, error) {
	switch m.Role {
	case cometsdk.RoleUser:
		parts, err := inputContentParts(m.Content)
		if err != nil {
			return nil, err
		}
		return []codexInput{{Role: "user", Content: parts}}, nil
	case cometsdk.RoleAssistant:
		var out []codexInput
		var textParts []codexContentPart
		for _, b := range m.Content {
			switch v := b.(type) {
			case cometsdk.TextBlock:
				textParts = append(textParts, codexContentPart{Type: "output_text", Text: v.Text})
			case cometsdk.ToolCallBlock:
				args := v.Input
				if len(strings.TrimSpace(string(args))) == 0 {
					args = json.RawMessage(`{}`)
				}
				out = append(out, codexInput{Type: "function_call", CallID: v.ID, Name: v.Name, Args: args})
			default:
				return nil, fmt.Errorf("codex: unsupported block type %T in assistant message", b)
			}
		}
		if len(textParts) > 0 {
			out = append([]codexInput{{Role: "assistant", Content: textParts}}, out...)
		}
		return out, nil
	case cometsdk.RoleToolResult:
		var out []codexInput
		for _, b := range m.Content {
			tr, ok := b.(cometsdk.ToolResultBlock)
			if !ok {
				return nil, fmt.Errorf("codex: RoleToolResult message contains non-ToolResultBlock")
			}
			out = append(out, codexInput{Type: "function_call_output", CallID: tr.ToolCallID, Output: tr.Content})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("codex: unknown role %q", m.Role)
	}
}

func inputContentParts(blocks []cometsdk.Block) ([]codexContentPart, error) {
	parts := make([]codexContentPart, 0, len(blocks))
	for _, b := range blocks {
		switch v := b.(type) {
		case cometsdk.TextBlock:
			parts = append(parts, codexContentPart{Type: "input_text", Text: v.Text})
		case cometsdk.ImageBlock:
			parts = append(parts, codexContentPart{Type: "input_image", ImageURL: fmt.Sprintf("data:%s;base64,%s", v.MediaType, v.Data)})
		default:
			return nil, fmt.Errorf("codex: unsupported block type %T in user message", b)
		}
	}
	return parts, nil
}
