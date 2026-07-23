package agent

import (
	"fmt"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestIsContextOverflowError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{fmt.Errorf("provider exploded"), false},
		{fmt.Errorf("prompt is too long"), true},
		{fmt.Errorf("Request exceeds maximum context length"), true},
		{&cometsdk.ServerError{ProviderID: "openai", StatusCode: 400, Message: "context_length_exceeded"}, true},
		{&cometsdk.StreamError{ProviderID: "anthropic", Cause: fmt.Errorf("input is too long")}, true},
		{&cometsdk.ServerError{ProviderID: "x", StatusCode: 400, Message: "invalid api key"}, false},
	}
	for _, tc := range cases {
		if got := isContextOverflowError(tc.err); got != tc.want {
			t.Fatalf("isContextOverflowError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
