package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/usage"
	_ "modernc.org/sqlite"
)

type failRecorder struct{}

func (failRecorder) Record(context.Context, usage.Event) error {
	return errors.New("ledger down")
}

func TestSaveTokenUsageAccumulatesAndRecords(t *testing.T) {
	t.Parallel()

	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	svc := New(conn)
	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "claude-sonnet-4-5", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	usageSvc := usage.NewService(conn)
	svc.SetUsageRecorder(usageSvc)

	if err := svc.SaveTokenUsage(context.Background(), sess.ID, cometsdk.TokenUsage{InputTokens: 10, OutputTokens: 2}, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveTokenUsage(context.Background(), sess.ID, cometsdk.TokenUsage{InputTokens: 5, OutputTokens: 3}, "", ""); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	var total cometsdk.TokenUsage
	if err := json.Unmarshal([]byte(got.TokenUsage), &total); err != nil {
		t.Fatal(err)
	}
	if total.InputTokens != 15 || total.OutputTokens != 5 {
		t.Fatalf("token usage = %+v, want in=15 out=5", total)
	}
	page, err := usageSvc.List(context.Background(), got.CreatedAt-1, got.UpdatedAt+1, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("ledger rows = %d, want 2", page.Total)
	}
}

func TestSaveTokenUsageLedgerFailureDoesNotError(t *testing.T) {
	t.Parallel()
	svc, _, sess := newUsageTestSession(t)
	svc.SetUsageRecorder(failRecorder{})
	if err := svc.SaveTokenUsage(context.Background(), sess.ID, cometsdk.TokenUsage{InputTokens: 4}, "openai", "gpt-4o"); err != nil {
		t.Fatalf("SaveTokenUsage returned %v, want nil", err)
	}
	got, err := svc.GetSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	var total cometsdk.TokenUsage
	if err := json.Unmarshal([]byte(got.TokenUsage), &total); err != nil {
		t.Fatal(err)
	}
	if total.InputTokens != 4 {
		t.Fatalf("token usage = %+v, want in=4", total)
	}
}

func TestSaveTokenUsageUsesStepModelAndRepairsInvalidJSON(t *testing.T) {
	t.Parallel()
	svc, conn, sess := newUsageTestSession(t)
	usageSvc := usage.NewService(conn)
	svc.SetUsageRecorder(usageSvc)
	if _, err := conn.ExecContext(context.Background(), `UPDATE sessions SET token_usage = 'not-json' WHERE id = ?`, sess.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveTokenUsage(context.Background(), sess.ID, cometsdk.TokenUsage{InputTokens: 3}, "openai", "gpt-4o"); err != nil {
		t.Fatal(err)
	}
	page, err := usageSvc.List(context.Background(), 0, sess.UpdatedAt+60*60*1000, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("ledger rows = %d, want 1", page.Total)
	}
	if page.Items[0].ProviderID != "openai" || page.Items[0].ModelID != "gpt-4o" {
		t.Fatalf("ledger model = %s/%s", page.Items[0].ProviderID, page.Items[0].ModelID)
	}
	got, err := svc.GetSession(context.Background(), sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	var total cometsdk.TokenUsage
	if err := json.Unmarshal([]byte(got.TokenUsage), &total); err != nil {
		t.Fatal(err)
	}
	if total.InputTokens != 3 {
		t.Fatalf("repaired usage = %+v, want in=3", total)
	}
	if err := svc.SaveTokenUsage(context.Background(), "missing-session", cometsdk.TokenUsage{InputTokens: 1}, "openai", "gpt-4o"); err != nil {
		t.Fatalf("missing session returned %v, want nil", err)
	}
}

func newUsageTestSession(t *testing.T) (*Service, *sql.DB, Session) {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	svc := New(conn)
	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "claude-sonnet-4-5", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	return svc, conn, sess
}
