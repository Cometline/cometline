package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestCreateSessionAutoRegistersWorkspacePath(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
			path:   "/api/v1/sessions/missing/messages",
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

func TestListSessionsRequiresWorkspaceScope(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
	if err := svc.SetTitleIfEmpty(ctx, sess.ID, "keep this title"); err != nil {
		t.Fatalf("SetTitleIfEmpty() error = %v", err)
	}
	if err := svc.UpdateContextSummary(ctx, sess.ID, "old summary", "ignored"); err != nil {
		t.Fatalf("UpdateContextSummary() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+sess.ID+"/messages", nil)
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
	if got.ContextSummary != "" || got.CompactedUntilMessageID != "" {
		t.Fatalf("session state after clear = %+v", got)
	}
	if got.Title != "keep this title" {
		t.Fatalf("session title after clear = %q, want %q", got.Title, "keep this title")
	}
}

func TestClearSessionRemovesChildSessions(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/"+parent.ID+"/messages", nil)
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
