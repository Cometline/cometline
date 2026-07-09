package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/jobs"
)

const defaultJobProgressNudgeAfterTools = 3

// defaultJobCompletionGateBudget is how many times the runner may refuse a
// text-only stop and force another step while a session still holds an
// ongoing job without complete_job/release_job.
const defaultJobCompletionGateBudget = 2

// OngoingJobLookup resolves the ongoing job assigned to a session, if any.
type OngoingJobLookup interface {
	JobForSession(ctx context.Context, sessionID string) (jobs.Job, bool, error)
}

// JobProgressTracker nudges the model to call update_job after several tool
// calls without a progress write during an ongoing job turn, and enforces a
// terminal job tool before a text-only stop is accepted.
type JobProgressTracker struct {
	JobID                string
	SinceLastProgress    int
	active               bool
	threshold            int
	completionGateBudget int
	completionGateUsed   int
}

func newJobProgressTracker(ctx context.Context, lookup OngoingJobLookup, sessionID string) *JobProgressTracker {
	t := &JobProgressTracker{
		threshold:            defaultJobProgressNudgeAfterTools,
		completionGateBudget: defaultJobCompletionGateBudget,
	}
	if lookup == nil || strings.TrimSpace(sessionID) == "" {
		return t
	}
	job, ok, err := lookup.JobForSession(ctx, sessionID)
	if err != nil || !ok {
		return t
	}
	t.JobID = job.ID
	t.active = true
	return t
}

// NeedsCompletionGate reports whether the turn still holds an ongoing job
// that has not yet been completed or released via a terminal job tool.
func (t *JobProgressTracker) NeedsCompletionGate() bool {
	return t != nil && t.active && strings.TrimSpace(t.JobID) != ""
}

// TryConsumeCompletionGate returns true when the runner should refuse a
// text-only stop and inject a completion-gate system block for another step.
// Returns false when no job is held or the per-turn budget is exhausted.
func (t *JobProgressTracker) TryConsumeCompletionGate() bool {
	if !t.NeedsCompletionGate() {
		return false
	}
	if t.completionGateBudget <= 0 {
		t.completionGateBudget = defaultJobCompletionGateBudget
	}
	if t.completionGateUsed >= t.completionGateBudget {
		return false
	}
	t.completionGateUsed++
	return true
}

// ObserveTool records a tool call. It returns true when a progress nudge should
// be injected into the next step's system prompt.
func (t *JobProgressTracker) ObserveTool(name string, input json.RawMessage) bool {
	if t == nil || !t.active || t.JobID == "" {
		return false
	}
	switch name {
	case "complete_job", "release_job":
		t.active = false
		return false
	case "update_job":
		if updateJobHasProgress(input) {
			t.SinceLastProgress = 0
		} else {
			t.SinceLastProgress++
		}
	default:
		t.SinceLastProgress++
	}
	if t.SinceLastProgress < t.threshold {
		return false
	}
	t.SinceLastProgress = 0
	return true
}

func updateJobHasProgress(input json.RawMessage) bool {
	if len(input) == 0 {
		return false
	}
	var in struct {
		Progress string `json:"progress"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	return strings.TrimSpace(in.Progress) != ""
}

// FormatJobProgressNudgeBlock returns a system-prompt block reminding the
// model to persist job progress.
func FormatJobProgressNudgeBlock(jobID string) string {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return ""
	}
	return fmt.Sprintf(
		"This session is working job %q. You have run several tools without updating job progress. Call `update_job` with a brief `progress` summary of what is done and what remains before continuing.",
		jobID,
	)
}

// FormatJobCompletionGateBlock returns a system-prompt block that forces a
// terminal job tool before the turn may end.
func FormatJobCompletionGateBlock(jobID string) string {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return ""
	}
	return fmt.Sprintf(
		"PROTOCOL: This session still holds job %q. You may not end this turn with text only. Call `complete_job` with a final progress summary if Definition of Done is met, or `release_job` with a reason if it is not. Ending without one of those tools is a protocol violation.",
		jobID,
	)
}
