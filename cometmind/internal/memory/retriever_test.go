package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestRetrieverRanksByRetrievalScore(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}

	embedder := stubEmbedder{
		vectors: map[string][]float32{
			"query": {1, 0},
			"alpha": {1, 0},
			"beta":  {0.9, 0.1},
		},
	}
	st := newStore(conn)
	now := time.Now()
	for _, m := range []Record{
		{ID: "a", Scope: "global", Kind: "fact", Content: "alpha", Embedding: []float32{1, 0}, Source: "manual", BaseWeight: 0.5, LastAccessedAt: &now, CreatedAt: now, UpdatedAt: now},
		{ID: "b", Scope: "global", Kind: "fact", Content: "beta", Embedding: []float32{0.9, 0.1}, Source: "manual", BaseWeight: 1.0, LastAccessedAt: &now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := st.insert(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	r := &retriever{
		store:    st,
		embedder: embedder,
		settings: DefaultSettings(),
	}
	got, err := r.retrieve(ctx, "query", 2, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].ID != "b" {
		t.Fatalf("expected higher retrieval score for b, got %s first", got[0].ID)
	}
}

func TestRetrieverBuildsExclusiveTaskAndSemanticPools(t *testing.T) {
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
	for _, m := range []Record{
		{ID: "task", Scope: "global", Kind: "task_outcome", Content: "matching outcome", Embedding: []float32{1, 0}, Source: "job", BaseWeight: 2, LastAccessedAt: &now, CreatedAt: now, UpdatedAt: now},
		{ID: "fact", Scope: "global", Kind: "fact", Content: "matching fact", Embedding: []float32{1, 0}, Source: "manual", BaseWeight: 1, LastAccessedAt: &now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := st.insert(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	r := &retriever{
		store: st,
		embedder: stubEmbedder{vectors: map[string][]float32{
			"query": {1, 0},
		}},
		settings: DefaultSettings(),
	}
	got, err := r.retrievePools(ctx, "query", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 2 || got.Records[0].Bucket != BucketTaskOutcome || got.Records[1].Bucket != BucketSemantic {
		t.Fatalf("got %#v, want exclusive task and semantic records", got)
	}
}

func TestRetrievePoolsAlwaysPreferencesFirstThenCategoryDiverseRelevant(t *testing.T) {
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
	for _, rec := range []Record{
		{ID: "always", Scope: "global", Kind: "preference", PreferenceCategory: "tone", Content: "matching tone", Embedding: []float32{1, 0}, Source: "test", BaseWeight: 1, ApplicationPolicy: ApplicationAlways, RetentionPolicy: RetentionProtected, CreatedAt: now, UpdatedAt: now},
		{ID: "language", Scope: "global", Kind: "preference", PreferenceCategory: "language", Content: "matching language", Embedding: []float32{1, 0}, Source: "test", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying, CreatedAt: now, UpdatedAt: now},
		{ID: "workflow", Scope: "global", Kind: "preference", PreferenceCategory: "workflow", Content: "matching workflow", Embedding: []float32{0.99, 0.01}, Source: "test", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying, CreatedAt: now, UpdatedAt: now},
		{ID: "language-2", Scope: "global", Kind: "preference", PreferenceCategory: "language", Content: "matching second language", Embedding: []float32{1, 0}, Source: "test", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying, CreatedAt: now, UpdatedAt: now},
	} {
		if err := st.insert(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	r := &retriever{store: st, settings: DefaultSettings(), embedder: countingEmbedder{calls: &calls, vector: []float32{1, 0}}}
	got, err := r.retrievePools(ctx, "matching", 100)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("embedding calls = %d, want one", calls)
	}
	if got.Count(BucketPreference) != 3 || got.Records[0].ID != "always" {
		t.Fatalf("preferences = %#v", got.Records)
	}
	categories := map[string]bool{}
	for _, item := range got.Records[1:] {
		categories[item.PreferenceCategory] = true
	}
	if !categories["language"] || !categories["workflow"] {
		t.Fatalf("relevant categories = %#v, want diversity", categories)
	}
}

type stubEmbedder struct {
	vectors map[string][]float32
}

type countingEmbedder struct {
	calls  *int
	vector []float32
}

func (e countingEmbedder) Model() string { return "counting" }
func (e countingEmbedder) Embed(_ context.Context, texts ...string) ([][]float32, error) {
	*e.calls++
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = e.vector
	}
	return out, nil
}

func (s stubEmbedder) Model() string { return "stub" }

func (s stubEmbedder) Embed(_ context.Context, texts ...string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = s.vectors[text]
	}
	return out, nil
}
