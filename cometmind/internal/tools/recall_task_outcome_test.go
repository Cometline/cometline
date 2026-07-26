package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/memory"
	_ "modernc.org/sqlite"
)

func testMemoryService(t *testing.T) (context.Context, *sql.DB, *memory.Service) {
	t.Helper()
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureSchema(ctx, conn); err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	settings := memory.DefaultSettings()
	svc, err := memory.NewService(conn, settings, nil, nil)
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	return ctx, conn, svc
}

func insertMemoryForToolTest(t *testing.T, ctx context.Context, conn *sql.DB, id, kind, content string) {
	t.Helper()
	now := time.Now().UnixMilli()
	if err := db.New(conn).InsertMemory(ctx, db.InsertMemoryParams{
		ID:                id,
		Scope:             "global",
		Kind:              kind,
		Content:           content,
		Source:            "test",
		BaseWeight:        1,
		AccessCount:       0,
		ApplicationPolicy: memory.ApplicationRelevant,
		RetentionPolicy:   memory.RetentionDecaying,
		SummaryJson:       "{}",
		Archived:          0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
}

func TestRecallTaskOutcomeRecent(t *testing.T) {
	ctx, conn, svc := testMemoryService(t)
	defer conn.Close()

	insertMemoryForToolTest(t, ctx, conn, "outcome", "task_outcome", "Completed retry policy")
	insertMemoryForToolTest(t, ctx, conn, "fact", "fact", "Unrelated fact")

	tool := RecallTaskOutcome{Memory: svc}
	res, err := tool.Execute(ctx, json.RawMessage(`{"limit":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	if !strings.Contains(res.Output, "Completed retry policy") || strings.Contains(res.Output, "Unrelated fact") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
}

func TestRegistryIncludesRecallTaskOutcomeWhenMemoryConfigured(t *testing.T) {
	_, conn, svc := testMemoryService(t)
	defer conn.Close()

	r := NewRegistry(t.TempDir(), RegistryOptions{Memory: svc})
	if !r.Has("recall_task_outcome") {
		t.Fatal("registry missing recall_task_outcome")
	}
}
