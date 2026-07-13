package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/gin-gonic/gin"
)

type memFakeProvider struct{}

func (memFakeProvider) ID() string { return "fake" }

func (memFakeProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, 1)
	close(ch)
	return ch, nil
}

func TestMemorySettingsGetPut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "mem-test.db")
	sqlDB, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	cfg := config.Defaults()
	cfg.Memory.Enabled = true
	sessions := session.New(sqlDB)
	mem, err := memory.NewService(sqlDB, cfg.MemorySettings(), memFakeProvider{}, sessions)
	if err != nil {
		t.Fatal(err)
	}

	hub := event.NewHub()
	engine, err := New(Deps{Config: cfg, Sessions: sessions, Memory: mem, Events: hub, NewRunner: func(session.Session, string) (Runner, error) {
		return &noopRunner{}, nil
	}, Runs: NewRunManager()})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/memories/settings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET settings: %d %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["enabled"]; !ok {
		t.Fatalf("expected snake_case enabled key, got %v", payload)
	}

	body, _ := json.Marshal(map[string]any{
		"enabled":              false,
		"auto_extract":         true,
		"auto_retrieve":        true,
		"max_retrieved":        cfg.Memory.MaxRetrieved,
		"similarity_threshold": cfg.Memory.SimilarityThreshold,
		"lifecycle":            cfg.Memory.Lifecycle,
		"embedding":            cfg.Memory.Embedding,
	})
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/memories/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT settings: %d %s", rec.Code, rec.Body.String())
	}
	var updated config.MemoryConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Enabled {
		t.Fatal("expected enabled=false")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/memories/purge-runs", bytes.NewBufferString(`{"older_than_days":0}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST purge: %d %s", rec.Code, rec.Body.String())
	}
	var purged purgeMemoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &purged); err != nil {
		t.Fatal(err)
	}
	if purged.Status != "ok" || purged.MemoriesPurged != 0 || purged.MemoryEventsPurged != 0 {
		t.Fatalf("unexpected purge response: %+v", purged)
	}

	sub := hub.Subscribe()
	defer sub.Close()
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/memories/compaction-runs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST compaction: %d %s", rec.Code, rec.Body.String())
	}
	var compacted struct {
		Status  string `json:"status"`
		Before  int64  `json:"before"`
		After   int64  `json:"after"`
		Trigger string `json:"trigger"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &compacted); err != nil {
		t.Fatal(err)
	}
	if compacted.Status != "ok" || compacted.Before != 0 || compacted.After != 0 || compacted.Trigger != "manual" {
		t.Fatalf("unexpected compaction response: %+v", compacted)
	}
	select {
	case got := <-sub.Events:
		if got.Kind != event.KindMemoryCompactionCompleted || got.CompactionTrigger != "manual" {
			t.Fatalf("unexpected compaction event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for compaction event")
	}
}

type noopRunner struct{}

func (noopRunner) Run(context.Context, session.AgentTurn, chan<- event.Event) error { return nil }
