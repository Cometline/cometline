package agent

import (
	"errors"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

// isContextOverflowError reports provider failures caused by prompt/context length.
func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	var stream *cometsdk.StreamError
	if errors.As(err, &stream) && stream.Cause != nil {
		err = stream.Cause
	}
	var server *cometsdk.ServerError
	if errors.As(err, &server) {
		if looksLikeContextOverflow(server.Message) || looksLikeContextOverflow(server.Error()) {
			return true
		}
	}
	return looksLikeContextOverflow(err.Error())
}

func looksLikeContextOverflow(msg string) bool {
	lower := strings.ToLower(msg)
	needles := []string{
		"context length",
		"context_length",
		"context window",
		"prompt is too long",
		"prompt too long",
		"maximum context",
		"max context",
		"max_tokens.*context", // unused; kept as documentation
		"too many tokens",
		"token limit",
		"exceeds the context",
		"exceeded model token",
		"input is too long",
		"input too long",
		"request too large",
		"context_length_exceeded",
		"string_above_max_length",
		"prompt tokens exceed",
		"maximum number of tokens",
	}
	for _, n := range needles {
		if strings.Contains(n, "*") {
			continue
		}
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}
