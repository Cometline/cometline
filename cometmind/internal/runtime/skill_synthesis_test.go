package runtime

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/skills"
)

type synthesisTestProvider struct {
	text string
}

func (p synthesisTestProvider) ID() string { return "static" }

func (p synthesisTestProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, 3)
	go func() {
		defer close(ch)
		select {
		case <-ctx.Done():
			ch <- cometsdk.ErrorEvent{Err: ctx.Err()}
			ch <- cometsdk.DoneEvent{}
			return
		default:
		}
		ch <- cometsdk.TextDeltaEvent{Text: p.text}
		ch <- cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop}
		ch <- cometsdk.DoneEvent{}
	}()
	return ch, nil
}

func TestSkillSynthesisNotifierDetachesFromCallerContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	n := &skillSynthesisNotifier{
		provider: synthesisTestProvider{text: `{"should_propose":true,"name":"retry-policy","content":"---\nname: retry-policy\ndescription: Handle retry policy work\n---\n\n## Trigger\nUse for retry policy work.","reason":"reusable workflow"}`},
		model:    "test-model",
	}
	ctx, cancel := context.WithCancel(context.Background())
	n.OnJobEvent(ctx, jobs.Job{
		ID:               "job-1",
		Description:      "Implement a retry policy",
		DefinitionOfDone: "Retries are bounded",
		Progress:         "Added blocked state",
	}, jobs.EventCompleted, "")
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _, err := skills.DraftMarkdown("retry-policy")
		if err == nil {
			return
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("DraftMarkdown() unexpected error = %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	_, _, err := skills.DraftMarkdown("retry-policy")
	t.Fatalf("expected synthesized draft despite caller cancellation, got err=%v", err)
}
