package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/runstate"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/gin-gonic/gin"
)

type sessionEventTestServer struct {
	engine *gin.Engine
	state  *runstate.Service
	hub    *event.SessionHub
	events *event.Hub
	sess   session.Session
}

func newSessionEventTestServer(t *testing.T) *sessionEventTestServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := store.OpenSQLite(context.Background(), filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	sessions := session.New(database)
	workspace, err := sessions.EnsureWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := sessions.NewSession(context.Background(), workspace.ID, "model", "provider")
	if err != nil {
		t.Fatal(err)
	}
	state := runstate.New(database)
	hub := event.NewSessionHub()
	events := event.NewHub()
	runs := NewRunManager(state)
	engine, err := New(Deps{
		Config:        &config.Config{Provider: "provider", Model: "model"},
		Sessions:      sessions,
		Runs:          runs,
		SessionEvents: hub,
		Events:        events,
		NewRunner: func(session.Session, string, session.AgentMode) (Runner, error) {
			return nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &sessionEventTestServer{engine: engine, state: state, hub: hub, events: events, sess: sess}
}

func (s *sessionEventTestServer) ingest(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sessions/"+s.sess.ID+"/events",
		bytes.NewReader(raw),
	)
	request.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(recorder, request)
	return recorder
}

func TestSessionEventsRejectsSessionWithoutActiveRun(t *testing.T) {
	s := newSessionEventTestServer(t)
	recorder := httptest.NewRecorder()
	s.engine.ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+s.sess.ID+"/events", nil),
	)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGatewayIngestIsIdempotentAndReleasesLeaseAfterDone(t *testing.T) {
	s := newSessionEventTestServer(t)
	lease, err := s.state.Acquire(context.Background(), s.sess.ID, runstate.OwnerGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Finish()
	lifecycle := s.events.Subscribe()
	defer lifecycle.Close()

	if rec := s.ingest(t, map[string]any{"run_id": lease.RunID(), "start": true}); rec.Code != http.StatusNoContent {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	delta := map[string]any{
		"run_id": lease.RunID(), "sequence": 1,
		"event": map[string]any{"type": "text_delta", "delta": "once"},
	}
	if rec := s.ingest(t, delta); rec.Code != http.StatusNoContent {
		t.Fatalf("delta status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := s.ingest(t, delta); rec.Code != http.StatusNoContent {
		t.Fatalf("duplicate status = %d body=%s", rec.Code, rec.Body.String())
	}
	sub, ok := s.hub.Subscribe(s.sess.ID, lease.RunID())
	if !ok {
		t.Fatal("Subscribe() = false")
	}
	defer sub.Close()
	if rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "sequence": 2,
		"event": map[string]any{"type": "done"},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("done status = %d body=%s", rec.Code, rec.Body.String())
	}

	select {
	case got := <-sub.Events:
		if got.Kind != event.KindDone {
			t.Fatalf("live event = %s, want done", got.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("missing live done event")
	}
	if rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "finish": true,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("finish status = %d body=%s", rec.Code, rec.Body.String())
	}
	running, err := s.state.Running(context.Background(), s.sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("gateway lease still running after finish")
	}
	if rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "finish": true,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("duplicate finish status = %d body=%s", rec.Code, rec.Body.String())
	}
	replacement, err := s.state.Acquire(context.Background(), s.sess.ID, runstate.OwnerGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Finish()
	if rec := s.ingest(t, map[string]any{
		"run_id": replacement.RunID(), "start": true,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("replacement start status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "finish": true,
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("stale finish retry status = %d body=%s", rec.Code, rec.Body.String())
	}
	current, err := s.state.Current(context.Background(), s.sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.RunID != replacement.RunID() {
		t.Fatalf("current run = %s, want replacement %s", current.RunID, replacement.RunID())
	}

	for _, want := range []event.Kind{event.KindRunStarted, event.KindRunFinished} {
		select {
		case got := <-lifecycle.Events:
			if got.Kind != want {
				t.Fatalf("lifecycle event = %s, want %s", got.Kind, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing lifecycle event %s", want)
		}
	}
}

func TestGatewayIngestRejectsEventsFromHTTPRunEvenWhenHubMatches(t *testing.T) {
	s := newSessionEventTestServer(t)
	lease, err := s.state.Acquire(context.Background(), s.sess.ID, runstate.OwnerHTTP)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Finish()
	s.hub.Start(s.sess.ID, lease.RunID())

	rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "sequence": 1,
		"event": map[string]any{"type": "done"},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionEventsReplaysAndTailsActiveRun(t *testing.T) {
	s := newSessionEventTestServer(t)
	lease, err := s.state.Acquire(context.Background(), s.sess.ID, runstate.OwnerGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Finish()
	if rec := s.ingest(t, map[string]any{"run_id": lease.RunID(), "start": true}); rec.Code != http.StatusNoContent {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "sequence": 1,
		"event": map[string]any{"type": "text_delta", "delta": "replay"},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("replay status = %d body=%s", rec.Code, rec.Body.String())
	}

	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.engine.ServeHTTP(
			recorder,
			httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+s.sess.ID+"/events", nil),
		)
	}()
	time.Sleep(20 * time.Millisecond)
	if rec := s.ingest(t, map[string]any{
		"run_id": lease.RunID(), "sequence": 2,
		"event": map[string]any{"type": "done"},
	}); rec.Code != http.StatusNoContent {
		t.Fatalf("done status = %d body=%s", rec.Code, rec.Body.String())
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session event stream did not terminate")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"delta":"replay"`) || !strings.Contains(body, `"type":"done"`) {
		t.Fatalf("SSE body = %s", body)
	}
}
