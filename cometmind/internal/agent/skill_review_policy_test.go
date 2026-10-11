package agent

import (
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/db"
)

func TestDecideSkillReview(t *testing.T) {
	tests := []struct {
		name    string
		in      skillReviewInput
		start   bool
		reason  string
		logSkip string
		wide    bool
	}{
		{name: "ninth iteration does not start", in: skillReviewInput{UserChat: true, HasModel: true, Iterations: 9}},
		{
			name:   "tenth tool iteration",
			in:     skillReviewInput{UserChat: true, HasModel: true, Iterations: 10},
			start:  true,
			reason: "iterations",
			wide:   true,
		},
		{name: "child session does not start", in: skillReviewInput{UserChat: false, HasModel: true, Iterations: 10}},
		{
			name:    "missing extraction model skips",
			in:      skillReviewInput{UserChat: true, HasModel: false, Iterations: 10},
			logSkip: "skills.review.skipped_no_model",
		},
		{name: "missing model below threshold does not log", in: skillReviewInput{UserChat: true, HasModel: false, Iterations: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideSkillReview(tt.in)
			if got.Start != tt.start || got.Reason != tt.reason || got.LogSkip != tt.logSkip || got.WideWindow != tt.wide {
				t.Fatalf("decision = %+v, want start=%v reason=%q log=%q wide=%v", got, tt.start, tt.reason, tt.logSkip, tt.wide)
			}
		})
	}
}

func TestToolCallingIterationsCountStepsNotCalls(t *testing.T) {
	rows := []db.Message{
		{ID: "a1", Role: "assistant"},
		{ID: "a2", Role: "assistant"},
		{ID: "a3", Role: "assistant"},
	}
	calls := []db.ToolCall{
		{ID: "c1", MessageID: "a1", ToolName: "read_file"},
		{ID: "c2", MessageID: "a1", ToolName: "grep"},
		{ID: "c3", MessageID: "a2", ToolName: "edit_file"},
	}
	if got := toolCallingIterations(calls, rows); got != 2 {
		t.Fatalf("iterations = %d, want 2", got)
	}
	if got := toolCallingIterations(nil, rows); got != 0 {
		t.Fatalf("empty iterations = %d", got)
	}
}
