package responsesproto

import (
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

// IsMaxOutputTokensUnsupportedError reports whether err is a 4xx ServerError
// whose message says max_output_tokens is rejected.
func IsMaxOutputTokensUnsupportedError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok {
		return false
	}
	if se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	return strings.Contains(msg, "max_output_tokens") &&
		(strings.Contains(msg, "unsupported") || strings.Contains(msg, "unknown") || strings.Contains(msg, "invalid"))
}

// IsReasoningSummaryUnsupportedError reports whether err is a 4xx ServerError
// whose message says the reasoning summary is rejected.
func IsReasoningSummaryUnsupportedError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok || se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	return strings.Contains(msg, "reasoning") &&
		strings.Contains(msg, "summary") &&
		(strings.Contains(msg, "unsupported") || strings.Contains(msg, "unknown") || strings.Contains(msg, "invalid"))
}

// IsEncryptedReasoningReplayError reports whether err is a 4xx ServerError
// caused by replaying encrypted reasoning state.
func IsEncryptedReasoningReplayError(err error) bool {
	se, ok := err.(*cometsdk.ServerError)
	if !ok || se.StatusCode < 400 || se.StatusCode >= 500 {
		return false
	}
	msg := strings.ToLower(se.Message)
	if strings.Contains(msg, "encrypted_content") || strings.Contains(msg, "encrypted content") {
		return true
	}
	// Providers reject reasoning items that omit summary when encrypted state is replayed.
	return strings.Contains(msg, "input[") &&
		strings.Contains(msg, ".summary") &&
		strings.Contains(msg, "missing")
}
