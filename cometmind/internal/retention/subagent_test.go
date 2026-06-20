package retention

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/session"
	_ "modernc.org/sqlite"
)

func TestRunner_PurgesStaleSubagentRows(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	svc := session.New(conn)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := svc.NewSession(ctx, ws.ID, "m", "p")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.NewChildSession(ctx, parent, "done task", "general")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateDelegationState(ctx, child.ID, "completed", "summary", ""); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-10 * 24 * time.Hour).UnixMilli()
	if _, err := conn.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, old, child.ID); err != nil {
		t.Fatal(err)
	}

	rr := &Runner{
		DB:       conn,
		Sessions: svc,
		Config: config.StorageConfig{
			SubagentRetentionDays: 7,
			VacuumAfterPurge:      false,
		},
	}
	got, err := rr.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubagentsDeleted != 1 {
		t.Fatalf("subagents_deleted=%d want 1", got.SubagentsDeleted)
	}
	if _, err := svc.GetSession(ctx, child.ID); err == nil {
		t.Fatal("expected child purged")
	}
	if _, err := svc.GetSession(ctx, parent.ID); err != nil {
		t.Fatal("expected parent to remain")
	}
}
