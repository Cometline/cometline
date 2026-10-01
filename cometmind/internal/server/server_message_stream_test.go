package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/contract"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestGetMessagesReturnsTranscriptItems(t *testing.T) {
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

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sess.ID+"/messages", bytes.NewBufferString(`{"text":"hello from api"}`))
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

func TestPostMessageReleasesRunBeforeDoneIsFlushed(t *testing.T) {
	t.Parallel()

	engine, svc, cleanup := newTestEngine(t, func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error) {
		return fakeRunner(func(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
			ch <- event.TextDelta("complete")
			ch <- event.Done()
			return nil
		}), nil
	})
	defer cleanup()

	ctx := context.Background()
	workspace, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("EnsureWorkspace() error = %v", err)
	}
	sess, err := svc.NewSession(ctx, workspace.ID, "test-model", "test-provider")
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	var observedDone bool
	var runningAtDone bool
	var observationErr error
	rec := &flushObserverRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		onFlush: func(body string) {
			if observedDone || !strings.Contains(body, `"type":"done"`) {
				return
			}
			observedDone = true
			sessionRec := httptest.NewRecorder()
			engine.ServeHTTP(
				sessionRec,
				httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sess.ID, nil),
			)
			if sessionRec.Code != http.StatusOK {
				observationErr = fmt.Errorf("session status = %d body=%s", sessionRec.Code, sessionRec.Body.String())
				return
			}
			var resource sessionResource
			if err := json.Unmarshal(sessionRec.Body.Bytes(), &resource); err != nil {
				observationErr = fmt.Errorf("decode session: %w", err)
				return
			}
			runningAtDone = resource.Running
		},
	}
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
	if observationErr != nil {
		t.Fatal(observationErr)
	}
	if !observedDone {
		t.Fatalf("stream body missing done event:\n%s", rec.Body.String())
	}
	if runningAtDone {
		t.Fatal("session was still running when done was flushed")
	}
}

func TestPostMessageInlinesFilePaths(t *testing.T) {
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
	mustWrite(t, filepath.Join(workspacePath, "main.go"), "package main\n")
	if err := os.MkdirAll(filepath.Join(workspacePath, "src", "lib"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
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
	body := `{"text":"review @main.go and @missing.go","file_paths":["main.go","missing.go","main.go","src/lib/"]}`
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
	if !strings.Contains(text, "review @main.go and @missing.go") {
		t.Fatalf("user text missing original prompt: %q", text)
	}
	if !strings.Contains(text, "[Referenced file: main.go —") {
		t.Fatalf("user text missing main.go path stub: %q", text)
	}
	if strings.Contains(text, "package main") {
		t.Fatalf("user text should not inline main.go body: %q", text)
	}
	if !strings.Contains(text, "Could not include missing.go") {
		t.Fatalf("user text missing missing.go error note: %q", text)
	}
	if !strings.Contains(text, "[Referenced directory: src/lib/ —") {
		t.Fatalf("user text missing directory stub: %q", text)
	}
	if strings.Count(text, "[Referenced file: main.go —") != 1 {
		t.Fatalf("main.go should be referenced once, got: %q", text)
	}
}
