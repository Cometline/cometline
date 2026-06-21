package tools

import (
	"context"
	"testing"

	"github.com/cometline/cometmind/internal/acp"
)

func TestNormalizeDelegationOutcomeCancelledByUser(t *testing.T) {
	t.Parallel()

	status, summary := normalizeDelegationOutcome(acp.TaskResult{Status: "cancelled"}, nil)
	if status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", status)
	}
	if summary != delegationCancelledByUser {
		t.Fatalf("summary = %q", summary)
	}

	status, summary = normalizeDelegationOutcome(acp.TaskResult{}, context.Canceled)
	if status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", status)
	}
	if summary != delegationCancelledByUser {
		t.Fatalf("summary = %q", summary)
	}
}
