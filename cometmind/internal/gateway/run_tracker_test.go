package gateway

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/runstate"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
)

func newRunTrackerTest(t *testing.T) (*TurnRunTracker, string) {
	t.Helper()
	database, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	sessions := session.New(database)
	workspace, err := sessions.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.NewSession(context.Background(), workspace.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	return NewTurnRunTracker(runstate.New(database)), sess.ID
}

func TestTurnRunTrackerStopCancelsActiveTurn(t *testing.T) {
	t.Parallel()

	tracker, sessionID := newRunTrackerTest(t)
	ctx, finish, err := tracker.Start(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	done, ok := tracker.Stop(sessionID)
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
	if _, _, err := tracker.Start(context.Background(), sessionID); err == nil {
		t.Fatal("Start() succeeded before the prior turn finished cleanup")
	}
	finish()
	select {
	case <-done:
	default:
		t.Fatal("finish() did not report cleanup complete")
	}
	if _, ok := tracker.Stop(sessionID); ok {
		t.Fatal("second Stop() = true, want false")
	}
}

func TestTurnRunTrackerStopMissingTurn(t *testing.T) {
	t.Parallel()

	tracker, _ := newRunTrackerTest(t)
	if _, ok := tracker.Stop("missing"); ok {
		t.Fatal("Stop() = true for missing session")
	}
}
