package agent

import (
	"testing"

	"github.com/cometline/cometmind/internal/session"
)

func TestRunner_ReasoningEffortComesFromTurn(t *testing.T) {
	r := &Runner{}

	if got := r.reasoningEffortFor(session.AgentTurn{ReasoningEffort: "high"}); got != "high" {
		t.Fatalf("turn effort = %q, want high", got)
	}
	if got := r.reasoningEffortFor(session.AgentTurn{ReasoningEffort: "  medium  "}); got != "medium" {
		t.Fatalf("trimmed turn effort = %q, want medium", got)
	}
}

func TestRunner_ReasoningEffortEmptyWhenUnset(t *testing.T) {
	r := &Runner{}
	if got := r.reasoningEffortFor(session.AgentTurn{}); got != "" {
		t.Fatalf("effort = %q, want empty", got)
	}
}
