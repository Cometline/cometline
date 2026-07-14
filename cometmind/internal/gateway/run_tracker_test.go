package gateway

import (
	"context"
	"testing"
)

func TestTurnRunTrackerStopCancelsActiveTurn(t *testing.T) {
	t.Parallel()

	tracker := NewTurnRunTracker()
	ctx, finish, err := tracker.Start(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	done, ok := tracker.Stop("session-1")
	if !ok {
		t.Fatal("Stop() = false, want true")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Stop() did not cancel the turn context")
	}
	select {
	case <-done:
		t.Fatal("turn reported cleanup complete before finish")
	default:
	}
	if _, _, err := tracker.Start(context.Background(), "session-1"); err == nil {
		t.Fatal("Start() succeeded before the prior turn finished cleanup")
	}
	finish()
	select {
	case <-done:
	default:
		t.Fatal("finish() did not report cleanup complete")
	}
	if _, ok := tracker.Stop("session-1"); ok {
		t.Fatal("second Stop() = true, want false")
	}
}

func TestTurnRunTrackerStopMissingTurn(t *testing.T) {
	t.Parallel()

	if _, ok := NewTurnRunTracker().Stop("missing"); ok {
		t.Fatal("Stop() = true for missing session")
	}
}
