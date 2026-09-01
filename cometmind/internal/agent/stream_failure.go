package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	cometsdk "github.com/cometline/comet-sdk"
)

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
