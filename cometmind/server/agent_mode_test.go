package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

// capturingRunnerFactory records the agent mode each runner was built with.
func capturingRunnerFactory(modes *[]session.AgentMode) RunnerFactory {
	var mu sync.Mutex
	return func(_ session.Session, _ string, mode session.AgentMode) (Runner, error) {
		mu.Lock()
		*modes = append(*modes, mode)
		mu.Unlock()
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	}
}

func TestCreateSessionDefaultsToAuto(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, capturingRunnerFactory(new([]session.AgentMode)))
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(t.TempDir())+`}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got sessionResource
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.AgentMode != string(session.AgentModeAuto) {
		t.Fatalf("agent_mode = %q, want %q", got.AgentMode, session.AgentModeAuto)
	}
}

func TestPatchSessionAgentModeRoundTrip(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, capturingRunnerFactory(new([]session.AgentMode)))
	defer cleanup()

	var created sessionResource
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(t.TempDir())+`}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	decodeJSON(t, rec.Body.Bytes(), &created)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+created.ID, bytes.NewBufferString(`{"agent_mode":"plan"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var patched sessionResource
	decodeJSON(t, rec.Body.Bytes(), &patched)
	if patched.AgentMode != string(session.AgentModePlan) {
		t.Fatalf("patched agent_mode = %q, want %q", patched.AgentMode, session.AgentModePlan)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+created.ID, nil)
	engine.ServeHTTP(rec, req)
	var fetched sessionResource
	decodeJSON(t, rec.Body.Bytes(), &fetched)
	if fetched.AgentMode != string(session.AgentModePlan) {
		t.Fatalf("fetched agent_mode = %q, want %q", fetched.AgentMode, session.AgentModePlan)
	}
}

func TestPatchSessionRejectsInvalidAgentMode(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, capturingRunnerFactory(new([]session.AgentMode)))
	defer cleanup()

	var created sessionResource
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewBufferString(`{"workspace_path":`+mustJSON(t.TempDir())+`}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	decodeJSON(t, rec.Body.Bytes(), &created)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/sessions/"+created.ID, bytes.NewBufferString(`{"agent_mode":"sandbox"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("patch status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPostMessageForwardsAgentModeToRunner(t *testing.T) {
	t.Parallel()

	var modes []session.AgentMode
	engine, svc, cleanup := newTestEngine(t, capturingRunnerFactory(&modes))
	defer cleanup()

	wsPath := t.TempDir()
	ws, err := svc.EnsureWorkspace(context.Background(), wsPath)
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)
		return rec
	}

	// Explicit per-turn plan mode reaches the runner.
	rec := post(`{"text":"plan turn","agent_mode":"plan"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(modes) != 1 || modes[0] != session.AgentModePlan {
		t.Fatalf("runner modes = %v, want [plan]", modes)
	}

	// Persisted session mode applies when the request omits agent_mode.
	if _, err := svc.UpdateSessionAgentMode(context.Background(), sess.ID, session.AgentModePlan); err != nil {
		t.Fatalf("UpdateSessionAgentMode() error = %v", err)
	}
	rec = post(`{"text":"second turn"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(modes) != 2 || modes[1] != session.AgentModePlan {
		t.Fatalf("runner modes = %v, want [plan plan]", modes)
	}
}

func TestPostMessageRejectsInvalidAgentMode(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, capturingRunnerFactory(new([]session.AgentMode)))
	defer cleanup()

	ws, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(context.Background(), ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(`{"text":"x","agent_mode":"sandbox"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid agent mode") {
		t.Fatalf("body = %s, want invalid agent mode error", rec.Body.String())
	}
}
