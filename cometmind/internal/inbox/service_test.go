package inbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/store"
)

func TestInboxCreateReplyDismissAndPurge(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	svc := inbox.NewService(sqlDB)
	msg, err := svc.Create(ctx, inbox.CreateInput{
		Title:       "Smoke failed",
		Body:        "package foo broke",
		WorkspaceID: "ws-1",
		JobID:       "job-1",
		SessionID:   "sess-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if msg.Status != inbox.StatusOpen {
		t.Fatalf("status=%q", msg.Status)
	}

	open, err := svc.CountOpen(ctx)
	if err != nil || open != 1 {
		t.Fatalf("CountOpen=%d err=%v", open, err)
	}

	listed, err := svc.List(ctx, inbox.ListFilter{Status: inbox.StatusOpen})
	if err != nil || len(listed) != 1 {
		t.Fatalf("List open: len=%d err=%v", len(listed), err)
	}

	replied, err := svc.Reply(ctx, msg.ID, "please include package name next time")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if replied.Status != inbox.StatusArchived || replied.ArchiveReason != inbox.ArchiveReasonReplied {
		t.Fatalf("reply archive: %+v", replied)
	}
	if open, _ = svc.CountOpen(ctx); open != 0 {
		t.Fatalf("open after reply=%d", open)
	}

	pending, err := svc.ListPendingProcess(ctx, 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%d err=%v", len(pending), err)
	}

	claimed, err := svc.ClaimForProcess(ctx, replied.ID)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if claimed.ProcessAttempts != 1 {
		t.Fatalf("attempts=%d", claimed.ProcessAttempts)
	}
	if _, err := svc.MarkProcessed(ctx, claimed.ID, ""); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	pending, err = svc.ListPendingProcess(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after process=%d err=%v", len(pending), err)
	}

	msg2, err := svc.Create(ctx, inbox.CreateInput{Title: "Backup ok", Body: "no issues"})
	if err != nil {
		t.Fatal(err)
	}
	dismissed, err := svc.Dismiss(ctx, msg2.ID)
	if err != nil {
		t.Fatalf("Dismiss: %v", err)
	}
	if dismissed.ArchiveReason != inbox.ArchiveReasonDismissed {
		t.Fatalf("dismiss reason=%q", dismissed.ArchiveReason)
	}
	pending, err = svc.ListPendingProcess(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("dismissed must not be pending: %d", len(pending))
	}

	// Force archived_at into the past and purge.
	if _, err := sqlDB.ExecContext(ctx, `UPDATE inbox_messages SET archived_at = ?`, time.Now().Add(-48*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	n, err := svc.PurgeExpired(ctx, 24)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n < 2 {
		t.Fatalf("purged=%d want >=2", n)
	}
}

func TestInboxReplyRequiresContent(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	svc := inbox.NewService(sqlDB)
	msg, err := svc.Create(ctx, inbox.CreateInput{Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reply(ctx, msg.ID, "  "); err == nil {
		t.Fatal("expected empty reply error")
	}
}
