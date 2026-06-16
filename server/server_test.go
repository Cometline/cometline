package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	if got.Items[0].Type != "user" || got.Items[1].Type != "assistant" || got.Items[2].Type != "reasoning" || got.Items[3].Type != "tool" {
		t.Fatalf("item types = %+v", got.Items)
	}
	inputMap, ok := got.Items[3].ToolInput.(map[string]any)
	if !ok || inputMap["path"] != "main.go" {
		t.Fatalf("tool input = %#v, want path main.go", got.Items[3].ToolInput)
	}
	if got.Items[3].ToolOutput != "package main" {
		t.Fatalf("tool output = %q, want %q", got.Items[3].ToolOutput, "package main")
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
