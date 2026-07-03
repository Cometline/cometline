package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/scheduler"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/gin-gonic/gin"
)

func newScheduledJobTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	ctx := context.Background()
	sqlDB, err := store.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sessions := session.New(sqlDB)
	engine, err := New(Deps{
		Config:    config.Defaults(),
		Sessions:  sessions,
		Jobs:      jobs.NewService(sqlDB, nil, nil),
		Scheduler: scheduler.NewService(sqlDB),
		NewRunner: func(session.Session, string) (Runner, error) { return nil, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestScheduledJobHandlersCRUD(t *testing.T) {
	engine := newScheduledJobTestServer(t)
	runAt := int64(3_000_000)
	createBody := `{"description":"ship release notes","definition_of_done":"notes shipped","workspace_path":"/tmp/project","run_at":3000000}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduled-jobs", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		RunAt       *int64 `json:"run_at"`
		NextRunAt   int64  `json:"next_run_at"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Description != "ship release notes" || created.RunAt == nil || *created.RunAt != runAt || created.NextRunAt != runAt || !created.Enabled {
		t.Fatalf("created=%+v body=%s", created, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-jobs", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), created.ID) {
		t.Fatalf("list body missing schedule id: %s", w.Body.String())
	}

	patchBody := `{"description":"ship better release notes","run_at":3001000,"enabled":false}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/scheduled-jobs/"+created.ID, bytes.NewBufferString(patchBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", w.Code, w.Body.String())
	}
	var updated struct {
		Description string `json:"description"`
		NextRunAt   int64  `json:"next_run_at"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Description != "ship better release notes" || updated.NextRunAt != 3_001_000 || updated.Enabled {
		t.Fatalf("updated=%+v", updated)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-jobs/"+created.ID, nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/scheduled-jobs/"+created.ID, nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestScheduledJobHandlersRejectBothCronAndRunAt(t *testing.T) {
	engine := newScheduledJobTestServer(t)
	body := `{"description":"repeat","cron_expr":"* * * * *","run_at":3000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scheduled-jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
