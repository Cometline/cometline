package cometsdk

import "testing"

func TestNormalizeFinishReason(t *testing.T) {
	cases := map[string]string{
		// Passthrough: empty and already-canonical values.
		"":           "",
		"stop":       FinishStop,
		"tool_use":   FinishToolUse,
		"max_tokens": FinishMaxTokens,
		"error":      FinishError,
		// Anthropic stop_reason vocabulary.
		"end_turn":      FinishStop,
		"stop_sequence": FinishStop,
		// OpenAI finish_reason vocabulary.
		"tool_calls":     FinishToolUse,
		"length":         FinishMaxTokens,
		"content_filter": FinishStop,
		// Unknown reasons fall back to stop so the agent loop terminates.
		"something_new": FinishStop,
	}
	for raw, want := range cases {
		if got := NormalizeFinishReason(raw); got != want {
			t.Errorf("NormalizeFinishReason(%q) = %q, want %q", raw, got, want)
		}
	}
}
