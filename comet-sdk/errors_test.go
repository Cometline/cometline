package cometsdk

import (
	"testing"
	"time"
)

// retryAfterReader mirrors the retry.RetryAfterError interface. We redeclare it
// here (rather than import internal/retry) to assert, without an import cycle,
// that *RateLimitError exposes the server-provided delay as a method.
type retryAfterReader interface {
	error
	RetryAfter() time.Duration
}

func TestRateLimitError_SatisfiesRetryAfter(t *testing.T) {
	var err error = &RateLimitError{ProviderID: "anthropic", RetryAfterDelay: 30 * time.Second}

	ra, ok := err.(retryAfterReader)
	if !ok {
		t.Fatalf("*RateLimitError does not satisfy the RetryAfter interface")
	}
	if got := ra.RetryAfter(); got != 30*time.Second {
		t.Fatalf("RetryAfter() = %v, want %v", got, 30*time.Second)
	}
}

func TestRateLimitError_ZeroDelay(t *testing.T) {
	e := &RateLimitError{ProviderID: "openai"}
	if got := e.RetryAfter(); got != 0 {
		t.Fatalf("RetryAfter() = %v, want 0", got)
	}
}
