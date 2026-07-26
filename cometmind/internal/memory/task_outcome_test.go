package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func TestTaskOutcomeRollupKeepsLatestFiveAndArchivesOlderToSummary(t *testing.T) {
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.EnsureSchema(ctx, conn); err != nil {
		t.Fatal(err)
	}
	settings := DefaultSettings()
	st := newStore(conn)
	svc := &Service{settings: settings, store: st, retriever: &retriever{
		store: st, settings: settings, embedder: stubEmbedder{vectors: map[string][]float32{}},
	}}
	svc.retriever.embedder = constantEmbedder{vector: []float32{1, 0}}
	old := time.Now().Add(-45 * 24 * time.Hour)
	for i := 0; i < 6; i++ {
		payload, _ := json.Marshal(TaskSummary{Status: "done", LastCompletedAt: old.Add(time.Duration(i) * time.Hour).UnixMilli()})
		rec := Record{ID: NewID(), Scope: "global", Kind: "task_outcome", Content: "old outcome", Embedding: []float32{1, 0}, Source: "job", BaseWeight: 1, ApplicationPolicy: ApplicationRelevant, RetentionPolicy: RetentionDecaying, OriginType: "job", OriginID: "job-1", SummaryJSON: string(payload), CreatedAt: old.Add(time.Duration(i) * time.Hour), UpdatedAt: old}
		if err := st.insert(ctx, rec); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, `UPDATE memories SET created_at = ?, updated_at = ? WHERE id = ?`, rec.CreatedAt.UnixMilli(), rec.UpdatedAt.UnixMilli(), rec.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.rollUpTaskLineage(ctx, "job", "job-1"); err != nil {
		t.Fatal(err)
	}
	var activeOutcomes, activeSummaries, archived int
	if err := conn.QueryRowContext(ctx, `SELECT
		SUM(CASE WHEN archived = 0 AND kind = 'task_outcome' THEN 1 ELSE 0 END),
		SUM(CASE WHEN archived = 0 AND kind = 'task_summary' THEN 1 ELSE 0 END),
		SUM(CASE WHEN archived = 1 AND superseded_by IS NOT NULL THEN 1 ELSE 0 END)
		FROM memories WHERE origin_type = 'job' AND origin_id = 'job-1'`).Scan(&activeOutcomes, &activeSummaries, &archived); err != nil {
		t.Fatal(err)
	}
	if activeOutcomes != 5 || activeSummaries != 1 || archived != 1 {
		t.Fatalf("outcomes=%d summaries=%d archived=%d", activeOutcomes, activeSummaries, archived)
	}
	var content, raw string
	if err := conn.QueryRowContext(ctx, `SELECT content, summary_json FROM memories WHERE archived = 0 AND kind = 'task_summary'`).Scan(&content, &raw); err != nil {
		t.Fatal(err)
	}
	var summary TaskSummary
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Summary != "old outcome" || !strings.Contains(content, "Task history: old outcome") {
		t.Fatalf("summary=%q content=%q", summary.Summary, content)
	}
}

func TestRecordTaskOutcomePersistsScheduledJobOriginAndStructuredJSON(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()
	svc.retriever.embedder = constantEmbedder{vector: []float32{1, 0}}
	rec, err := svc.RecordTaskOutcome(ctx, TaskOutcomeInput{OriginType: "scheduled_job", OriginID: "schedule-1", Status: "done", Description: "refresh index", Artifacts: []string{"index.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if rec.OriginType != "scheduled_job" || rec.OriginID != "schedule-1" {
		t.Fatalf("origin = %s/%s", rec.OriginType, rec.OriginID)
	}
	var summary TaskSummary
	if err := json.Unmarshal([]byte(rec.SummaryJSON), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != "done" || len(summary.Artifacts) != 1 || summary.LastCompletedAt == 0 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Summary != "refresh index" || rec.Content != "Task done: refresh index" {
		t.Fatalf("human summary=%q content=%q", summary.Summary, rec.Content)
	}
}

func TestRecordTaskOutcomeReturnsSuccessWhenRollupMaintenanceFails(t *testing.T) {
	ctx, conn, svc := newPreferenceTestService(t)
	defer conn.Close()
	svc.retriever.embedder = constantEmbedder{vector: []float32{1, 0}}
	svc.rollUpTaskLineageOverride = func(context.Context, string, string) error {
		return fmt.Errorf("maintenance unavailable")
	}
	rec, err := svc.RecordTaskOutcome(ctx, TaskOutcomeInput{OriginType: "job", OriginID: "job-1", Status: "done", Description: "durable result"})
	if err != nil {
		t.Fatalf("durable insert reported false failure: %v", err)
	}
	stored, err := svc.store.get(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Content != "Task done: durable result" {
		t.Fatalf("stored content = %q", stored.Content)
	}
}

func TestMergeTaskSummaryTextIsDeterministicAndBounded(t *testing.T) {
	values := []string{"prior one | prior two"}
	for i := 0; i < 10; i++ {
		values = append(values, fmt.Sprintf("outcome %d %s", i, strings.Repeat("x", 300)))
	}
	got := mergeTaskSummaryText(values...)
	if len([]rune(got)) > taskSummaryMaxRunes {
		t.Fatalf("summary has %d runes", len([]rune(got)))
	}
	fragments := strings.Split(got, " | ")
	if len(fragments) > taskSummaryMaxFragments {
		t.Fatalf("summary has %d fragments", len(fragments))
	}
	if !strings.Contains(got, "outcome 9") || strings.Contains(got, "prior one") {
		t.Fatalf("summary should preserve bounded newest outcomes: %q", got)
	}
	if again := mergeTaskSummaryText(values...); again != got {
		t.Fatalf("summary is not deterministic: %q != %q", again, got)
	}
}

type constantEmbedder struct{ vector []float32 }

func (e constantEmbedder) Model() string { return "constant" }
func (e constantEmbedder) Embed(_ context.Context, texts ...string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = append([]float32(nil), e.vector...)
	}
	return out, nil
}
