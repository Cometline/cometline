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

	if !tracker.Stop("session-1") {
		t.Fatal("Stop() = false, want true")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("Stop() did not cancel the turn context")
	}
	finish()
	if tracker.Stop("session-1") {
		t.Fatal("second Stop() = true, want false")
	}
}

func TestTurnRunTrackerStopMissingTurn(t *testing.T) {
	t.Parallel()

	if NewTurnRunTracker().Stop("missing") {
		t.Fatal("Stop() = true for missing session")
	}
}
