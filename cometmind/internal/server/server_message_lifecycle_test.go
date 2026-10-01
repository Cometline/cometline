package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestLocalCORSAllowsCometlineRenderer(t *testing.T) {
	t.Parallel()

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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

	engine, _, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/memories/settings", nil)
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
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(`{"text":"hello"}`))
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

	abortReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/sessions/"+sess.ID+"/runs/current", nil)
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

func TestPostMessageEmitsDoneAfterRunnerErrorEvent(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.Errorf("provider disconnected", "llm")
			return fmt.Errorf("provider disconnected")
		}), nil
	})
	defer cleanup()

	workspace, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(context.Background(), workspace.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/"+sess.ID+"/messages",
		bytes.NewBufferString(`{"text":"hello"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	frames := parseSSEDataFrames(rec.Body.String())
	if len(frames) != 2 {
		t.Fatalf("SSE frame count = %d, want 2 body=%s", len(frames), rec.Body.String())
	}
	var gotError, gotDone struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(frames[0], &gotError); err != nil {
		t.Fatalf("decode error event: %v", err)
	}
	if err := json.Unmarshal(frames[1], &gotDone); err != nil {
		t.Fatalf("decode done event: %v", err)
	}
	if gotError.Type != string(event.KindError) || gotDone.Type != string(event.KindDone) {
		t.Fatalf("event types = %q, %q; want error, done", gotError.Type, gotDone.Type)
	}
}

func TestPostMessageKeepsAgentRunningAfterClientDisconnect(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	cancelled := make(chan struct{})
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			close(started)
			select {
			case <-ctx.Done():
				close(cancelled)
				return ctx.Err()
			case <-release:
				close(finished)
				ch <- event.Done()
				return nil
			}
		}), nil
	})
	defer cleanup()

	workspace, err := svc.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(context.Background(), workspace.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	srv := httptest.NewServer(engine)
	defer srv.Close()

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		srv.URL+"/api/v1/sessions/"+sess.ID+"/messages",
		bytes.NewBufferString(`{"text":"hello"}`),
	)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	requestDone := make(chan error, 1)
	go func() {
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		requestDone <- requestErr
	}()

	<-started
	cancelRequest()
	select {
	case <-requestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("client request did not disconnect")
	}

	close(release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("agent did not finish after client disconnect")
	}
	select {
	case <-cancelled:
		t.Fatal("client disconnect cancelled the agent run")
	default:
	}
}
