package cometsdk

import (
	"fmt"
	"time"
)

// AuthError is returned when the API key is invalid or missing (HTTP 401).
type AuthError struct {
	ProviderID string
	StatusCode int
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("cometsdk: %s: authentication failed (HTTP %d)", e.ProviderID, e.StatusCode)
}

// RateLimitError is returned on HTTP 429.
// RetryAfterDelay indicates how long to wait before retrying (0 if not provided
// by the provider). It is exposed to the retry layer via the RetryAfter method.
type RateLimitError struct {
	ProviderID      string
	RetryAfterDelay time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfterDelay > 0 {
		return fmt.Sprintf("cometsdk: %s: rate limited, retry after %s", e.ProviderID, e.RetryAfterDelay)
	}
	return fmt.Sprintf("cometsdk: %s: rate limited", e.ProviderID)
}

// RetryAfter reports the server-provided retry delay so the retry layer can
// honour a 429 Retry-After header instead of falling back to plain exponential
// backoff. Satisfies the retry.RetryAfterError interface.
func (e *RateLimitError) RetryAfter() time.Duration { return e.RetryAfterDelay }

// ServerError is returned on HTTP 5xx responses.
type ServerError struct {
	ProviderID string
	StatusCode int
	Message    string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("cometsdk: %s: server error (HTTP %d): %s", e.ProviderID, e.StatusCode, e.Message)
}

// StreamError wraps an error that occurred mid-stream (after HTTP 200 was received).
type StreamError struct {
	ProviderID string
	Cause      error
}

func (e *StreamError) Error() string {
	return fmt.Sprintf("cometsdk: %s: stream error: %v", e.ProviderID, e.Cause)
}

func (e *StreamError) Unwrap() error {
	return e.Cause
}
