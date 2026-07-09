package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJobProgressTracker_NudgeAfterThreeTools(t *testing.T) {
	t.Parallel()
	tracker := &JobProgressTracker{JobID: "job-1", active: true, threshold: defaultJobProgressNudgeAfterTools}

	for i := 0; i < 2; i++ {
		if nudge := tracker.ObserveTool("read_file", json.RawMessage(`{"path":"a.go"}`)); nudge {
			t.Fatalf("ObserveTool #%d nudge = true, want false", i+1)
		}
	}
	if nudge := tracker.ObserveTool("read_file", json.RawMessage(`{"path":"b.go"}`)); !nudge {
		t.Fatal("third read_file should trigger nudge")
	}
	if tracker.SinceLastProgress != 0 {
		t.Fatalf("SinceLastProgress = %d, want 0 after nudge", tracker.SinceLastProgress)
	}
}

func TestJobProgressTracker_UpdateJobWithProgressResets(t *testing.T) {
	t.Parallel()
	tracker := &JobProgressTracker{JobID: "job-1", active: true, threshold: defaultJobProgressNudgeAfterTools}

	tracker.ObserveTool("read_file", json.RawMessage(`{"path":"a.go"}`))
	tracker.ObserveTool("read_file", json.RawMessage(`{"path":"b.go"}`))
	if nudge := tracker.ObserveTool("update_job", json.RawMessage(`{"job_id":"job-1","progress":"done step 1"}`)); nudge {
		t.Fatal("update_job with progress should reset, not nudge")
	}
	if tracker.SinceLastProgress != 0 {
		t.Fatalf("SinceLastProgress = %d, want 0", tracker.SinceLastProgress)
	}

	for i := 0; i < 2; i++ {
		tracker.ObserveTool("grep", json.RawMessage(`{"pattern":"foo"}`))
	}
	if nudge := tracker.ObserveTool("grep", json.RawMessage(`{"pattern":"bar"}`)); !nudge {
		t.Fatal("expected nudge after three more non-progress tools")
	}
}

func TestJobProgressTracker_UpdateJobWithoutProgressCounts(t *testing.T) {
	t.Parallel()
	tracker := &JobProgressTracker{JobID: "job-1", active: true, threshold: defaultJobProgressNudgeAfterTools}

	if nudge := tracker.ObserveTool("update_job", json.RawMessage(`{"job_id":"job-1"}`)); nudge {
		t.Fatal("update_job without progress should count as non-progress")
	}
	if tracker.SinceLastProgress != 1 {
		t.Fatalf("SinceLastProgress = %d, want 1", tracker.SinceLastProgress)
	}
}

func TestJobProgressTracker_CompleteJobDeactivates(t *testing.T) {
	t.Parallel()
	tracker := &JobProgressTracker{JobID: "job-1", active: true, threshold: defaultJobProgressNudgeAfterTools}

	tracker.ObserveTool("read_file", json.RawMessage(`{"path":"a.go"}`))
	tracker.ObserveTool("read_file", json.RawMessage(`{"path":"b.go"}`))
	if nudge := tracker.ObserveTool("complete_job", json.RawMessage(`{"job_id":"job-1"}`)); nudge {
		t.Fatal("complete_job should not nudge")
	}
	if tracker.active {
		t.Fatal("tracker should be inactive after complete_job")
	}
	if nudge := tracker.ObserveTool("read_file", json.RawMessage(`{"path":"c.go"}`)); nudge {
		t.Fatal("inactive tracker should not nudge")
	}
}

func TestFormatJobProgressNudgeBlock(t *testing.T) {
	t.Parallel()
	block := FormatJobProgressNudgeBlock("job-abc")
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	if !strings.Contains(block, "job-abc") {
		t.Fatalf("block = %q, want job id", block)
	}
	if !strings.Contains(block, "update_job") {
		t.Fatalf("block = %q, want update_job mention", block)
	}
}

func TestJobProgressTracker_CompletionGateBudget(t *testing.T) {
	t.Parallel()
	tracker := &JobProgressTracker{
		JobID:                "job-1",
		active:               true,
		completionGateBudget: defaultJobCompletionGateBudget,
	}
	if !tracker.NeedsCompletionGate() {
		t.Fatal("expected active job to need completion gate")
	}
	if !tracker.TryConsumeCompletionGate() {
		t.Fatal("first gate consume should succeed")
	}
	if !tracker.TryConsumeCompletionGate() {
		t.Fatal("second gate consume should succeed")
	}
	if tracker.TryConsumeCompletionGate() {
		t.Fatal("third gate consume should fail after budget")
	}

	tracker.ObserveTool("complete_job", json.RawMessage(`{"job_id":"job-1"}`))
	if tracker.NeedsCompletionGate() {
		t.Fatal("gate should be inactive after complete_job")
	}
	if tracker.TryConsumeCompletionGate() {
		t.Fatal("inactive tracker must not consume gate")
	}
}

func TestFormatJobCompletionGateBlock(t *testing.T) {
	t.Parallel()
	block := FormatJobCompletionGateBlock("job-xyz")
	if block == "" {
		t.Fatal("expected non-empty block")
	}
	if !strings.Contains(block, "job-xyz") {
		t.Fatalf("block = %q, want job id", block)
	}
	if !strings.Contains(block, "complete_job") || !strings.Contains(block, "release_job") {
		t.Fatalf("block = %q, want terminal tools", block)
	}
	if FormatJobCompletionGateBlock("") != "" {
		t.Fatal("empty job id should yield empty block")
	}
}
