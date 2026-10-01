package agent

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf16"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"go.uber.org/zap"
)

const (
	maxStreamRecoveryAttempts    = 3
	defaultStreamRecoveryBackoff = 2 * time.Second
	maxStreamRecoveryBackoff     = 8 * time.Second
	timeoutContinueHint          = "The model timed out before finishing. Send another message to continue from here."
)

// waitToRetryStream backs off before retrying a recoverable stream failure.
// It returns true when the turn ended while waiting; the error is nil when the
// user stopped it.
func (r *Runner) waitToRetryStream(ctx context.Context, s *turnState, a *streamAttempt, result *llm.GenerateMessageResult, err error, attempt int) (bool, error) {
	textChars, reasoningChars := partialRenderLengths(result)
	delay := recoveryDelay(r.StreamRecoveryBackoff, attempt)
	if ra := retryAfterDelay(err); ra > delay {
		delay = ra
	}
	logging.L().Warn("agent.step.recover", zap.String("session", s.turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", s.turn.ModelID), zap.Int("step", s.steps+1), zap.String("failure_category", string(a.failureCategory)), zap.Int("recovery_attempt", attempt), zap.Int64("delay_ms", delay.Milliseconds()), zap.Int("text_chars", textChars), zap.Int("reasoning_chars", reasoningChars))
	s.ch <- event.TurnRecover(textChars, reasoningChars)
	waitErr := waitForRecovery(ctx, delay)
	if waitErr == nil {
		return false, nil
	}
	r.persistPartial(ctx, s, result)
	if isUserStop(ctx, waitErr) {
		r.logStepStopped(s, a)
		return true, nil
	}
	return true, r.reportStepFailure(s, a, attempt, waitErr)
}

func (r *Runner) persistPartial(ctx context.Context, s *turnState, result *llm.GenerateMessageResult) {
	persistPartialStep(ctx, r.Sessions, s.turn.ID, s.turn.ProviderID, s.turn.ModelID, result, s.pendingMemories)
}

func (r *Runner) logStepStopped(s *turnState, a *streamAttempt) {
	logging.L().Info("agent.step.stopped", zap.String("session", s.turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", s.turn.ModelID), zap.Int("step", s.steps+1), zap.Int64("duration_ms", time.Since(a.started).Milliseconds()))
}

func (r *Runner) reportStepFailure(s *turnState, a *streamAttempt, attempt int, err error) error {
	logging.L().Error("agent.step.failed", zap.String("session", s.turn.ID), zap.String("provider", r.Provider.ID()), zap.String("model", s.turn.ModelID), zap.Int("step", s.steps+1), zap.Int("events", a.events), zap.Bool("first_event", a.firstEvent), zap.Bool("first_output", a.firstOutput), zap.Bool("complete_tool_call", a.completeToolCall), zap.String("failure_category", string(a.failureCategory)), zap.Int("recovery_attempt", attempt), zap.Int64("duration_ms", time.Since(a.started).Milliseconds()), zap.Error(err))
	s.ch <- event.Errorf(userFacingAgentError(err), "llm")
	return err
}

// isUserStop reports whether err comes from cancelling the runner context
// itself rather than from a provider-side cancellation.
func isUserStop(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled)
}

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
