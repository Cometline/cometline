package providerbase

import (
	"errors"
	"fmt"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestIsRetryableGatewayTimeout(t *testing.T) {
	t.Parallel()

	err := &cometsdk.ServerError{StatusCode: 504, Message: "Gateway Timeout"}
	if !IsRetryable(err) {
		t.Fatal("HTTP 504 should be retried")
	}
}

func TestIsRetryableHTTP400(t *testing.T) {
	t.Parallel()

	err := &cometsdk.ServerError{StatusCode: 400, Message: "Upstream request failed"}
	if !IsRetryable(err) {
		t.Fatal("HTTP 400 should be retried")
	}
}

func TestIsRetryableHTTP404(t *testing.T) {
	t.Parallel()

	err := &cometsdk.ServerError{StatusCode: 404, Message: "not found"}
	if IsRetryable(err) {
		t.Fatal("HTTP 404 should not be retried")
	}
}

func TestIsRetryableWrappedRateLimit(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("codex: request: %w", &cometsdk.RateLimitError{ProviderID: "codex"})
	if !IsRetryable(err) {
		t.Fatal("wrapped RateLimitError should be retried")
	}
}

func TestIsRetryableWrappedServerError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("request: %w", &cometsdk.ServerError{StatusCode: 503, Message: "unavailable"})
	if !IsRetryable(err) {
		t.Fatal("wrapped HTTP 503 should be retried")
	}
}

func TestIsRetryableWrappedAuthError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("request: %w", &cometsdk.AuthError{StatusCode: 401})
	if IsRetryable(err) {
		t.Fatal("wrapped AuthError should not be retried")
	}
}

func TestClientServerError(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("request: %w", &cometsdk.ServerError{StatusCode: 400, Message: "bad"})
	se, ok := ClientServerError(wrapped)
	if !ok || se.Message != "bad" {
		t.Fatalf("ClientServerError(wrapped 400) = %v, %v; want message %q", se, ok, "bad")
	}

	if _, ok := ClientServerError(&cometsdk.ServerError{StatusCode: 500}); ok {
		t.Fatal("HTTP 500 is not a client error")
	}
	if _, ok := ClientServerError(errors.New("plain")); ok {
		t.Fatal("plain error is not a ServerError")
	}
}
