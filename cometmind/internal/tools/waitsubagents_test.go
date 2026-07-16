package tools

import (
	"context"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/subagent"
)

func TestWaitSubagentsWaitsUntilChildFinishes(t *testing.T) {
	orchestrator := subagent.NewOrchestrator(1)
	_, cancelChild := context.WithCancel(context.Background())
	defer cancelChild()
	if err := orchestrator.Register("parent", "child", subagent.KindGeneral, cancelChild); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	resultCh := make(chan Result, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := (WaitSubagents{
			Sessions:     &session.Service{},
			Orchestrator: orchestrator,
		}).Execute(WithToolSession(context.Background(), "parent"), []byte(`{"timeout_seconds":1}`))
		resultCh <- result
		errCh <- err
	}()

	select {
	case result := <-resultCh:
		t.Fatalf("Execute() returned before child completed: %+v", result)
	case err := <-errCh:
		t.Fatalf("Execute() returned before child completed: %v", err)
	case <-time.After(1200 * time.Millisecond):
	}

	orchestrator.Complete("child", subagent.Result{Status: "completed", Summary: "finished"})
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Execute() did not return after child completed")
	}
	result := <-resultCh
	if !result.OK || result.Output == "" {
		t.Fatalf("Execute() result = %+v, want completed child result", result)
	}
}
