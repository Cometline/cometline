package memory

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func newPreferenceTestService(t *testing.T) (context.Context, *sql.DB, *Service) {
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
	st := newStore(conn)
	settings := DefaultSettings()
	svc := &Service{
		settings: settings,
		store:    st,
		retriever: &retriever{
			store:    st,
			embedder: stubEmbedder{vectors: map[string][]float32{"x": {1, 0}}},
			settings: settings,
		},
	}
	return ctx, conn, svc
}

func insertPreferenceTestRecord(t *testing.T, ctx context.Context, st *store, id, kind, category, content string, pinned bool) {
	t.Helper()
	now := time.Now()
	if err := st.insert(ctx, Record{
		ID:                 id,
		Scope:              "global",
		Kind:               kind,
		PreferenceCategory: category,
		Content:            content,
		Embedding:          []float32{1, 0},
		Source:             "manual",
		BaseWeight:         1,
		ApplicationPolicy:  map[bool]string{true: ApplicationAlways, false: ApplicationRelevant}[pinned],
		RetentionPolicy:    map[bool]string{true: RetentionProtected, false: RetentionDecaying}[pinned],
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
}

func TestBaselinePreferencesReturnsOnlyPreferencesAndTouchesAccess(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()

	insertPreferenceTestRecord(t, ctx, svc.store, "fact", "fact", "", "A fact", false)
	insertPreferenceTestRecord(t, ctx, svc.store, "pref", "preference", "language", "User prefers Traditional Chinese replies.", true)

	got, err := svc.BaselinePreferences(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "pref" {
		t.Fatalf("got %+v, want only pref", got)
	}
	rec, err := svc.store.get(ctx, "pref")
	if err != nil {
		t.Fatal(err)
	}
	if rec.AccessCount != 1 {
		t.Fatalf("AccessCount = %d, want 1", rec.AccessCount)
	}
}

func TestAlwaysPreferenceIsProtectedByDomainInvariant(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()

	now := time.Now()
	rec := Record{
		ID: "always", Scope: "global", Kind: "preference", Content: "Always answer concisely.",
		Embedding: []float32{1, 0}, Source: "manual", BaseWeight: 1,
		ApplicationPolicy: ApplicationAlways, RetentionPolicy: RetentionDecaying,
		CreatedAt: now, UpdatedAt: now,
	}
	applyPolicyInvariants(&rec)
	if rec.RetentionPolicy != RetentionProtected {
		t.Fatalf("retention policy = %q, want protected", rec.RetentionPolicy)
	}
	if err := svc.store.insert(ctx, rec); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.store.get(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.RetentionPolicy != RetentionProtected {
		t.Fatalf("stored retention policy = %q, want protected", stored.RetentionPolicy)
	}
}

func TestRecentTaskOutcomesReturnsRecentTaskOutcomesAndTouchesAccess(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()

	insertPreferenceTestRecord(t, ctx, svc.store, "old", "task_outcome", "", "Old outcome", false)
	insertPreferenceTestRecord(t, ctx, svc.store, "fact", "fact", "", "A fact", false)
	insertPreferenceTestRecord(t, ctx, svc.store, "new", "task_outcome", "", "New outcome", false)

	got, err := svc.RecentTaskOutcomes(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "new" || got[1].ID != "old" {
		t.Fatalf("got %+v, want newest task outcomes only", got)
	}
	rec, err := svc.store.get(ctx, "new")
	if err != nil {
		t.Fatal(err)
	}
	if rec.AccessCount != 1 {
		t.Fatalf("AccessCount = %d, want 1", rec.AccessCount)
	}
}

func TestCompactPreferenceCategoryArchivesOldDecayingBeyondCap(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()

	insertPreferenceTestRecord(t, ctx, svc.store, "old", "preference", "language", "User prefers English replies.", false)
	insertPreferenceTestRecord(t, ctx, svc.store, "new", "preference", "language", "User prefers Traditional Chinese replies.", false)

	if err := svc.CompactPreferenceCategory(ctx, "language"); err != nil {
		t.Fatal(err)
	}
	oldRec, err := svc.store.get(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	newRec, err := svc.store.get(ctx, "new")
	if err != nil {
		t.Fatal(err)
	}
	if !oldRec.Archived || oldRec.ArchivedReason != "preference_category_cap" {
		t.Fatalf("old archived=%v reason=%q, want archived preference_category_cap", oldRec.Archived, oldRec.ArchivedReason)
	}
	if newRec.Archived {
		t.Fatal("new preference should remain active")
	}
}

func TestCompactPreferenceCategoryBackfillsLegacyEmptyCategory(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()

	insertPreferenceTestRecord(t, ctx, svc.store, "old", "preference", "", "User prefers English replies.", false)
	insertPreferenceTestRecord(t, ctx, svc.store, "new", "preference", "language", "User prefers Traditional Chinese replies.", false)

	if err := svc.CompactPreferenceCategory(ctx, "language"); err != nil {
		t.Fatal(err)
	}
	oldRec, err := svc.store.get(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	if oldRec.PreferenceCategory != "language" {
		t.Fatalf("old category = %q, want language", oldRec.PreferenceCategory)
	}
	if !oldRec.Archived {
		t.Fatal("legacy old preference should be archived by category cap")
	}
}

func TestCompactPreferenceCategoryDoesNotArchiveProtectedPreferences(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()

	insertPreferenceTestRecord(t, ctx, svc.store, "pinned", "preference", "language", "User prefers English replies.", true)
	insertPreferenceTestRecord(t, ctx, svc.store, "new", "preference", "language", "User prefers Traditional Chinese replies.", false)

	if err := svc.CompactPreferenceCategory(ctx, "language"); err != nil {
		t.Fatal(err)
	}
	pinned, err := svc.store.get(ctx, "pinned")
	if err != nil {
		t.Fatal(err)
	}
	if pinned.Archived {
		t.Fatal("pinned preference should not be auto-archived")
	}
}

func TestFormatPromptMemoriesSeparatesPreferencesAndDedupesRelevant(t *testing.T) {
	pref := ScoredMemory{Record: Record{ID: "p", Kind: "preference", Content: "User prefers Traditional Chinese replies."}}
	outcome := ScoredMemory{Record: Record{ID: "o", Kind: "task_outcome", Content: "Completed the retry policy."}}
	relevant := ScoredMemory{Record: Record{ID: "r", Kind: "project", Content: "CometMind uses SQLite."}}
	out := FormatPromptMemories(PromptMemories{
		Records: NewPromptMemories([]ScoredMemory{pref}, []ScoredMemory{outcome}, []ScoredMemory{pref, relevant}).Records,
	})
	if !strings.Contains(out, "## User preferences") || !strings.Contains(out, "## Relevant task outcomes") || !strings.Contains(out, "## Semantic memories") {
		t.Fatalf("missing sections: %q", out)
	}
	if strings.Count(out, "User prefers Traditional Chinese replies.") != 1 {
		t.Fatalf("preference should appear once: %q", out)
	}
	if !strings.Contains(out, "[project] CometMind uses SQLite.") {
		t.Fatalf("missing relevant memory: %q", out)
	}
	if !strings.Contains(out, "Completed the retry policy.") {
		t.Fatalf("missing task outcome: %q", out)
	}
}

func TestNormalizePreferenceCategoryInfersLanguage(t *testing.T) {
	got := normalizePreferenceCategory("preference", "我偏好中文", "")
	if got != "language" {
		t.Fatalf("category = %q, want language", got)
	}
}
