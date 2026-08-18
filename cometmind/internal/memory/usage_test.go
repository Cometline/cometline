package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/usage"
	_ "modernc.org/sqlite"
)

type usageStubEmbedder struct{}

func (usageStubEmbedder) Model() string { return "stub" }

func (usageStubEmbedder) Embed(_ context.Context, texts ...string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1}
	}
	return out, nil
}

type captureRecorder struct {
	events []usage.Event
}

func (c *captureRecorder) Record(_ context.Context, ev usage.Event) error {
	c.events = append(c.events, ev)
	return nil
}

func TestRecordingEmbedderUsesUsageScope(t *testing.T) {
	rec := &captureRecorder{}
	embedder := wrapEmbedder(usageStubEmbedder{}, rec, EmbeddingSettings{Provider: "ollama", Model: "nomic-embed-text"})
	ctx := usage.WithScope(context.Background(), "ws-1", "sess-1")
	if _, err := embedder.Embed(ctx, "hello world"); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("events=%d", len(rec.events))
	}
	got := rec.events[0]
	if got.CallKind != usage.KindEmbedding || got.WorkspaceID != "ws-1" || got.SessionID != "sess-1" {
		t.Fatalf("event=%+v", got)
	}
	if got.Usage.InputTokens <= 0 {
		t.Fatalf("tokens=%d", got.Usage.InputTokens)
	}
}

func TestRecordUsageWritesSessionAndWorkspace(t *testing.T) {
	rec := &captureRecorder{}
	ctx := usage.WithScope(context.Background(), "ws-2", "sess-2")
	recordUsage(ctx, rec, nil, "claude-sonnet-4-5", usage.KindMemoryExtract, "sess-2", cometsdk.TokenUsage{InputTokens: 8})
	if len(rec.events) != 1 {
		t.Fatalf("events=%d", len(rec.events))
	}
	if rec.events[0].WorkspaceID != "ws-2" || rec.events[0].SessionID != "sess-2" {
		t.Fatalf("event=%+v", rec.events[0])
	}
}

type jsonUsageProvider struct {
	text  string
	usage cometsdk.TokenUsage
}

func (p jsonUsageProvider) ID() string { return "anthropic" }

func (p jsonUsageProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, 3)
	ch <- cometsdk.TextDeltaEvent{Text: p.text}
	ch <- cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: p.usage}
	ch <- cometsdk.DoneEvent{}
	close(ch)
	return ch, nil
}

func TestExtractAfterTurnFollowOnUsesWorkspaceScope(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	sessions := session.New(conn)
	ws, err := sessions.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.NewSession(ctx, ws.ID, "claude-sonnet-4-5", "anthropic")
	if err != nil {
		t.Fatal(err)
	}

	settings := DefaultSettings()
	settings.ExtractionModel = "claude-sonnet-4-5"
	settings.DefaultModel = "claude-sonnet-4-5"
	settings.Lifecycle.MaxMemories = 1
	settings.Lifecycle.CompactionOnExtract = true
	settings.Lifecycle.CompactionTargetRatio = 0.5
	settings.Embedding = EmbeddingSettings{Provider: "ollama", Model: "nomic-embed-text", BaseURL: "http://127.0.0.1:1"}

	provider := jsonUsageProvider{
		text:  `{"content":"merged memory"}`,
		usage: cometsdk.TokenUsage{InputTokens: 40, OutputTokens: 8},
	}
	svc, err := NewService(conn, settings, provider, sessions)
	if err != nil {
		t.Fatal(err)
	}
	rec := &captureRecorder{}
	svc.SetUsageRecorder(rec)
	svc.applyEmbedder(constantEmbedder{vector: []float32{1, 0}})

	now := time.Now()
	for _, id := range []string{"mem-a", "mem-b"} {
		if err := svc.store.insert(ctx, Record{
			ID:                id,
			Scope:             "global",
			Kind:              "fact",
			Content:           "related fact " + id,
			Embedding:         []float32{1, 0},
			Source:            "manual",
			BaseWeight:        1,
			ApplicationPolicy: ApplicationRelevant,
			RetentionPolicy:   RetentionDecaying,
			CreatedAt:         now,
			UpdatedAt:         now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := svc.ExtractAfterTurn(ctx, sess.ID, "claude-sonnet-4-5", nil); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) == 0 {
		t.Fatal("expected compaction or embedding usage from extract follow-on")
	}
	for _, ev := range rec.events {
		if ev.WorkspaceID != ws.ID {
			t.Fatalf("event workspace=%q want %q (%+v)", ev.WorkspaceID, ws.ID, ev)
		}
		if ev.SessionID != sess.ID {
			t.Fatalf("event session=%q want %q (%+v)", ev.SessionID, sess.ID, ev)
		}
	}
}

func TestRecordTaskOutcomeUsesUsageScope(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	t.Cleanup(func() { _ = conn.Close() })
	rec := &captureRecorder{}
	svc.retriever.embedder = wrapEmbedder(constantEmbedder{vector: []float32{1, 0}}, rec, EmbeddingSettings{
		Provider: "ollama", Model: "nomic-embed-text",
	})
	scoped := usage.WithScope(ctx, "ws-outcome", "sess-outcome")
	if _, err := svc.RecordTaskOutcome(scoped, TaskOutcomeInput{
		OriginType: "job", OriginID: "job-scope", Status: "done", Description: "scoped outcome",
	}); err != nil {
		t.Fatal(err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("events=%d", len(rec.events))
	}
	if rec.events[0].WorkspaceID != "ws-outcome" || rec.events[0].SessionID != "sess-outcome" {
		t.Fatalf("event=%+v", rec.events[0])
	}
}
