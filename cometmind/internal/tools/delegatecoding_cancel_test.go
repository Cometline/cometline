package tools

import (
	"context"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/acp"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestNormalizeDelegationOutcomeCancelledByUser(t *testing.T) {
	t.Parallel()

	status, summary := normalizeDelegationOutcome(acp.TaskResult{Status: session.DelegationCancelled.String()}, nil)
	if status != session.DelegationCancelled {
		t.Fatalf("status = %q, want cancelled", status)
	}
	if summary != delegationCancelledByUser {
		t.Fatalf("summary = %q", summary)
	}

	status, summary = normalizeDelegationOutcome(acp.TaskResult{}, context.Canceled)
	if status != session.DelegationCancelled {
		t.Fatalf("status = %q, want cancelled", status)
	}
	if summary != delegationCancelledByUser {
		t.Fatalf("summary = %q", summary)
	}
}
