package openai

import (
	"fmt"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
)

func TestFallbackClassifiersUnwrapServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		message  string
		classify func(error) bool
	}{
		{"image", "unknown variant `image_url`, expected `text`", isImageUnsupportedError},
		{"reasoning split", "Unrecognized request argument: reasoning_split", isReasoningSplitUnsupportedError},
		{"max tokens", "Unsupported parameter: 'max_tokens'. Use 'max_completion_tokens' instead.", isMaxTokensUnsupportedError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			se := &cometsdk.ServerError{StatusCode: 400, Message: tt.message}
			if !tt.classify(fmt.Errorf("openai: %w", se)) {
				t.Fatal("wrapped 400 ServerError should match")
			}
			if tt.classify(&cometsdk.ServerError{StatusCode: 503, Message: tt.message}) {
				t.Fatal("5xx ServerError should not match")
			}
		})
	}
}
