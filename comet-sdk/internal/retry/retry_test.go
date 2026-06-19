package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cometline/comet-sdk/internal/retry"
	"github.com/stretchr/testify/require"
)

var errRetryable = errors.New("retryable error")
var errFatal = errors.New("fatal error")

func isRetryable(err error) bool {
	return errors.Is(err, errRetryable)
}

func TestDo_SucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), 4, func() error {
		calls++
		return nil
	}, isRetryable)

	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestDo_RetriesOnRetryableError(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return errRetryable
		}
		return nil
	}, isRetryable)

	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestDo_StopsOnNonRetryableError(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), 4, func() error {
		calls++
		return errFatal
	}, isRetryable)

	require.ErrorIs(t, err, errFatal)
	require.Equal(t, 1, calls) // no retries
}

func TestDo_ExhaustsAllAttempts(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), 3, func() error {
		calls++
		return errRetryable
	}, isRetryable)

	require.ErrorIs(t, err, errRetryable)
	require.Equal(t, 3, calls)
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := retry.Do(ctx, 10, func() error {
		calls++
		cancel() // cancel after first attempt
		return errRetryable
	}, isRetryable)

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

func TestDo_MaxAttemptsOne_NoRetry(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), 1, func() error {
		calls++
		return errRetryable
	}, isRetryable)

	require.ErrorIs(t, err, errRetryable)
	require.Equal(t, 1, calls)
}

// retryAfterError is a test error that implements RetryAfterError.
type retryAfterError struct {
	d time.Duration
}

func (e *retryAfterError) Error() string             { return "rate limited" }
func (e *retryAfterError) RetryAfter() time.Duration { return e.d }

func TestDo_UsesRetryAfterDuration(t *testing.T) {
	// We just verify the call succeeds eventually; we don't verify exact sleep
	// duration since that would make the test slow or require clock injection.
	calls := 0
	raErr := &retryAfterError{d: 1 * time.Millisecond}

	err := retry.Do(context.Background(), 3, func() error {
		calls++
		if calls < 2 {
			return raErr
		}
		return nil
	}, func(err error) bool { return true })

	require.NoError(t, err)
	require.Equal(t, 2, calls)
}
