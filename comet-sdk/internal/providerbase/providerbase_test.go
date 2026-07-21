package providerbase

import (
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
