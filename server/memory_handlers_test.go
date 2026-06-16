package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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

	engine, err := New(Deps{Config: cfg, Sessions: sessions, Memory: mem, NewRunner: func(session.Session, string) (Runner, error) {
		return &noopRunner{}, nil
	}, Runs: NewRunManager()})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/memory/settings", nil))
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
	req := httptest.NewRequest(http.MethodPut, "/api/v1/memory/settings", bytes.NewReader(body))
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
}

type noopRunner struct{}

func (noopRunner) Run(context.Context, session.AgentTurn, chan<- event.Event) error { return nil }
