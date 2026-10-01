package agent

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf16"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
)

const (
	maxStreamRecoveryAttempts    = 3
	defaultStreamRecoveryBackoff = 2 * time.Second
	maxStreamRecoveryBackoff     = 8 * time.Second
	timeoutContinueHint          = "The model timed out before finishing. Send another message to continue from here."
)

func partialRenderLengths(result *llm.GenerateMessageResult) (textChars, reasoningChars int) {
	if result == nil {
		return 0, 0
	}
	textChars = len(utf16.Encode([]rune(assistantPlainText(result.Message))))
	for _, block := range result.Message.ReasoningContent {
		switch value := block.(type) {
		case cometsdk.ReasoningBlock:
			reasoningChars += len(utf16.Encode([]rune(value.Text)))
		case cometsdk.TextBlock:
			reasoningChars += len(utf16.Encode([]rune(value.Text)))
		}
	}
	return textChars, reasoningChars
}

// userFacingAgentError maps provider/runtime errors into short messages safe
// to show in the chat transcript. Context cancel (client abort or disconnect)
// is the most common silent-empty-turn cause for mini models.
func userFacingAgentError(err error) string {
	if err == nil {
		return "The request failed."
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if errors.Is(err, context.DeadlineExceeded) {
			return timeoutContinueHint
		}
		return "Response interrupted. Send the message again to continue."
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "The request failed."
	}
	// Some providers wrap cancel without satisfying errors.Is.
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "context canceled") || strings.Contains(lower, "context cancelled") {
		return "Response interrupted. Send the message again to continue."
	}
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return timeoutContinueHint
	}
	return msg
}

func recoveryDelay(base time.Duration, attempt int) time.Duration {
	if base <= 0 {
		base = defaultStreamRecoveryBackoff
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := base * time.Duration(1<<uint(attempt-1))
	if delay > maxStreamRecoveryBackoff {
		return maxStreamRecoveryBackoff
	}
	return delay
}

func retryAfterDelay(err error) time.Duration {
	var rateLimit *cometsdk.RateLimitError
	if errors.As(err, &rateLimit) {
		return rateLimit.RetryAfterDelay
	}
	return 0
}

func waitForRecovery(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
