// Package retry provides exponential-backoff retry logic for HTTP requests,
// backed by github.com/cenkalti/backoff/v4.
package retry

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryAfterError is implemented by errors that carry an explicit retry delay
// (e.g. from a Retry-After HTTP header).
type RetryAfterError interface {
	error
	RetryAfter() time.Duration
}

// Do calls fn up to maxAttempts times using exponential backoff with jitter.
// It retries when fn returns an error for which isRetryable returns true.
// If the error implements RetryAfterError with a positive duration, that
// duration overrides the computed backoff for that attempt.
//
// Returns nil on success, or the last error if all attempts fail.
// Context cancellation stops retrying immediately.
func Do(ctx context.Context, maxAttempts int, fn func() error, isRetryable func(error) bool) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	// Build an exponential backoff: initial=1s, multiplier=2, max=30s, with jitter.
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = 1 * time.Second
	eb.Multiplier = 2
	eb.MaxInterval = 30 * time.Second
	eb.MaxElapsedTime = 0 // we control max attempts ourselves

	// Wrap with context and max attempt count.
	bo := backoff.WithContext(
		backoff.WithMaxRetries(eb, uint64(maxAttempts-1)),
		ctx,
	)

	var lastErr error
	operation := func() error {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if !isRetryable(lastErr) {
			// Wrap in PermanentError so backoff stops immediately.
			return backoff.Permanent(lastErr)
		}

		// If the error carries an explicit Retry-After, override the next interval.
		if ra, ok := lastErr.(RetryAfterError); ok {
			if d := ra.RetryAfter(); d > 0 {
				eb.InitialInterval = d
				eb.Reset()
			}
		}

		return lastErr
	}

	if err := backoff.Retry(operation, bo); err != nil {
		// Unwrap PermanentError so callers see the original error type.
		if pe, ok := err.(*backoff.PermanentError); ok {
			return pe.Err
		}
		// Context cancelled — surface ctx.Err() directly.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Max retries hit — return last known error.
		if lastErr != nil {
			return lastErr
		}
		return err
	}

	return nil
}
