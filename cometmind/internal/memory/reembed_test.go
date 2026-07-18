package memory

import (
	"context"
	"database/sql"
	"testing"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestPreviewAndStartReembedWithNoMemoriesAppliesImmediately(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}

	svc, err := NewService(conn, Settings{
		Enabled: true,
		Embedding: EmbeddingSettings{
			Provider: "ollama",
			Model:    "nomic-embed-text",
			BaseURL:  "http://127.0.0.1:11434",
		},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := svc.PreviewReembed(context.Background(), EmbeddingSettings{
		Provider: "ollama",
		Model:    "qwen3-embedding:0.6b",
		BaseURL:  "http://127.0.0.1:11434",
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.MigrationNeeded {
		t.Fatalf("expected no migration with empty memory store, got %+v", preview)
	}

	job, err := svc.StartReembed(context.Background(), EmbeddingSettings{
		Provider: "ollama",
		Model:    "qwen3-embedding:0.6b",
		BaseURL:  "http://127.0.0.1:11434",
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != ReembedCompleted {
		t.Fatalf("status = %s, want completed", job.Status)
	}
	if svc.Settings().Embedding.Model != "qwen3-embedding:0.6b" {
		t.Fatalf("settings model = %q", svc.Settings().Embedding.Model)
	}
}
