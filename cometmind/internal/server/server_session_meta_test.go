package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestPatchSessionUpdatesModel(t *testing.T) {
	t.Parallel()

	var gotTurn session.AgentTurn
	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
	req = httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(`{"text":"hello"}`))
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

func TestForkSessionCopiesTranscript(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
	forkReq := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+src.ID+"/forks", bytes.NewBufferString(`{"workspace_path":`+mustJSON(ws2)+`}`))
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

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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

func TestGetSessionIncludesGatewayMetadata(t *testing.T) {
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
	sess, err := svc.NewSession(ctx, ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if _, err := svc.UpsertGatewaySession(ctx, "discord", "user-1", "channel-9", "thread-3", sess.ID, ws.ID); err != nil {
		t.Fatalf("UpsertGatewaySession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got sessionResource
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got.Gateway == nil {
		t.Fatal("expected gateway metadata")
	}
	if got.Gateway.Platform != "discord" {
		t.Fatalf("gateway.platform = %q want discord", got.Gateway.Platform)
	}
	if got.Gateway.ChannelID != "channel-9" {
		t.Fatalf("gateway.channel_id = %q want channel-9", got.Gateway.ChannelID)
	}
	if got.Gateway.ThreadID != "thread-3" {
		t.Fatalf("gateway.thread_id = %q want thread-3", got.Gateway.ThreadID)
	}
}

func TestGetSessionIncludesOrigin(t *testing.T) {
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
	sess, err := svc.NewSession(ctx, ws.ID, "m", "p")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID, nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var got map[string]any
	decodeJSON(t, rec.Body.Bytes(), &got)
	if got["origin"] != "user" {
		t.Fatalf("origin = %v want user", got["origin"])
	}
}
