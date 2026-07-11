package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "network timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}

func TestClassifyStreamFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want streamFailureCategory
	}{
		{"timeout", timeoutError{}, streamFailureRecoverable},
		{"unexpected eof", io.ErrUnexpectedEOF, streamFailureRecoverable},
		{"connection reset", syscall.ECONNRESET, streamFailureRecoverable},
		{"cancelled", context.Canceled, streamFailureCancelled},
		{"auth", &cometsdk.AuthError{ProviderID: "x", StatusCode: 401}, streamFailureAuth},
		{"http 400", &cometsdk.ServerError{ProviderID: "x", StatusCode: 400}, streamFailureHTTP4xx},
		{"rate limit", &cometsdk.RateLimitError{ProviderID: "x"}, streamFailureHTTP4xx},
		{"other", errors.New("bad response"), streamFailureOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyStreamFailure(tc.err); got != tc.want {
				t.Fatalf("classifyStreamFailure(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}
