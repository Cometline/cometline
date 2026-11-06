package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
)

// errDroppedProviderStream is a provider-agnostic stand-in for a stream that
// opened and closed without a finish reason. HTTP 200 plus an empty body is
// not a finished turn. A real finish reason, including an empty stop, is.
var errDroppedProviderStream = errors.New("provider stream ended without a completion")

type streamFailureCategory string

const (
	streamFailureRecoverable streamFailureCategory = "recoverable_transport"
	streamFailureCancelled   streamFailureCategory = "cancelled"
	streamFailureAuth        streamFailureCategory = "authentication"
	streamFailureHTTP4xx     streamFailureCategory = "http_4xx"
	streamFailureOther       streamFailureCategory = "other"
)

// classifyStreamFailure keeps recovery policy independent of provider names.
// User cancellation and auth failures stay terminal. Transient gateway pressure
// (429, 5xx, timeouts) is recoverable so a later model step can continue after
// tools have already finished.
func classifyStreamFailure(err error) streamFailureCategory {
	if err == nil {
		return streamFailureOther
	}
	if errors.Is(err, errDroppedProviderStream) {
		return streamFailureRecoverable
	}
	if errors.Is(err, context.Canceled) {
		return streamFailureCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return streamFailureRecoverable
	}
	var auth *cometsdk.AuthError
	if errors.As(err, &auth) {
		return streamFailureAuth
	}
	var rateLimit *cometsdk.RateLimitError
	if errors.As(err, &rateLimit) {
		return streamFailureRecoverable
	}
	var server *cometsdk.ServerError
	if errors.As(err, &server) {
		switch server.StatusCode {
		case 429, 500, 502, 503, 504, 529:
			return streamFailureRecoverable
		}
		if server.StatusCode >= 400 && server.StatusCode < 500 {
			return streamFailureHTTP4xx
		}
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
		return streamFailureRecoverable
	}
	var netErr net.Error
	//nolint:staticcheck // SA1019: some net errors still only report Temporary(); dropping it would change retry classification.
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return streamFailureRecoverable
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "connection reset") || strings.Contains(lower, "unexpected eof") ||
		strings.Contains(lower, "connection aborted") || strings.Contains(lower, "broken pipe") ||
		strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return streamFailureRecoverable
	}
	return streamFailureOther
}

func recoverableStreamFailure(err error) bool {
	return classifyStreamFailure(err) == streamFailureRecoverable
}

// droppedProviderStream reports a stream that ended without a finish reason.
// It is independent of provider: any adapter can return HTTP 200 and then close
// the body before a completion event. A finish reason is a real completion,
// even when the model sent no text and reported no usage.
func droppedProviderStream(result *llm.GenerateMessageResult, a *streamAttempt) bool {
	if a != nil && (a.completeToolCall || len(a.toolCalls) > 0) {
		return false
	}
	if result == nil {
		return true
	}
	if len(result.ToolCalls) > 0 {
		return false
	}
	return result.FinishReason == ""
}
