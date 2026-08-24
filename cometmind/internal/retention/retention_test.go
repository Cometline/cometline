package retention

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/usage"
	_ "modernc.org/sqlite"
)

func TestRunner_PurgesDeletedJobs(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	jobSvc := jobs.NewService(conn, nil, nil)
	job, err := jobSvc.Create(ctx, jobs.CreateInput{Description: "purge me"})
	if err != nil {
		t.Fatal(err)
	}
	if err := jobSvc.SoftDelete(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).UnixMilli()
	if _, err := conn.ExecContext(ctx, `UPDATE jobs SET deleted_at = ? WHERE id = ?`, old, job.ID); err != nil {
		t.Fatal(err)
	}

	got, err := (&Runner{
		DB:           conn,
		Sessions:     session.New(conn),
		Jobs:         jobSvc,
		JobPurgeDays: 1,
	}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.JobsPurged != 1 {
		t.Fatalf("jobs_purged = %d, want 1", got.JobsPurged)
	}
	if _, err := jobSvc.Get(ctx, job.ID); err != jobs.ErrNotFound {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

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
		ID: "sess-old", WorkspaceID: ws.ID, Title: "old", ModelID: "m", ProviderID: "p", Status: "active", Origin: "user", AgentMode: "auto",
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
			id, scope, kind, content, embedding, source, base_weight, access_count,
			archived, archived_reason, created_at, updated_at
		) VALUES (?, 'global', 'fact', 'old archived fact', X'', 'manual', 1, 0, 1, 'decayed', ?, ?)`,
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

func TestRunner_PurgesExpiredDetachedMedia(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	sessions := session.New(conn)
	ws, err := sessions.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.NewSession(ctx, ws.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := media.RegisterBytes(sess.ID, "image/png", "expired", []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.AppendAssistantMedia(ctx, sess.ID, []session.ContentBlock{{
		Type: "image", ID: ref.ID, MediaType: ref.MediaType, Alt: ref.Alt,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := sessions.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-31 * 24 * time.Hour).UnixMilli()
	if err := db.New(conn).InitializeDetachedSessionMedia(ctx, old); err != nil {
		t.Fatal(err)
	}

	result, err := (&Runner{
		DB:       conn,
		Sessions: sessions,
		Config: config.StorageConfig{
			DetachedMediaRetentionDays: 30,
		},
	}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.MediaDeleted != 1 {
		t.Fatalf("media_deleted=%d want 1", result.MediaDeleted)
	}
	item, err := sessions.GetMedia(ctx, ref.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != "deleted" {
		t.Fatalf("status=%q want deleted", item.Status)
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
			ID: id, WorkspaceID: ws.ID, Title: id, ModelID: "m", ProviderID: "p", Status: "active", Origin: "user", AgentMode: "auto",
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

func TestRunner_PurgesUsageOlderThanOneYear(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	old := time.Now().AddDate(-2, 0, 0).UnixMilli()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO usage_events (id, created_at, provider_id, model_id, call_kind, input_tokens, output_tokens, cache_read, cache_write)
		VALUES ('old', ?, 'openai', 'gpt-4o', 'agent_step', 10, 0, 0, 0)`, old); err != nil {
		t.Fatal(err)
	}
	got, err := (&Runner{
		DB:       conn,
		Sessions: session.New(conn),
		Usage:    usage.NewService(conn),
		Config:   config.StorageConfig{VacuumAfterPurge: false},
	}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.UsageEventsPurged != 1 {
		t.Fatalf("usage_events_purged=%d want 1", got.UsageEventsPurged)
	}
}
