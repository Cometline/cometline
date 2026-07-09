package server

import (
	"context"
	"testing"
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
