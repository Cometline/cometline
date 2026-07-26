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

func TestRunLifecycleRecountsAfterForgettingBeforeCompaction(t *testing.T) {
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
	old := time.Now().Add(-180 * 24 * time.Hour)
	if err := st.insert(ctx, Record{ID: "decayed", Scope: "global", Kind: "fact", Content: "old", Source: "test", BaseWeight: 0.1, RetentionPolicy: RetentionDecaying, LastAccessedAt: &old}); err != nil {
		t.Fatal(err)
	}
	settings := DefaultSettings()
	settings.Lifecycle.MaxMemories = 1
	settings.Lifecycle.ForgetThreshold = 0.5
	svc := &Service{settings: settings, store: st, compactor: &compactor{store: st, settings: settings}}
	notified := false
	svc.SetCompactionCompletedNotifier(func(CompactionResult) { notified = true })
	if err := svc.RunLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	if notified {
		t.Fatal("full compaction ran after decay reduced active count below threshold")
	}
	if count, err := st.countActive(ctx); err != nil || count != 0 {
		t.Fatalf("active count = %d, err = %v", count, err)
	}
}

func TestCompactorOnlyClustersCompatiblePools(t *testing.T) {
	fact := Record{Kind: "fact", Scope: "global", RetentionPolicy: RetentionDecaying}
	if !compatibleForMerge(fact, fact) {
		t.Fatal("same kind/scope facts should merge")
	}
	if compatibleForMerge(fact, Record{Kind: "project", Scope: "global", RetentionPolicy: RetentionDecaying}) {
		t.Fatal("mixed kinds must not merge")
	}
	withOrigin := fact
	withOrigin.OriginType, withOrigin.OriginID = "source", "one"
	otherOrigin := withOrigin
	otherOrigin.OriginID = "two"
	if compatibleForMerge(withOrigin, otherOrigin) {
		t.Fatal("different lineages must not merge")
	}
	if mergeable(Record{Kind: "task_outcome", Scope: "global", RetentionPolicy: RetentionDecaying}) {
		t.Fatal("task outcomes must not generic-merge")
	}
	if compactable(Record{Kind: "preference", ApplicationPolicy: ApplicationAlways, RetentionPolicy: RetentionProtected}) {
		t.Fatal("always/protected preference must not compact")
	}
}

func TestReplaceWithMergedRollsBackWhenFTSWriteFails(t *testing.T) {
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
	now := time.Now()
	sources := []Record{
		{ID: "source-a", Scope: "global", Kind: "fact", Content: "a", Source: "test", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying, CreatedAt: now, UpdatedAt: now},
		{ID: "source-b", Scope: "global", Kind: "fact", Content: "b", Source: "test", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying, CreatedAt: now, UpdatedAt: now},
	}
	for _, source := range sources {
		if err := st.insert(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.ExecContext(ctx, `DROP TABLE memories_fts`); err != nil {
		t.Fatal(err)
	}
	merged := Record{ID: "merged", Scope: "global", Kind: "fact", Content: "merged", Source: "compacted", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying}
	if err := st.replaceWithMerged(ctx, merged, sources, `{}`); err == nil {
		t.Fatal("replaceWithMerged should fail without FTS table")
	}
	var mergedCount, activeSources int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE id = 'merged'`).Scan(&mergedCount); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE id IN ('source-a', 'source-b') AND archived = 0`).Scan(&activeSources); err != nil {
		t.Fatal(err)
	}
	if mergedCount != 0 || activeSources != 2 {
		t.Fatalf("merged=%d active_sources=%d, transaction was not atomic", mergedCount, activeSources)
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
