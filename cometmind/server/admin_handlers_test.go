package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/processctl"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/gin-gonic/gin"
)

func TestAdminListProcesses(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	if err := processctl.WriteMetadata(processctl.ModeServe); err != nil {
		t.Fatalf("WriteMetadata() error = %v", err)
	}
	defer processctl.RemoveMetadata(processctl.ModeServe)

	engine, cleanup := newAdminTestEngine(t, nil, nil)
	defer cleanup()

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/processes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/admin/processes status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got listAdminProcessesResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(got.Processes))
	}
	if got.Processes[0].Mode != processctl.ModeServe {
		t.Fatalf("expected first mode %q, got %#v", processctl.ModeServe, got.Processes)
	}
	if !got.Processes[0].Present || !got.Processes[0].Running {
		t.Fatalf("expected serve process to be running, got %#v", got.Processes[0])
	}
	if got.Processes[0].DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, got.Processes[0].DataDir)
	}
	if got.Processes[1].Mode != processctl.ModeGatewayDiscord || got.Processes[1].Present {
		t.Fatalf("expected gateway-discord to be absent, got %#v", got.Processes[1])
	}
}

func TestAdminReloadCallsRuntimeReload(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	called := 0
	engine, cleanup := newAdminTestEngine(t, func(context.Context) error {
		called++
		return nil
	}, nil)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reload-runs", strings.NewReader(`{"modes":["serve"]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/admin/reload-runs status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called != 1 {
		t.Fatalf("expected reload callback once, got %d", called)
	}
}

func TestAdminRestartAcceptsServeStop(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	stopped := 0
	engine, cleanup := newAdminTestEngine(t, nil, func() { stopped++ })
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/restart-runs", strings.NewReader(`{"modes":["serve"]}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/admin/restart-runs status=%d body=%s", rec.Code, rec.Body.String())
	}
	if stopped != 1 {
		t.Fatalf("expected stop callback once, got %d", stopped)
	}
}

func newAdminTestEngine(t *testing.T, reload func(context.Context) error, stop func()) (*gin.Engine, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlDB, err := store.OpenSQLite(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	engine, err := New(Deps{
		Config:   config.Defaults(),
		Sessions: session.New(sqlDB),
		NewRunner: func(session.Session, string) (Runner, error) {
			return &noopRunner{}, nil
		},
		Runs:          NewRunManager(),
		ReloadRuntime: reload,
		RequestStop:   stop,
	})
	if err != nil {
		_ = sqlDB.Close()
		t.Fatalf("New() error = %v", err)
	}
	return engine, func() {
		_ = sqlDB.Close()
	}
}
