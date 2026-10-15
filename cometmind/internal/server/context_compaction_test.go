package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestCompactSessionContextSkipsEmptyTranscript(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(session.Session, string, session.AgentMode) (Runner, error) {
		return fakeRunner(func(context.Context, session.AgentTurn, chan<- event.Event) error {
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/context-compactions", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}
	var got contextCompactionResponse
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "skipped" || got.Reason != "no_prefix" {
		t.Fatalf("body = %+v, want skipped no_prefix", got)
	}

	get := httptest.NewRecorder()
	engine.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID, nil))
	var after sessionResource
	decodeJSON(t, get.Body.Bytes(), &after)
	if after.Running || after.Compacting {
		t.Fatalf("session still busy: running=%v compacting=%v", after.Running, after.Compacting)
	}
}

func TestCompactSessionContextRejectsRunningSession(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	engine, svc, cleanup := newTestEngine(t, func(session.Session, string, session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, _ session.AgentTurn, ch chan<- event.Event) error {
			close(started)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-release:
				ch <- event.Done()
				return nil
			}
		}), nil
	})
	defer cleanup()
	defer close(release)

	ctx := context.Background()
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, ws.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	go func() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(`{"text":"hold the lease"}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the turn to start")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/context-compactions", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}
}
