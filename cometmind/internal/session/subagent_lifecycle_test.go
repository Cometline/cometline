package session

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestCompactChildSessionWipesMessages(t *testing.T) {
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
	parent, err := svc.NewSession(ctx, ws.ID, "parent-model", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.NewChildSession(ctx, parent, "research task", "general")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendUserMessage(ctx, child.ID, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompactChildSession(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	msgs, err := svc.BuildSDKMessages(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after compact, got %d", len(msgs))
	}
	got, err := svc.GetSession(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentSessionID != parent.ID {
		t.Fatalf("parent_session_id = %q want %q", got.ParentSessionID, parent.ID)
	}
}

func TestDeleteSessionRemovesChildren(t *testing.T) {
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
	parent, err := svc.NewSession(ctx, ws.ID, "parent-model", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.NewChildSession(ctx, parent, "task", "acp")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSession(ctx, parent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetSession(ctx, child.ID); err == nil {
		t.Fatal("expected child deleted with parent")
	}
}

func TestListAllSessionsExcludesChildRows(t *testing.T) {
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
	parent, err := svc.NewSession(ctx, ws.ID, "parent-model", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.NewChildSession(ctx, parent, "task", "general"); err != nil {
		t.Fatal(err)
	}
	all, err := svc.ListAllSessions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("ListAllSessions() len = %d want 1", len(all))
	}
	if all[0].ID != parent.ID {
		t.Fatalf("ListAllSessions()[0].ID = %q want parent", all[0].ID)
	}
}
