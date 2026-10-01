package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestPostMessageInlinesWebContext(t *testing.T) {
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
	body := `{"text":"Summarize this page","display_text":"Summarize this page","web_context":{"title":"Example page","url":"https://example.com/article","content":"Page body with useful facts."}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
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
	for _, want := range []string{"Summarize this page", "Example page", "https://example.com/article", "Page body with useful facts.", "untrusted source material"} {
		if !strings.Contains(text, want) {
			t.Fatalf("user text missing %q: %q", want, text)
		}
	}

	transcript, err := svc.LoadTranscript(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetTranscript() error = %v", err)
	}
	if len(transcript) != 1 || transcript[0].Text != "Summarize this page" {
		t.Fatalf("transcript display text = %+v, want hidden web context", transcript)
	}
}

func TestPostMessageInlinesMultipleWebContexts(t *testing.T) {
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

	rec := httptest.NewRecorder()
	body := `{"text":"Compare them","web_contexts":[{"kind":"page","title":"Article","source":"https://example.com/article","content":"Article facts."},{"kind":"file","title":"notes.md","source":"workspace-file:notes.md","content":"Local notes."}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	text := msgs[0].Content[0].(cometsdk.TextBlock).Text
	for _, want := range []string{"Article facts.", "Local notes.", "https://example.com/article", "workspace-file:notes.md"} {
		if !strings.Contains(text, want) {
			t.Fatalf("combined context missing %q: %q", want, text)
		}
	}
}

func TestPostMessagePathOnlyFileWebContext(t *testing.T) {
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

	rec := httptest.NewRecorder()
	body := `{"text":"look here","web_contexts":[{"kind":"file","title":"notes.md","source":"workspace-file:notes.md","content":""},{"kind":"file","title":"notes.md:2-3","source":"workspace-file:notes.md#L2-L3","content":"line two\nline three"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	text := msgs[0].Content[0].(cometsdk.TextBlock).Text
	for _, want := range []string{
		"currently open in workspace panel",
		"workspace-file:notes.md",
		"notes.md:2-3",
		"line two",
		"workspace-file:notes.md#L2-L3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("path/snippet context missing %q: %q", want, text)
		}
	}

	transcript, err := svc.LoadTranscript(ctx, sess.ID)
	if err != nil {
		t.Fatalf("LoadTranscript() error = %v", err)
	}
	if len(transcript) != 1 || transcript[0].Kind != session.TranscriptKindUser {
		t.Fatalf("transcript = %+v, want one user entry", transcript)
	}
	if len(transcript[0].Contexts) != 2 {
		t.Fatalf("contexts = %#v, want 2 UI refs", transcript[0].Contexts)
	}
	if transcript[0].Contexts[0].Role != "viewing" || transcript[0].Contexts[0].Source != "workspace-file:notes.md" {
		t.Fatalf("first context = %#v", transcript[0].Contexts[0])
	}
	if transcript[0].Contexts[1].Source != "workspace-file:notes.md#L2-L3" {
		t.Fatalf("second context = %#v", transcript[0].Contexts[1])
	}
}

func TestPostMessageInlinesExplicitTerminalContext(t *testing.T) {
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

	rec := httptest.NewRecorder()
	body := `{"text":"Explain this","web_contexts":[{"kind":"terminal","title":"Terminal selection","source":"terminal://session-1","content":"go test ./... failed"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	text := msgs[0].Content[0].(cometsdk.TextBlock).Text
	for _, want := range []string{"Terminal selection", "terminal://session-1", "go test ./... failed", "untrusted source material"} {
		if !strings.Contains(text, want) {
			t.Fatalf("terminal context missing %q: %q", want, text)
		}
	}
}

func TestPostMessageInlinesAssistantResponseContext(t *testing.T) {
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

	rec := httptest.NewRecorder()
	body := `{"text":"Use this part","web_contexts":[{"kind":"message","title":"Selected answer","source":"assistant-response://session-1/2","content":"The selected assistant passage."}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	msgs, err := svc.BuildSDKMessages(ctx, sess.ID)
	if err != nil {
		t.Fatalf("BuildSDKMessages() error = %v", err)
	}
	text := msgs[0].Content[0].(cometsdk.TextBlock).Text
	for _, want := range []string{"Prior assistant response", "Selected answer", "assistant-response://session-1/2", "The selected assistant passage.", "ASSISTANT_RESPONSE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("assistant response context missing %q: %q", want, text)
		}
	}

	transcript, err := svc.LoadTranscript(ctx, sess.ID)
	if err != nil {
		t.Fatalf("LoadTranscript() error = %v", err)
	}
	if len(transcript) != 1 || len(transcript[0].Contexts) != 1 {
		t.Fatalf("transcript contexts = %+v", transcript)
	}
	if transcript[0].Contexts[0].Kind != "message" || transcript[0].Contexts[0].Source != "assistant-response://session-1/2" {
		t.Fatalf("assistant response ref = %#v", transcript[0].Contexts[0])
	}
}

func TestFormatWebContextRejectsMalformedAssistantResponseSource(t *testing.T) {
	t.Parallel()
	_, err := formatWebContext(webContextInput{
		Kind:    "message",
		Source:  "assistant-response://session-only",
		Content: "selection",
	})
	if err == nil || !strings.Contains(err.Error(), "identify an assistant response") {
		t.Fatalf("formatWebContext() error = %v", err)
	}
}
