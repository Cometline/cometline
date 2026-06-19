package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

var jsonFenceRE = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")

// GenerateJSON sends a request and unmarshals the model's text response into out.
// The caller owns schema validation; out must be a pointer to a struct or map.
//
// When req.Options is nil it is initialized. OpenAI providers receive
// response_format json_object via Options["openai"]. Anthropic receives
// a JSON-only instruction appended to the system prompt.
func GenerateJSON(ctx context.Context, p cometsdk.Provider, req *cometsdk.Request, out any) error {
	if out == nil {
		return fmt.Errorf("GenerateJSON: out must be non-nil")
	}
	if req == nil {
		return fmt.Errorf("GenerateJSON: req is required")
	}

	req = cloneRequest(req)
	ensureJSONMode(p, req)

	result, err := GenerateText(ctx, p, req)
	if err != nil {
		return err
	}

	raw, err := extractJSON(result.Text)
	if err != nil {
		return fmt.Errorf("GenerateJSON: parse response: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("GenerateJSON: unmarshal: %w", err)
	}
	return nil
}

func cloneRequest(req *cometsdk.Request) *cometsdk.Request {
	cp := *req
	if req.Options != nil {
		opts := make(map[string]any, len(req.Options))
		for k, v := range req.Options {
			opts[k] = v
		}
		cp.Options = opts
	}
	if len(req.Messages) > 0 {
		cp.Messages = append([]cometsdk.Message(nil), req.Messages...)
	}
	return &cp
}

func ensureJSONMode(p cometsdk.Provider, req *cometsdk.Request) {
	if req.Options == nil {
		req.Options = map[string]any{}
	}

	switch p.ID() {
	case "openai", "openai-compatible", "opencode-go":
		openaiOpts, _ := req.Options["openai"].(map[string]any)
		if openaiOpts == nil {
			openaiOpts = map[string]any{}
		}
		openaiOpts["response_format"] = map[string]any{"type": "json_object"}
		req.Options["openai"] = openaiOpts
	default:
		const hint = "\n\nRespond with a single valid JSON object only. No markdown fences or commentary."
		if strings.Contains(req.System, hint) {
			return
		}
		if strings.TrimSpace(req.System) == "" {
			req.System = strings.TrimSpace(hint)
		} else {
			req.System += hint
		}
	}
}

func extractJSON(text string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty response")
	}
	if m := jsonFenceRE.FindStringSubmatch(text); len(m) == 2 {
		text = strings.TrimSpace(m[1])
	}
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start >= 0 && end > start {
		text = text[start : end+1]
	}
	if !json.Valid([]byte(text)) {
		return nil, fmt.Errorf("invalid JSON: %q", truncate(text, 200))
	}
	return []byte(text), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
