package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
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
		{"deadline exceeded", context.DeadlineExceeded, streamFailureRecoverable},
		{"unexpected eof", io.ErrUnexpectedEOF, streamFailureRecoverable},
		{"connection reset", syscall.ECONNRESET, streamFailureRecoverable},
		{"cancelled", context.Canceled, streamFailureCancelled},
		{"auth", &cometsdk.AuthError{ProviderID: "x", StatusCode: 401}, streamFailureAuth},
		{"http 400", &cometsdk.ServerError{ProviderID: "x", StatusCode: 400}, streamFailureHTTP4xx},
		{"rate limit", &cometsdk.RateLimitError{ProviderID: "x"}, streamFailureRecoverable},
		{"gateway timeout", &cometsdk.ServerError{ProviderID: "x", StatusCode: 504, Message: "Gateway Timeout"}, streamFailureRecoverable},
		{"timeout text", errors.New("upstream timeout"), streamFailureRecoverable},
		{"other", errors.New("bad response"), streamFailureOther},
		{"dropped stream", errDroppedProviderStream, streamFailureRecoverable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyStreamFailure(tc.err); got != tc.want {
				t.Fatalf("classifyStreamFailure(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestDroppedProviderStream(t *testing.T) {
	t.Parallel()
	text := &llm.GenerateMessageResult{
		Message:      cometsdk.Message{Content: []cometsdk.Block{cometsdk.TextBlock{Text: "done"}}},
		FinishReason: cometsdk.FinishStop,
	}
	usedEmpty := &llm.GenerateMessageResult{
		FinishReason: cometsdk.FinishStop,
		Usage:        cometsdk.TokenUsage{InputTokens: 20},
	}
	syntheticStop := &llm.GenerateMessageResult{FinishReason: cometsdk.FinishStop}
	empty := &llm.GenerateMessageResult{}
	withTool := &llm.GenerateMessageResult{
		ToolCalls: []cometsdk.ToolCallBlock{{ID: "tc", Name: "read_file"}},
	}
	cases := []struct {
		name    string
		result  *llm.GenerateMessageResult
		attempt *streamAttempt
		want    bool
	}{
		{"nil result", nil, &streamAttempt{}, true},
		{"no events and no finish", empty, &streamAttempt{}, true},
		{"bare stop without usage", syntheticStop, &streamAttempt{events: 1}, false},
		{"real empty answer with usage", usedEmpty, &streamAttempt{events: 1}, false},
		{"text completion", text, &streamAttempt{events: 2, firstOutput: true}, false},
		{"completed tool call", empty, &streamAttempt{completeToolCall: true, toolCalls: []cometsdk.ToolCallBlock{{ID: "tc"}}}, false},
		{"result tool calls", withTool, &streamAttempt{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := droppedProviderStream(tc.result, tc.attempt); got != tc.want {
				t.Fatalf("droppedProviderStream() = %v, want %v", got, tc.want)
			}
		})
	}
}
