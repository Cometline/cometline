package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/runstate"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/usage"
)

func TestUsageHandlersSummarySeriesAndEvents(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	sessions := session.New(sqlDB)
	usageSvc := usage.NewService(sqlDB)
	if err := usageSvc.Record(ctx, usage.Event{
		ProviderID: "anthropic",
		ModelID:    "claude-sonnet-4-5",
		CallKind:   usage.KindAgentStep,
		Usage:      cometsdk.TokenUsage{InputTokens: 100, OutputTokens: 20},
	}); err != nil {
		t.Fatal(err)
	}

	engine, err := New(Deps{
		Config:    config.Defaults(),
		Sessions:  sessions,
		Usage:     usageSvc,
		Runs:      NewRunManager(runstate.New(sqlDB)),
		NewRunner: func(session.Session, string, session.AgentMode) (Runner, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/summary", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("summary status=%d body=%s", w.Code, w.Body.String())
	}
	var summary struct {
		Totals struct {
			Tokens int `json:"tokens"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Totals.Tokens != 120 {
		t.Fatalf("tokens=%d", summary.Totals.Tokens)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/usage/series?group_by=kind", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("series status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("events status=%d body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("total=%d", page.Total)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/usage/series?from=0&to=9999999999999", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("extreme range status=%d body=%s", w.Code, w.Body.String())
	}
	var series struct {
		Points []struct {
			Date string `json:"date"`
		} `json:"points"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &series); err != nil {
		t.Fatal(err)
	}
	if len(series.Points) == 0 || len(series.Points) > usage.MaxRangeDays {
		t.Fatalf("clamped series days=%d", len(series.Points))
	}
}
