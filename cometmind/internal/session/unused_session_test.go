package session

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestPruneUnusedUserSessions(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	svc := New(conn)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	unused, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	used, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendUserMessage(ctx, used.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	cleared, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendUserMessage(ctx, cleared.ID, "clear me"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearSessionTranscript(ctx, cleared.ID); err != nil {
		t.Fatal(err)
	}
	configuredModel, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateSessionModel(ctx, configuredModel.ID, "other-model", "other-provider"); err != nil {
		t.Fatal(err)
	}
	configuredMode, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateSessionAgentMode(ctx, configuredMode.ID, AgentModePlan); err != nil {
		t.Fatal(err)
	}
	configuredPinned, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateSessionPinned(ctx, configuredPinned.ID, true); err != nil {
		t.Fatal(err)
	}
	configuredTitle, err := svc.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateSessionTitle(ctx, configuredTitle.ID, "planned chat"); err != nil {
		t.Fatal(err)
	}
	child, err := svc.NewChildSession(ctx, used, "task", "general")
	if err != nil {
		t.Fatal(err)
	}
	autonomous, err := svc.NewAutonomySession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := svc.NewInboxSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}

	pruned, err := svc.PruneUnusedUserSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 {
		t.Fatalf("PruneUnusedUserSessions() = %d, want 1", pruned)
	}
	if _, err := svc.GetSession(ctx, unused.ID); err == nil {
		t.Fatal("expected unused session to be deleted")
	}
	for _, id := range []string{
		used.ID,
		cleared.ID,
		configuredModel.ID,
		configuredMode.ID,
		configuredPinned.ID,
		configuredTitle.ID,
		child.ID,
		autonomous.ID,
		inbox.ID,
	} {
		if _, err := svc.GetSession(ctx, id); err != nil {
			t.Fatalf("expected session %q to be preserved: %v", id, err)
		}
	}
}
