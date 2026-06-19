package acp_test

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/acp"
)

func TestSessionManagerInteractiveMultiTurn(t *testing.T) {
	t.Parallel()

	mgr := acp.NewSessionManager(acp.DefaultConfig())
	mgr.ProcessStarter = func(ctx context.Context, cfg acp.Config) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
		return acp.StartInteractiveFakeAgentPipes(ctx)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var awaiting acp.AwaitingInfo
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := mgr.Run(ctx, acp.RunOptions{
			ChildSessionID: "child-1",
			WorkspaceRoot:  t.TempDir(),
			Task:           "fix tests",
			Interactive:    true,
			OnAwaiting: func(info acp.AwaitingInfo) {
				awaiting = info
				_ = mgr.Respond("child-1", acp.RespondInput{Text: "feat/interactive"})
			},
		})
		if err != nil {
			t.Errorf("Run: %v", err)
			return
		}
		if result.Status != "completed" {
			t.Errorf("status = %q", result.Status)
		}
		if !strings.Contains(result.Summary, "feat/interactive") {
			t.Errorf("summary = %q", result.Summary)
		}
	}()

	wg.Wait()
	if awaiting.Kind != "input" {
		t.Fatalf("awaiting kind = %q", awaiting.Kind)
	}
	if !strings.Contains(awaiting.Question, "branch") {
		t.Fatalf("question = %q", awaiting.Question)
	}
}

func TestSessionManagerCancel(t *testing.T) {
	t.Parallel()

	mgr := acp.NewSessionManager(acp.DefaultConfig())
	mgr.ProcessStarter = func(ctx context.Context, cfg acp.Config) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
		return acp.StartInteractiveFakeAgentPipes(ctx)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = mgr.Run(ctx, acp.RunOptions{
			ChildSessionID: "child-2",
			WorkspaceRoot:  t.TempDir(),
			Task:           "long task",
			Interactive:    true,
			OnAwaiting: func(info acp.AwaitingInfo) {
				_ = mgr.Cancel("child-2")
			},
		})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not finish run")
	}
}
