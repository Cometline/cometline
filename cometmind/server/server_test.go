package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/contract"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/store"
	"github.com/gin-gonic/gin"
)

type fakeRunner func(context.Context, session.AgentTurn, chan<- event.Event) error

func (f fakeRunner) Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
	return f(ctx, turn, ch)
}

func TestCreateSessionAutoRegistersWorkspacePath(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`}`))
	req.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got sessionResource
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.WorkspacePath != filepath.Clean(workspacePath) {
		t.Fatalf("workspace_path = %q, want %q", got.WorkspacePath, filepath.Clean(workspacePath))
	}
	if got.WorkspaceID == "" || got.ID == "" {
		t.Fatalf("expected workspace and session ids to be populated: %+v", got)
	}

	list, err := svc.ListSessions(context.Background(), got.WorkspaceID)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("persisted sessions = %+v, want created session %q", list, got.ID)
	}
}

func TestMissingSessionEndpointsReturnSessionNotFound(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "messages",
			method: http.MethodGet,
			path:   "/api/v1/sessions/missing/messages",
		},
		{
			name:   "children",
			method: http.MethodGet,
			path:   "/api/v1/sessions/missing/children",
		},
		{
			name:   "message",
			method: http.MethodPost,
			path:   "/api/v1/sessions/missing/message",
			body:   `{"text":"hello"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
			var got errorResponse
			decodeJSON(t, rec.Body.Bytes(), &got)
			if got.Error.Code != "session_not_found" {
				t.Fatalf("error code = %q, want session_not_found", got.Error.Code)
			}
		})
	}
}

func TestCreateWorkspaceRegistersPath(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewBufferString(`{"workspace_path":`+mustJSON(workspacePath)+`}`))
	req.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got workspaceResource
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Path != filepath.Clean(workspacePath) {
		t.Fatalf("workspace path = %q, want %q", got.Path, filepath.Clean(workspacePath))
	}
	if got.ID == "" {
		t.Fatalf("expected workspace id to be populated: %+v", got)
	}

	ws, err := svc.LookupWorkspaceByPath(context.Background(), workspacePath)
	if err != nil {
		t.Fatalf("LookupWorkspaceByPath() error = %v", err)
	}
	if ws.ID != got.ID {
		t.Fatalf("lookup workspace id = %q, want %q", ws.ID, got.ID)
	}
}

func TestListWorkspaceFiles(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")
	mustWrite(t, filepath.Join(workspacePath, "README.md"), "# readme")
	mustWrite(t, filepath.Join(workspacePath, ".hidden", "secret.go"), "package hidden")
	mustWrite(t, filepath.Join(workspacePath, "node_modules", "x", "index.js"), "x")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files?workspace_path="+workspacePath, nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	want := []string{"README.md", "main.go"}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %v, want %v", got.Files, want)
	}
	for i, w := range want {
		if got.Files[i] != w {
			t.Fatalf("files = %v, want %v", got.Files, want)
		}
	}

	// Verify the workspace was registered.
	_, err := svc.LookupWorkspaceByPath(context.Background(), workspacePath)
	if err != nil {
		t.Fatalf("LookupWorkspaceByPath() error = %v", err)
	}
}

func TestReadWorkspaceFileContent(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files/content?workspace_path="+workspacePath+"&path=main.go", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workspaceFileTextContent
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Kind != "text" || got.Content != "package main" || got.Extension != ".go" {
		t.Fatalf("got = %+v", got)
	}
}

func TestWriteWorkspaceFileContent(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	filePath := filepath.Join(workspacePath, "main.go")
	mustWrite(t, filePath, "package main")

	body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"main.go","content":"package main\n\nfunc main() {}"}`, workspacePath))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(data) != "package main\n\nfunc main() {}" {
		t.Fatalf("file content = %q", string(data))
	}

	readRec := httptest.NewRecorder()
	readReq := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files/content?workspace_path="+workspacePath+"&path=main.go", nil)
	engine.ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("read status = %d, want %d body=%s", readRec.Code, http.StatusOK, readRec.Body.String())
	}

	var got workspaceFileTextContent
	decodeJSON(t, readRec.Body.Bytes(), &got)
	if got.Content != "package main\n\nfunc main() {}" {
		t.Fatalf("round trip content = %q", got.Content)
	}
}

func TestWriteWorkspaceFileContentRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")

	t.Run("path escape", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"../outside.go","content":"package main"}`, workspacePath))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("oversized content", func(t *testing.T) {
		content := strings.Repeat("a", maxMessageFileBytes+1)
		body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"main.go","content":%q}`, workspacePath, content))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("missing file", func(t *testing.T) {
		body := bytes.NewBufferString(fmt.Sprintf(`{"workspace_path":%q,"path":"missing.go","content":"package main"}`, workspacePath))
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/files/content", body)
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})
}

func TestListWorkspaceFilesFiltersByQuery(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main")
	mustWrite(t, filepath.Join(workspacePath, "README.md"), "# readme")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files?workspace_path="+workspacePath+"&q=go", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got workspaceFileListResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	want := []string{"main.go"}
	assertSlice(t, got.Files, want)
}

func TestListWorkspaceFilesMissingWorkspace(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/files", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func assertSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestListSessionsRequiresWorkspaceScope(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	workspacePath := t.TempDir()
	ws, err := svc.EnsureWorkspace(ctx, workspacePath)
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status without scope = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sessions?workspace_id="+ws.ID, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status with scope = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got listSessionsResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Sessions) != 1 || got.Sessions[0].ID != sess.ID {
		t.Fatalf("sessions = %+v, want session %q", got.Sessions, sess.ID)
	}
}

func TestDeleteSessionRemovesSession(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "remove me"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+sess.ID, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	list, err := svc.ListSessions(ctx, ws.ID)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("sessions after delete = %+v, want none", list)
	}
}

func TestClearSessionResetsTranscript(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "hello"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}
	if err := svc.UpdateContextSummary(ctx, sess.ID, "old summary", "ignored"); err != nil {
		t.Fatalf("UpdateContextSummary() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/clear", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	msgs, err := svc.ListMessageRows(ctx, sess.ID)
	if err != nil {
		t.Fatalf("ListMessageRows() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("messages after clear = %d, want 0", len(msgs))
	}
	got, err := svc.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.ContextSummary != "" || got.CompactedUntilMessageID != "" || got.Title != "" {
		t.Fatalf("session state after clear = %+v", got)
	}
}

func TestClearSessionRemovesChildSessions(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	parent, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, parent.ID, "delegate something"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}
	if _, err := svc.NewChildSession(ctx, parent, "refactor auth module", "acp"); err != nil {
		t.Fatalf("NewChildSession() error = %v", err)
	}
	children, err := svc.ListChildSessions(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListChildSessions() before clear error = %v", err)
	}
	if len(children) != 1 {
		t.Fatalf("children before clear = %d, want 1", len(children))
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+parent.ID+"/clear", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("clear status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	msgs, err := svc.ListMessageRows(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListMessageRows() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("messages after clear = %d, want 0", len(msgs))
	}
	children, err = svc.ListChildSessions(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListChildSessions() after clear error = %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("children after clear = %d, want 0", len(children))
	}
}

func TestPatchSessionUpdatesModel(t *testing.T) {
	t.Parallel()

	var gotTurn session.AgentTurn
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			gotTurn = turn
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "old-model", "old-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/sessions/"+sess.ID,
		bytes.NewBufferString(`{"model_id":"new-model","provider_id":"new-provider"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var patched sessionResource
	decodeJSON(t, rec.Body.Bytes(), &patched)
	if patched.ModelID != "new-model" || patched.ProviderID != "new-provider" {
		t.Fatalf("patched session = %+v, want new model/provider", patched)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/message", bytes.NewBufferString(`{"text":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("message status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotTurn.ModelID != "new-model" || gotTurn.ProviderID != "new-provider" {
		t.Fatalf("runner turn = %+v, want new-model/new-provider", gotTurn)
	}
}

func TestPatchSessionUpdatesPinned(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	older, err := svc.NewSession(ctx, ws.ID, "model-a", "provider-a")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	newer, err := svc.NewSession(ctx, ws.ID, "model-b", "provider-b")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/sessions/"+older.ID,
		bytes.NewBufferString(`{"pinned":true}`),
	)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch pin status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var pinned sessionResource
	decodeJSON(t, rec.Body.Bytes(), &pinned)
	if !pinned.Pinned {
		t.Fatalf("patched session pinned = false, want true")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/sessions?workspace_id="+ws.ID,
		nil,
	)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var listed listSessionsResponse
	decodeJSON(t, rec.Body.Bytes(), &listed)
	if len(listed.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(listed.Sessions))
	}
	if listed.Sessions[0].ID != older.ID {
		t.Fatalf("first session = %s, want pinned older session %s", listed.Sessions[0].ID, older.ID)
	}
	if listed.Sessions[1].ID != newer.ID {
		t.Fatalf("second session = %s, want newer session %s", listed.Sessions[1].ID, newer.ID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/sessions/"+older.ID,
		bytes.NewBufferString(`{"pinned":false}`),
	)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch unpin status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	decodeJSON(t, rec.Body.Bytes(), &pinned)
	if pinned.Pinned {
		t.Fatalf("patched session pinned = true, want false")
	}
}

func TestPatchSessionUpdatesTitle(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "model-a", "provider-a")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/sessions/"+sess.ID,
		bytes.NewBufferString(`{"title":"Renamed chat"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch title status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var renamed sessionResource
	decodeJSON(t, rec.Body.Bytes(), &renamed)
	if renamed.Title != "Renamed chat" {
		t.Fatalf("patched session title = %q, want %q", renamed.Title, "Renamed chat")
	}
}

func TestGetMessagesReturnsTranscriptItems(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, sess.ID, "inspect main.go"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}

	toolCall := cometsdk.ToolCallBlock{
		ID:    "provider-tool-1",
		Name:  "read_file",
		Input: json.RawMessage(`{"path":"main.go"}`),
	}
	_, toolIDs, err := svc.AppendAssistantStep(
		ctx,
		sess.ID,
		"Found it.",
		[]cometsdk.Block{cometsdk.ReasoningBlock{Text: "Need to inspect the entrypoint first."}},
		[]cometsdk.ToolCallBlock{toolCall},
		nil,
	)
	if err != nil {
		t.Fatalf("AppendAssistantStep() error = %v", err)
	}
	persistedToolID := toolIDs[toolCall.ID]
	if persistedToolID == "" {
		t.Fatalf("missing persisted tool id for %q", toolCall.ID)
	}
	if err := svc.UpdateToolCallResult(ctx, persistedToolID, "package main", 5, nil); err != nil {
		t.Fatalf("UpdateToolCallResult() error = %v", err)
	}
	if _, err := svc.AppendToolResultMessage(ctx, sess.ID, persistedToolID, "package main", false); err != nil {
		t.Fatalf("AppendToolResultMessage() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID+"/messages", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got transcriptResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Items) != 4 {
		t.Fatalf("items len = %d, want 4 (%+v)", len(got.Items), got.Items)
	}
	if got.Items[0].Type != "user" || got.Items[1].Type != "reasoning" || got.Items[2].Type != "tool" || got.Items[3].Type != "assistant" {
		t.Fatalf("item types = %+v", got.Items)
	}
	inputMap, ok := got.Items[2].ToolInput.(map[string]any)
	if !ok || inputMap["path"] != "main.go" {
		t.Fatalf("tool input = %#v, want path main.go", got.Items[2].ToolInput)
	}
	if got.Items[2].ToolOutput != "package main" {
		t.Fatalf("tool output = %q, want %q", got.Items[2].ToolOutput, "package main")
	}
}

func TestPostMessageStreamsSSEAndPersistsUserTurn(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.ReasoningStart()
			ch <- event.TextDelta("hello")
			ch <- event.ToolCall("tool-1", "read_file", []byte(`{"path":"main.go"}`))
			ch <- event.ToolResult("tool-1", "read_file", "package main", "")
			ch <- event.StepFinish(cometsdk.TokenUsage{InputTokens: 10, OutputTokens: 4, CacheRead: 1})
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/message", bytes.NewBufferString(`{"text":"hello from api"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"type":"reasoning_start"`,
		`"type":"text_delta","delta":"hello"`,
		`"type":"tool_call","id":"tool-1","tool":"read_file","input":{"path":"main.go"}`,
		`"type":"tool_result","id":"tool-1","tool":"read_file","output":"package main"`,
		`"type":"step_finish","usage":{"input_tokens":10,"output_tokens":4,"cache_read":1,"cache_write":0}`,
		`"type":"done"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream body missing %q:\n%s", want, body)
		}
	}

	for _, frame := range parseSSEDataFrames(body) {
		if err := contract.ValidateStreamEventJSON(frame); err != nil {
			t.Fatalf("ValidateStreamEventJSON() error = %v\nframe: %s", err, frame)
		}
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages len = %d, want 1", len(msgs))
	}
	if text := msgs[0].Content[0].(cometsdk.TextBlock).Text; text != "hello from api" {
		t.Fatalf("persisted user text = %q, want %q", text, "hello from api")
	}
	updated, err := svc.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if updated.Title != "hello from api" {
		t.Fatalf("session title = %q, want %q", updated.Title, "hello from api")
	}
}

func TestPostMessageInlinesFilePaths(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	workspacePath := t.TempDir()
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main\n")
	mustWrite(t, filepath.Join(workspacePath, "missing.go"), "")
	_ = os.Remove(filepath.Join(workspacePath, "missing.go"))

	ws, err := svc.EnsureWorkspace(ctx, workspacePath)
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	body := `{"text":"review @main.go and @missing.go","file_paths":["main.go","missing.go","main.go"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/message", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("messages len = %d, want 1", len(msgs))
	}
	text := msgs[0].Content[0].(cometsdk.TextBlock).Text
	if !strings.Contains(text, "review @main.go and @missing.go") {
		t.Fatalf("user text missing original prompt: %q", text)
	}
	if !strings.Contains(text, "[File: main.go]") {
		t.Fatalf("user text missing main.go content: %q", text)
	}
	if !strings.Contains(text, "package main") {
		t.Fatalf("user text missing main.go source: %q", text)
	}
	if !strings.Contains(text, "Could not include missing.go") {
		t.Fatalf("user text missing missing.go error note: %q", text)
	}
	if strings.Count(text, "[File: main.go]") != 1 {
		t.Fatalf("main.go should be inlined once, got: %q", text)
	}
}

func TestLocalCORSAllowsCometlineRenderer(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want renderer origin", got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, "/api/v1/sessions", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) || !strings.Contains(got, http.MethodPut) || !strings.Contains(got, http.MethodDelete) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want POST, PUT, and DELETE", got)
	}
}

func TestLocalCORSAllowsPackagedCometlineOrigin(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "app://bundle")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "app://bundle" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want packaged app origin", got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, "/api/v1/sessions", nil)
	req.Header.Set("Origin", "app://bundle")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) || !strings.Contains(got, http.MethodPut) || !strings.Contains(got, http.MethodDelete) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want POST, PUT, and DELETE", got)
	}
}

func TestLocalCORSAllowsMemorySettingsPut(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/memory/settings", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPut) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want PUT", got)
	}
}

func TestAbortSessionCancelsRunningStream(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	cancelled := make(chan struct{})
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			close(started)
			ch <- event.TextDelta("working")
			<-ctx.Done()
			close(cancelled)
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	srv := httptest.NewServer(engine)
	defer srv.Close()

	type responseResult struct {
		Status int
		Body   string
		Err    error
	}
	streamDone := make(chan responseResult, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/sessions/"+sess.ID+"/message", bytes.NewBufferString(`{"text":"hello"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			streamDone <- responseResult{Err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		streamDone <- responseResult{Status: resp.StatusCode, Body: string(body), Err: err}
	}()

	<-started

	abortReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/sessions/"+sess.ID+"/abort", nil)
	abortResp, err := http.DefaultClient.Do(abortReq)
	if err != nil {
		t.Fatalf("abort request error = %v", err)
	}
	defer abortResp.Body.Close()
	abortBody, err := io.ReadAll(abortResp.Body)
	if err != nil {
		t.Fatalf("abort body read error = %v", err)
	}
	if abortResp.StatusCode != http.StatusAccepted {
		t.Fatalf("abort status = %d, want %d body=%s", abortResp.StatusCode, http.StatusAccepted, string(abortBody))
	}

	<-cancelled
	got := <-streamDone
	if got.Err != nil {
		t.Fatalf("stream request error = %v", got.Err)
	}
	if got.Status != http.StatusOK {
		t.Fatalf("stream status = %d, want %d body=%s", got.Status, http.StatusOK, got.Body)
	}
	if !strings.Contains(got.Body, `"type":"done"`) {
		t.Fatalf("stream body missing done event:\n%s", got.Body)
	}
}

func TestSkillsDeleteAndExport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	content := "---\nname: api-skill\ndescription: api test skill\n---\n\n# API\n"
	if err := skills.WriteSkill("api-skill", content, false); err != nil {
		t.Fatalf("WriteSkill() error = %v", err)
	}

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	exportRec := httptest.NewRecorder()
	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/skills/api-skill/export", nil)
	engine.ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want %d body=%s", exportRec.Code, http.StatusOK, exportRec.Body.String())
	}
	if ct := exportRec.Header().Get("Content-Type"); !strings.Contains(ct, "application/zip") {
		t.Fatalf("export content-type = %q", ct)
	}
	if exportRec.Body.Len() == 0 {
		t.Fatal("export body is empty")
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/skills/api-skill", nil)
	engine.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d body=%s", deleteRec.Code, http.StatusOK, deleteRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/skills", nil)
	engine.ServeHTTP(listRec, listReq)
	var list listSkillsResponse
	decodeJSON(t, listRec.Body.Bytes(), &list)
	for _, skill := range list.Skills {
		if skill.Name == "api-skill" {
			t.Fatalf("skill still listed after delete: %+v", list.Skills)
		}
	}
}

func TestListWorkspaces(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
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
			break
		}
	}
	if !found {
		t.Fatalf("workspaces = %+v, want path %q", got.Workspaces, workspacePath)
	}
}

func TestListWorkspacesOmitsMissingPath(t *testing.T) {
	t.Parallel()

	_, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
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

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/prune", nil)
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

func TestChangeSessionWorkspace(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws1 := t.TempDir()
	ws2 := t.TempDir()

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(ws1)+`}`))
	createReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create session status = %d, want %d body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created sessionResource
	decodeJSON(t, createRec.Body.Bytes(), &created)

	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+created.ID+"/workspace", bytes.NewBufferString(`{"workspace_path":`+mustJSON(ws2)+`}`))
	patchReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("change workspace status = %d, want %d body=%s", patchRec.Code, http.StatusOK, patchRec.Body.String())
	}

	var updated sessionResource
	decodeJSON(t, patchRec.Body.Bytes(), &updated)
	if updated.WorkspacePath != filepath.Clean(ws2) {
		t.Fatalf("workspace_path = %q, want %q", updated.WorkspacePath, filepath.Clean(ws2))
	}

	sess, err := svc.GetSession(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	path, err := svc.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		t.Fatalf("WorkspacePath() error = %v", err)
	}
	if path != filepath.Clean(ws2) {
		t.Fatalf("persisted workspace path = %q, want %q", path, filepath.Clean(ws2))
	}

	msgRec := httptest.NewRecorder()
	msgReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+created.ID+"/messages", nil)
	engine.ServeHTTP(msgRec, msgReq)
	if msgRec.Code != http.StatusOK {
		t.Fatalf("messages status = %d, want %d body=%s", msgRec.Code, http.StatusOK, msgRec.Body.String())
	}
	var transcript transcriptResponse
	decodeJSON(t, msgRec.Body.Bytes(), &transcript)
	if len(transcript.Items) != 1 || transcript.Items[0].Type != "system" {
		t.Fatalf("transcript items = %+v, want one system notice", transcript.Items)
	}
}

func TestChangeSessionWorkspaceRejectsMissingDirectory(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ws1 := t.TempDir()
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(ws1)+`}`))
	createReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(createRec, createReq)
	var created sessionResource
	decodeJSON(t, createRec.Body.Bytes(), &created)

	patchRec := httptest.NewRecorder()
	missing := filepath.Join(t.TempDir(), "missing-workspace")
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+created.ID+"/workspace", bytes.NewBufferString(`{"workspace_path":`+mustJSON(missing)+`}`))
	patchReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusBadRequest {
		t.Fatalf("change workspace status = %d, want %d body=%s", patchRec.Code, http.StatusBadRequest, patchRec.Body.String())
	}
}

func TestForkSessionCopiesTranscript(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws1 := t.TempDir()
	ws2 := t.TempDir()

	srcWs, err := svc.EnsureWorkspace(ctx, ws1)
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	src, err := svc.NewSession(ctx, srcWs.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.AppendUserMessage(ctx, src.ID, "original message"); err != nil {
		t.Fatalf("AppendUserMessage() error = %v", err)
	}
	if err := svc.SetTitleIfEmpty(ctx, src.ID, "Original title"); err != nil {
		t.Fatalf("SetTitleIfEmpty() error = %v", err)
	}

	forkRec := httptest.NewRecorder()
	forkReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+src.ID+"/fork", bytes.NewBufferString(`{"workspace_path":`+mustJSON(ws2)+`}`))
	forkReq.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(forkRec, forkReq)
	if forkRec.Code != http.StatusCreated {
		t.Fatalf("fork status = %d, want %d body=%s", forkRec.Code, http.StatusCreated, forkRec.Body.String())
	}

	var forked sessionResource
	decodeJSON(t, forkRec.Body.Bytes(), &forked)
	if forked.ID == src.ID {
		t.Fatalf("forked session must have a new id")
	}
	if forked.WorkspacePath != filepath.Clean(ws2) {
		t.Fatalf("forked workspace_path = %q, want %q", forked.WorkspacePath, filepath.Clean(ws2))
	}
	if forked.Title != "Original title" {
		t.Fatalf("forked title = %q, want %q", forked.Title, "Original title")
	}

	// Original session is untouched.
	original, err := svc.GetSession(ctx, src.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if original.WorkspaceID != srcWs.ID {
		t.Fatalf("original workspace changed: %q", original.WorkspaceID)
	}

	// Forked transcript carries the copied message plus a system fork notice.
	forkedMsgs, err := svc.BuildSDKMessages(ctx, forked.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	found := false
	for _, msg := range forkedMsgs {
		for _, block := range msg.Content {
			if text, ok := block.(cometsdk.TextBlock); ok && strings.Contains(text.Text, "original message") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("forked transcript missing original message: %+v", forkedMsgs)
	}
}

func TestListAllSessionsAcrossWorkspaces(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws1, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	ws2, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	if _, err := svc.NewSession(ctx, ws1.ID, "m", "p"); err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.NewSession(ctx, ws2.ID, "m", "p"); err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?all=true", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got listSessionsResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions len = %d, want 2", len(got.Sessions))
	}
	paths := map[string]bool{}
	for _, sess := range got.Sessions {
		paths[sess.WorkspacePath] = true
	}
	if !paths[ws1.Path] || !paths[ws2.Path] {
		t.Fatalf("expected sessions for both workspaces, got %+v", got.Sessions)
	}
}

func TestListAllSessionsIncludesGatewayMetadata(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "channel-9", "thread-3", sess.ID, ws.ID); err != nil {
		t.Fatalf("UpsertGatewaySession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?all=true", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got listSessionsResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions len = %d want 1", len(got.Sessions))
	}
	if got.Sessions[0].Gateway == nil {
		t.Fatal("expected gateway metadata")
	}
	if got.Sessions[0].Gateway.Platform != "discord" {
		t.Fatalf("gateway.platform = %q want discord", got.Sessions[0].Gateway.Platform)
	}
	if got.Sessions[0].Gateway.ChannelID != "channel-9" {
		t.Fatalf("gateway.channel_id = %q want channel-9", got.Sessions[0].Gateway.ChannelID)
	}
	if got.Sessions[0].Gateway.ThreadID != "thread-3" {
		t.Fatalf("gateway.thread_id = %q want thread-3", got.Sessions[0].Gateway.ThreadID)
	}
}

func newTestEngine(t *testing.T, newRunner RunnerFactory) (*gin.Engine, *session.Service, func()) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dbPath := filepath.Join(t.TempDir(), "cometmind-test.db")
	sqlDB, err := store.OpenSQLite(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	svc := session.New(sqlDB)
	engine, err := New(Deps{
		Config: &config.Config{
			Provider:  "test-provider",
			Model:     "test-model",
			MaxTokens: 256,
			MaxSteps:  8,
			Skills: config.SkillsConfig{
				Enabled:         true,
				IncludeOpenCode: false,
				IncludeClaude:   false,
			},
		},
		Sessions:  svc,
		NewRunner: newRunner,
		Runs:      NewRunManager(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return engine, svc, func() {
		_ = sqlDB.Close()
	}
}

func decodeJSON(t *testing.T, raw []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", string(raw), err)
	}
}

func mustJSON(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

func parseSSEDataFrames(body string) [][]byte {
	var frames [][]byte
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		frames = append(frames, []byte(payload))
	}
	return frames
}
