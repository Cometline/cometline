package usage_test

import (
	"context"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/usage"
)

func TestUsageRecordSummarySeriesAndList(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := usage.NewService(sqlDB)
	now := time.Now()
	from := now.Add(-48 * time.Hour).UnixMilli()
	to := now.Add(time.Hour).UnixMilli()

	if err := svc.Record(ctx, usage.Event{
		ProviderID: "anthropic",
		ModelID:    "claude-sonnet-4-5",
		CallKind:   usage.KindAgentStep,
		Usage:      cometsdk.TokenUsage{InputTokens: 1000, OutputTokens: 200},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, usage.Event{
		ProviderID: "openai",
		ModelID:    "text-embedding-3-small",
		CallKind:   usage.KindEmbedding,
		Usage:      cometsdk.TokenUsage{InputTokens: 4000},
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := svc.Summary(ctx, from, to, "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Totals.Tokens != 5200 {
		t.Fatalf("tokens=%d want 5200", summary.Totals.Tokens)
	}
	if len(summary.ByModel) != 2 {
		t.Fatalf("by_model=%d", len(summary.ByModel))
	}
	if len(summary.ByKind) != 2 {
		t.Fatalf("by_kind=%d", len(summary.ByKind))
	}

	series, err := svc.Series(ctx, from, to, "", usage.GroupByKind, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Keys) != 2 {
		t.Fatalf("series keys=%v", series.Keys)
	}
	last := series.Points[len(series.Points)-1]
	if last.Cumulative[usage.KindAgentStep] != 1200 {
		t.Fatalf("cumulative agent=%d", last.Cumulative[usage.KindAgentStep])
	}

	page, err := svc.List(ctx, from, to, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("page total=%d items=%d", page.Total, len(page.Items))
	}
}

func TestUsageSeriesKeysIncludeProvider(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := usage.NewService(sqlDB)
	now := time.Now()
	from := now.Add(-time.Hour).UnixMilli()
	to := now.Add(time.Hour).UnixMilli()
	for _, ev := range []usage.Event{
		{ProviderID: "openai", ModelID: "gpt-4o", CallKind: usage.KindAgentStep, Usage: cometsdk.TokenUsage{InputTokens: 10}},
		{ProviderID: "opencode-go", ModelID: "gpt-4o", CallKind: usage.KindAgentStep, Usage: cometsdk.TokenUsage{InputTokens: 20}},
	} {
		if err := svc.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	series, err := svc.Series(ctx, from, to, "", usage.GroupByModel, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Keys) != 2 {
		t.Fatalf("keys=%v", series.Keys)
	}
}

func TestUsageSeriesUsesTimezoneOffset(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	created := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC).UnixMilli()
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO usage_events (id, created_at, provider_id, model_id, call_kind, input_tokens, output_tokens, cache_read, cache_write)
		VALUES ('evt-tz', ?, 'openai', 'gpt-4o', 'agent_step', 100, 0, 0, 0)`, created); err != nil {
		t.Fatal(err)
	}
	svc := usage.NewService(sqlDB)
	from := created - time.Hour.Milliseconds()
	to := created + time.Hour.Milliseconds()
	utc, err := svc.Series(ctx, from, to, "", usage.GroupByModel, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSeriesDay(utc, "2026-08-18") {
		t.Fatalf("utc days=%v", seriesDates(utc))
	}
	west, err := svc.Series(ctx, from, to, "", usage.GroupByModel, -300)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSeriesDay(west, "2026-08-17") {
		t.Fatalf("utc-5 days=%v", seriesDates(west))
	}
}

func TestUsageWorkspaceFilterIncludesSessionScopedRows(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sessions := session.New(sqlDB)
	ws, err := sessions.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.NewSession(ctx, ws.ID, "claude-sonnet-4-5", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	svc := usage.NewService(sqlDB)
	if err := svc.Record(ctx, usage.Event{
		SessionID:  sess.ID,
		ProviderID: "anthropic",
		ModelID:    "claude-sonnet-4-5",
		CallKind:   usage.KindMemoryExtract,
		Usage:      cometsdk.TokenUsage{InputTokens: 40},
	}); err != nil {
		t.Fatal(err)
	}
	from := sess.CreatedAt - 1
	to := time.Now().Add(time.Hour).UnixMilli()
	summary, err := svc.Summary(ctx, from, to, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Totals.Tokens != 40 {
		t.Fatalf("workspace-filtered tokens=%d, want 40", summary.Totals.Tokens)
	}
}

func TestUsageClampRangeAndPurgeOlderThan(t *testing.T) {
	from, to := usage.ClampRange(0, 9_999_999_999_999)
	if to-from > usage.RangeMS(usage.MaxRangeDays) {
		t.Fatalf("clamped span too large: %d", to-from)
	}

	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	old := time.Now().AddDate(-2, 0, 0).UnixMilli()
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO usage_events (id, created_at, provider_id, model_id, call_kind, input_tokens, output_tokens, cache_read, cache_write)
		VALUES ('old', ?, 'openai', 'gpt-4o', 'agent_step', 10, 0, 0, 0),
		       ('new', ?, 'openai', 'gpt-4o', 'agent_step', 20, 0, 0, 0)`, old, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	svc := usage.NewService(sqlDB)
	n, err := svc.PurgeOlderThan(ctx, usage.RetentionDays)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged=%d want 1", n)
	}
	page, err := svc.List(ctx, 0, time.Now().Add(time.Hour).UnixMilli(), "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].ID != "new" {
		t.Fatalf("kept %+v", page.Items)
	}
}

func TestUsageInclusiveCacheDoesNotDoubleCount(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := usage.NewService(sqlDB)
	now := time.Now()
	from := now.Add(-time.Hour).UnixMilli()
	to := now.Add(time.Hour).UnixMilli()
	if err := svc.Record(ctx, usage.Event{
		ProviderID: "openai",
		ModelID:    "gpt-4o",
		CallKind:   usage.KindAgentStep,
		Usage:      cometsdk.TokenUsage{InputTokens: 1000, OutputTokens: 50, CacheRead: 200},
	}); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.Summary(ctx, from, to, "")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Totals.Tokens != 1050 {
		t.Fatalf("tokens=%d want 1050 (fresh 800 + cache 200 + out 50)", summary.Totals.Tokens)
	}
	if summary.Totals.BilledInput != 800 {
		t.Fatalf("billed_input=%d want 800", summary.Totals.BilledInput)
	}
	if summary.Totals.CacheRead != 200 {
		t.Fatalf("cache_read=%d want 200", summary.Totals.CacheRead)
	}
	if summary.ByModel[0].CacheRead != 200 {
		t.Fatalf("cache_read=%d", summary.ByModel[0].CacheRead)
	}
	if summary.ByModel[0].BilledInput != 800 {
		t.Fatalf("model billed_input=%d", summary.ByModel[0].BilledInput)
	}
	page, err := svc.List(ctx, from, to, "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Items[0].InputTokens != 1000 || page.Items[0].CacheRead != 200 {
		t.Fatalf("raw row=%+v", page.Items[0])
	}
	if page.Items[0].BilledInput != 800 {
		t.Fatalf("billed input=%d", page.Items[0].BilledInput)
	}
}

func TestUsageRecordSkipsAllZeroTokens(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	svc := usage.NewService(sqlDB)
	if err := svc.Record(ctx, usage.Event{
		ProviderID: "anthropic",
		ModelID:    "claude-sonnet-4-5",
		CallKind:   usage.KindMemoryExtract,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Record(ctx, usage.Event{
		ProviderID: "anthropic",
		ModelID:    "claude-sonnet-4-5",
		CallKind:   usage.KindMemoryExtract,
		Usage:      cometsdk.TokenUsage{InputTokens: 12, OutputTokens: 3},
	}); err != nil {
		t.Fatal(err)
	}
	page, err := svc.List(ctx, 0, time.Now().Add(time.Hour).UnixMilli(), "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].InputTokens != 12 {
		t.Fatalf("page=%+v", page)
	}
}

func hasSeriesDay(series usage.Series, day string) bool {
	for _, point := range series.Points {
		if point.Date == day && point.DayTokens["openai/gpt-4o"] == 100 {
			return true
		}
	}
	return false
}

func seriesDates(series usage.Series) []string {
	out := make([]string, 0, len(series.Points))
	for _, point := range series.Points {
		out = append(out, point.Date)
	}
	return out
}
