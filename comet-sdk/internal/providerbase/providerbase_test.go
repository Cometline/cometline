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
