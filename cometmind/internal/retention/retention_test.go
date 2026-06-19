package retention

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	_ "modernc.org/sqlite"
)

func TestRunner_DeletesStaleSessionAndGatewayMapping(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	q := db.New(conn)
	ws, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{ID: "ws1", Name: "demo", Path: "/tmp/demo"})
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-100 * 24 * time.Hour).UnixMilli()
	sess, err := q.CreateSession(ctx, db.CreateSessionParams{
		ID: "sess-old", WorkspaceID: ws.ID, Title: "old", ModelID: "m", ProviderID: "p", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, old, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertGatewaySession(ctx, db.UpsertGatewaySessionParams{
		ID: "gw1", Platform: "discord", PlatformUserID: "u1", PlatformChannelID: "ch1",
		ThreadID: "", CometmindSessionID: sess.ID, WorkspaceID: ws.ID,
	}); err != nil {
		t.Fatal(err)
	}

	sessions := session.New(conn)
	rr := &Runner{
		DB:       conn,
		Sessions: sessions,
		Config: config.StorageConfig{
			RetentionDays:           90,
			MaxSessionsPerWorkspace: 0,
			ArchivedMemoryPurgeDays: 0,
			VacuumAfterPurge:        false,
		},
	}
	got, err := rr.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionsDeleted != 1 {
		t.Fatalf("sessions_deleted=%d want 1", got.SessionsDeleted)
	}
	if _, err := q.GetSession(ctx, sess.ID); err == nil {
		t.Fatal("expected session deleted")
	}
	if _, err := q.GetGatewaySession(ctx, db.GetGatewaySessionParams{
		Platform: "discord", PlatformUserID: "u1", PlatformChannelID: "ch1", ThreadID: "",
	}); err == nil {
		t.Fatal("expected gateway mapping cascade deleted")
	}
}

func TestRunner_PurgesArchivedMemories(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	oldMS := time.Now().Add(-100 * 24 * time.Hour).UnixMilli()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO memories (
			id, scope, kind, content, embedding, source, base_weight, access_count, pinned,
			archived, archived_reason, created_at, updated_at
		) VALUES (?, 'global', 'fact', 'old archived fact', X'', 'manual', 1, 0, 0, 1, 'decayed', ?, ?)`,
		"mem1", oldMS, oldMS,
	); err != nil {
		t.Fatal(err)
	}

	mem, err := memory.NewService(conn, memory.DefaultSettings(), nil, session.New(conn))
	if err != nil {
		t.Fatal(err)
	}

	rr := &Runner{
		DB:       conn,
		Sessions: session.New(conn),
		Memory:   mem,
		Config: config.StorageConfig{
			RetentionDays:           0,
			ArchivedMemoryPurgeDays: 90,
			VacuumAfterPurge:        false,
		},
	}
	got, err := rr.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.MemoriesPurged != 1 {
		t.Fatalf("memories_purged=%d want 1", got.MemoriesPurged)
	}
}

func TestRunner_MaxSessionsPerWorkspace(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	q := db.New(conn)
	ws, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{ID: "ws1", Name: "demo", Path: "/tmp/demo"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	for i, id := range []string{"s1", "s2", "s3"} {
		if _, err := q.CreateSession(ctx, db.CreateSessionParams{
			ID: id, WorkspaceID: ws.ID, Title: id, ModelID: "m", ProviderID: "p", Status: "active",
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, now+int64(i), id); err != nil {
			t.Fatal(err)
		}
	}

	rr := &Runner{
		DB:       conn,
		Sessions: session.New(conn),
		Config: config.StorageConfig{
			MaxSessionsPerWorkspace: 2,
			VacuumAfterPurge:        false,
		},
	}
	got, err := rr.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionsDeleted != 1 {
		t.Fatalf("sessions_deleted=%d want 1", got.SessionsDeleted)
	}
	if _, err := q.GetSession(ctx, "s1"); err == nil {
		t.Fatal("expected oldest session deleted")
	}
}
