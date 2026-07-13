package server

import (
	"context"
	"testing"
	"time"
)

func TestUserFacingMessageError(t *testing.T) {
	t.Parallel()
	if got := userFacingMessageError(context.Canceled.Error()); got != "Response interrupted. Send the message again to continue." {
		t.Fatalf("canceled = %q", got)
	}
	if got := userFacingMessageError("openai: context canceled"); got != "Response interrupted. Send the message again to continue." {
		t.Fatalf("wrapped = %q", got)
	}
	if got := userFacingMessageError("something else"); got != "something else" {
		t.Fatalf("passthrough = %q", got)
	}
}

func TestMessagePersistenceContextStartsFreshAfterRequestDeadline(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	persistCtx, cancelPersist := messagePersistenceContext(requestCtx)
	defer cancelPersist()

	select {
	case <-persistCtx.Done():
		t.Fatalf("persistence context inherited expired request: %v", persistCtx.Err())
	case <-time.After(10 * time.Millisecond):
	}
}
