package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestListWorkspaces(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`}`))
	createReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create workspace status = %d, want %d body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	engine.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list workspaces status = %d, want %d body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}

	var got listWorkspacesResponse
	decodeJSON(t, listRec.Body.Bytes(), &got)
	if len(got.Workspaces) == 0 {
		t.Fatal("expected at least one workspace")
	}
	found := false
	for _, ws := range got.Workspaces {
		if ws.Path == filepath.Clean(workspacePath) {
			found = true
			if ws.SessionCount != 0 {
				t.Fatalf("session_count = %d, want 0", ws.SessionCount)
			}
			break
		}
	}
	if !found {
		t.Fatalf("workspaces = %+v, want path %q", got.Workspaces, workspacePath)
	}
}

func TestDeleteWorkspace(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`}`))
	createReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create workspace status = %d, want %d body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces?workspace_path="+url.QueryEscape(workspacePath), nil)
	engine.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete workspace status = %d, want %d body=%s", deleteRec.Code, http.StatusNoContent, deleteRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	engine.ServeHTTP(listRec, listReq)
	var got listWorkspacesResponse
	decodeJSON(t, listRec.Body.Bytes(), &got)
	for _, ws := range got.Workspaces {
		if ws.Path == filepath.Clean(workspacePath) {
			t.Fatalf("workspace still listed after delete: %+v", ws)
		}
	}
}

func TestDeleteWorkspaceWithSessions(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`}`))
	createReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create workspace status = %d, want %d body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	sessionRec := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`,"model_id":"m","provider_id":"p"}`))
	sessionReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusCreated {
		t.Fatalf("create session status = %d, want %d body=%s", sessionRec.Code, http.StatusCreated, sessionRec.Body.String())
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces?workspace_path="+url.QueryEscape(workspacePath), nil)
	engine.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusConflict {
		t.Fatalf("delete workspace status = %d, want %d body=%s", deleteRec.Code, http.StatusConflict, deleteRec.Body.String())
	}
}

func TestListWorkspacesOmitsMissingPath(t *testing.T) {
	t.Parallel()

	_, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	root := t.TempDir()
	alive := filepath.Join(root, "alive")
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(alive, 0o755); err != nil {
		t.Fatalf("MkdirAll(alive) error = %v", err)
	}
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatalf("MkdirAll(gone) error = %v", err)
	}
	if _, err := svc.EnsureWorkspace(ctx, alive); err != nil {
		t.Fatalf("EnsureWorkspace(alive) error = %v", err)
	}
	if _, err := svc.EnsureWorkspace(ctx, gone); err != nil {
		t.Fatalf("EnsureWorkspace(gone) error = %v", err)
	}

	list, err := svc.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListWorkspaces() len = %d, want 2", len(list))
	}

	if err := os.RemoveAll(gone); err != nil {
		t.Fatalf("RemoveAll(gone) error = %v", err)
	}

	list, err = svc.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("ListWorkspaces() after delete error = %v", err)
	}
	if len(list) != 1 || list[0].Path != filepath.Clean(alive) {
		t.Fatalf("ListWorkspaces() = %+v, want only %q", list, alive)
	}

	pruned, err := svc.PruneMissingWorkspaces(ctx)
	if err != nil {
		t.Fatalf("PruneMissingWorkspaces() error = %v", err)
	}
	if pruned != 1 {
		t.Fatalf("PruneMissingWorkspaces() = %d, want 1", pruned)
	}
}

func TestPruneWorkspacesEndpoint(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	root := t.TempDir()
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatalf("MkdirAll(gone) error = %v", err)
	}
	if _, err := svc.EnsureWorkspace(ctx, gone); err != nil {
		t.Fatalf("EnsureWorkspace(gone) error = %v", err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatalf("RemoveAll(gone) error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/prune-runs", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got pruneWorkspacesResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Pruned != 1 {
		t.Fatalf("pruned = %d, want 1", got.Pruned)
	}
}

func TestLookupModelCatalogEndpoint(t *testing.T) {
	engine, _, cleanup := newTestEngine(t, func(session.Session, string, session.AgentMode) (Runner, error) {
		return fakeRunner(func(context.Context, session.AgentTurn, chan<- event.Event) error {
			return nil
		}), nil
	})
	defer cleanup()

	body, _ := json.Marshal(map[string]any{
		"method":    "openai-compatible",
		"model_ids": []string{"llama3", "llama3"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/models/catalog-lookups", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got apigen.ModelCatalogLookupResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Models) != 1 {
		t.Fatalf("models len = %d, want 1", len(got.Models))
	}
	if got.Models[0].VisionKnown {
		t.Fatal("openai-compatible must not claim vision_known")
	}
	if got.Models[0].LimitSource != apigen.Fallback {
		t.Fatalf("limit_source = %q", got.Models[0].LimitSource)
	}
}

func TestRemovedUnusedEndpointsReturnNotFound(t *testing.T) {
	engine, _, cleanup := newTestEngine(t, func(session.Session, string, session.AgentMode) (Runner, error) {
		return fakeRunner(func(context.Context, session.AgentTurn, chan<- event.Event) error {
			return nil
		}), nil
	})
	defer cleanup()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/models"},
		{method: http.MethodPatch, path: "/api/v1/sessions/session-id/workspace"},
		{method: http.MethodPatch, path: "/api/v1/memories/memory-id"},
		{method: http.MethodGet, path: "/api/v1/inbox/messages/message-id"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
		})
	}
}
