package agent

import (
	"testing"
	"time"
)

func TestDecideSkillReview(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	previous := skillTargets{Paths: []string{"src/main.go"}, Commands: []string{"go test ./..."}}

	tests := []struct {
		name    string
		in      skillReviewInput
		start   bool
		reason  string
		logSkip string
		wide    bool
	}{
		{
			name: "eight mutating calls",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				Calls: mutatingCalls(8, "edit_file", "src/main.go", ""),
			},
			start:  true,
			reason: "work",
		},
		{
			name: "eight reads do not start",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				Calls: mutatingCalls(8, "read_file", "src/main.go", ""),
			},
		},
		{
			name: "seven mutating calls do not start",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				Calls: mutatingCalls(7, "write_file", "src/main.go", ""),
			},
		},
		{
			name: "failed mutating calls do not count",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				Calls: failedCalls(8, "edit_file", "src/main.go"),
			},
		},
		{
			name: "correction redo below threshold",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				UserText: "please fix src/main.go again",
				Previous: previous,
				Calls:    mutatingCalls(1, "edit_file", "src/main.go", ""),
			},
			start:  true,
			reason: "correction",
		},
		{
			name: "correction mention without a redo does not start",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				UserText: "please fix src/main.go again",
				Previous: previous,
				Calls:    mutatingCalls(1, "edit_file", "other.go", ""),
			},
		},
		{
			name: "unrelated user text does not arm",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				UserText: "that was wrong, redo it",
				Previous: previous,
				Calls:    mutatingCalls(1, "edit_file", "src/main.go", ""),
			},
		},
		{
			name: "command rerun arms",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				UserText: "run go test ./... once more",
				Previous: previous,
				Calls:    mutatingCalls(1, "run_command", "", "go test ./..."),
			},
			start:  true,
			reason: "correction",
		},
		{
			name: "cooldown blocks an eligible turn",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				LastStarted: now.Add(-14 * time.Minute),
				Calls:       mutatingCalls(8, "edit_file", "src/main.go", ""),
			},
		},
		{
			name: "cooldown expires",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				LastStarted: now.Add(-15 * time.Minute),
				Calls:       mutatingCalls(8, "edit_file", "src/main.go", ""),
			},
			start:  true,
			reason: "work",
		},
		{
			name: "child session does not start",
			in: skillReviewInput{
				UserChat: false, HasModel: true, Now: now,
				Calls: mutatingCalls(8, "edit_file", "src/main.go", ""),
			},
		},
		{
			name: "missing extraction model skips",
			in: skillReviewInput{
				UserChat: true, HasModel: false, Now: now,
				Calls: mutatingCalls(8, "edit_file", "src/main.go", ""),
			},
			logSkip: "skills.review.skipped_no_model",
		},
		{
			name: "missing model below threshold does not log",
			in: skillReviewInput{
				UserChat: true, HasModel: false, Now: now,
				Calls: mutatingCalls(1, "edit_file", "src/main.go", ""),
			},
		},
		{
			name: "accumulated ten starts with the wider window",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				Accumulated: 10,
				Calls:       mutatingCalls(4, "edit_file", "src/main.go", ""),
			},
			start:  true,
			reason: "accumulated",
			wide:   true,
		},
		{
			name: "eight calls inside an accumulated window stay wide",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				Accumulated: 12,
				Calls:       mutatingCalls(8, "edit_file", "src/main.go", ""),
			},
			start:  true,
			reason: "accumulated",
			wide:   true,
		},
		{
			name: "accumulated ten during cooldown does not reset",
			in: skillReviewInput{
				UserChat: true, HasModel: true, Now: now,
				LastStarted: now.Add(-5 * time.Minute),
				Accumulated: 10,
				Calls:       mutatingCalls(1, "edit_file", "src/main.go", ""),
			},
		},
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

func TestTargetsFromCallsKeepSuccessfulMutations(t *testing.T) {
	got := targetsFromCalls([]skillCall{
		{Name: "edit_file", Path: "./src/main.go", OK: true},
		{Name: "read_file", Path: "src/other.go", OK: true},
		{Name: "run_command", Command: "go test ./...", OK: true},
		{Name: "write_file", Path: "src/main.go", OK: false},
	})
	if len(got.Paths) != 1 || got.Paths[0] != "src/main.go" {
		t.Fatalf("paths = %#v", got.Paths)
	}
	if len(got.Commands) != 1 || got.Commands[0] != "go test ./..." {
		t.Fatalf("commands = %#v", got.Commands)
	}
}

func mutatingCalls(n int, name, path, command string) []skillCall {
	out := make([]skillCall, n)
	for i := range out {
		out[i] = skillCall{Name: name, Path: path, Command: command, OK: true}
	}
	return out
}

func failedCalls(n int, name, path string) []skillCall {
	out := mutatingCalls(n, name, path, "")
	for i := range out {
		out[i].OK = false
	}
	return out
}
