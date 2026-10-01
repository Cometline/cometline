package responsesproto

import (
	"fmt"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestErrorClassifiersUnwrapServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		message  string
		classify func(error) bool
	}{
		{"max output tokens", "Unsupported parameter: max_output_tokens", IsMaxOutputTokensUnsupportedError},
		{"reasoning summary", "Unknown parameter: reasoning.summary", IsReasoningSummaryUnsupportedError},
		{"encrypted reasoning", "Invalid encrypted_content in input", IsEncryptedReasoningReplayError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			se := &cometsdk.ServerError{StatusCode: 400, Message: tt.message}
			if !tt.classify(se) {
				t.Fatal("bare 400 ServerError should match")
			}
			if !tt.classify(fmt.Errorf("codex: %w", se)) {
				t.Fatal("wrapped 400 ServerError should match")
			}
			if tt.classify(&cometsdk.ServerError{StatusCode: 500, Message: tt.message}) {
				t.Fatal("5xx ServerError should not match")
			}
		})
	}
}
