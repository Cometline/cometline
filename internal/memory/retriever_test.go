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
			"query":  {1, 0},
			"alpha":  {1, 0},
			"beta":   {0.9, 0.1},
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

type stubEmbedder struct {
	vectors map[string][]float32
}

func (s stubEmbedder) Model() string { return "stub" }

func (s stubEmbedder) Embed(_ context.Context, texts ...string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = s.vectors[text]
	}
	return out, nil
}
