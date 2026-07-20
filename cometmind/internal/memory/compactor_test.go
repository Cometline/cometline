package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestCompactorForgetsDecayed(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	st := newStore(conn)
	old := time.Now().Add(-120 * 24 * time.Hour)
	rec := Record{
		ID:             "old",
		Scope:          "global",
		Kind:           "fact",
		Content:        "stale",
		Embedding:      []float32{1, 0},
		Source:         "manual",
		BaseWeight:     0.2,
		LastAccessedAt: &old,
		CreatedAt:      old,
		UpdatedAt:      old,
	}
	if err := st.insert(ctx, rec); err != nil {
		t.Fatal(err)
	}

	settings := DefaultSettings()
	settings.Lifecycle.ForgetThreshold = 0.5
	c := &compactor{store: st, embedder: stubEmbedder{}, provider: nil, settings: settings}
	if err := c.forgetDecayed(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := st.get(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Archived {
		t.Fatal("expected archived memory")
	}
}

func TestRunLifecycleNotifiesAutomaticCompaction(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	st := newStore(conn)
	settings := DefaultSettings()
	settings.Lifecycle.MaxMemories = 0
	svc := &Service{
		settings:  settings,
		store:     st,
		compactor: &compactor{store: st, embedder: stubEmbedder{}, settings: settings},
	}
	var notified CompactionResult
	svc.SetCompactionCompletedNotifier(func(result CompactionResult) {
		notified = result
	})

	if err := svc.RunLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	if notified.Trigger != "automatic" || notified.Before != 0 || notified.After != 0 {
		t.Fatalf("unexpected notification: %+v", notified)
	}
}

func TestExtractionModelPrefersPinnedThenFallback(t *testing.T) {
	if got := extractionModel(Settings{ExtractionModel: "qwen3.7-plus", DefaultModel: "gpt-5.4"}); got != "qwen3.7-plus" {
		t.Fatalf("got %q, want pinned extraction model", got)
	}
	if got := extractionModel(Settings{DefaultModel: "gpt-5.4"}); got != "gpt-5.4" {
		t.Fatalf("got %q, want default chat model", got)
	}
	if got := extractionModel(Settings{}); got != "" {
		t.Fatalf("got %q, want empty when no model is configured", got)
	}
}
