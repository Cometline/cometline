package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/gin-gonic/gin"
)

type Runner interface {
	Run(context.Context, session.AgentTurn, chan<- event.Event) error
}

type RunnerFactory func(sess db.Session, workspacePath string) (Runner, error)

type Deps struct {
	Config    *config.Config
	Sessions  *session.Service
	NewRunner RunnerFactory
	Runs      *RunManager
}

type App struct {
	config    *config.Config
	sessions  *session.Service
	newRunner RunnerFactory
	runs      *RunManager
}

func New(deps Deps) (*gin.Engine, error) {
	if deps.Config == nil {
		return nil, fmt.Errorf("server config is required")
	}
	if deps.Sessions == nil {
		return nil, fmt.Errorf("session service is required")
	}
	if deps.NewRunner == nil {
		return nil, fmt.Errorf("runner factory is required")
	}
	if deps.Runs == nil {
		deps.Runs = NewRunManager()
	}

	app := &App{
		config:    deps.Config,
		sessions:  deps.Sessions,
		newRunner: deps.NewRunner,
		runs:      deps.Runs,
	}

	r := gin.New()
	r.Use(localCORS())
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")
	api.GET("/health", app.handleHealth)
	api.POST("/sessions", app.handleCreateSession)
	api.GET("/sessions", app.handleListSessions)
	api.GET("/sessions/:id", app.handleGetSession)
	api.DELETE("/sessions/:id", app.handleDeleteSession)
	api.GET("/sessions/:id/messages", app.handleGetMessages)
	api.POST("/sessions/:id/message", app.handlePostMessage)
	api.POST("/sessions/:id/abort", app.handleAbortSession)

	return r, nil
}

func localCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if isAllowedLocalOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isAllowedLocalOrigin(origin string) bool {
	if origin == "" || origin == "null" || origin == "file://" {
		return true
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://localhost:")
}

type createSessionRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspacePath string `json:"workspace_path"`
	ModelID       string `json:"model_id"`
	ProviderID    string `json:"provider_id"`
}

type postMessageRequest struct {
	Text string `json:"text"`
}

type healthResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type tokenUsageResource struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read"`
	CacheWrite   int `json:"cache_write"`
}

type sessionResource struct {
	ID            string             `json:"id"`
	WorkspaceID   string             `json:"workspace_id"`
	WorkspacePath string             `json:"workspace_path"`
	Title         string             `json:"title"`
	ModelID       string             `json:"model_id"`
	ProviderID    string             `json:"provider_id"`
	Status        string             `json:"status"`
	TokenUsage    tokenUsageResource `json:"token_usage"`
	CreatedAt     int64              `json:"created_at"`
	UpdatedAt     int64              `json:"updated_at"`
}

type listSessionsResponse struct {
	Sessions []sessionResource `json:"sessions"`
}

type transcriptItem struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	ToolInput  any    `json:"tool_input,omitempty"`
	ToolOutput string `json:"tool_output,omitempty"`
	ToolError  bool   `json:"tool_error,omitempty"`
}

type transcriptResponse struct {
	SessionID string           `json:"session_id"`
	Items     []transcriptItem `json:"items"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type sseReasoningStart struct {
	Type string `json:"type"`
}

type sseReasoningDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type sseTextDelta struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

type sseToolCall struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input"`
}

type sseToolResult struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Tool   string `json:"tool"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type sseStepFinish struct {
	Type  string             `json:"type"`
	Usage tokenUsageResource `json:"usage"`
}

type sseError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type sseDone struct {
	Type string `json:"type"`
}

func (a *App) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

func (a *App) handleCreateSession(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ws, ok := a.resolveCreateWorkspace(c, req.WorkspaceID, req.WorkspacePath)
	if !ok {
		return
	}

	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		modelID = a.config.Model
	}
	providerID := strings.TrimSpace(req.ProviderID)
	if providerID == "" {
		providerID = a.config.Provider
	}

	sess, err := a.sessions.NewSession(c.Request.Context(), ws.ID, modelID, providerID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	res, err := sessionResourceFromModel(sess, ws.Path)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.JSON(http.StatusCreated, res)
}

func (a *App) handleListSessions(c *gin.Context) {
	ws, ok := a.resolveReadWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}

	list, err := a.sessions.ListSessions(c.Request.Context(), ws.ID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	items := make([]sessionResource, 0, len(list))
	for _, sess := range list {
		res, err := sessionResourceFromModel(sess, ws.Path)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		items = append(items, res)
	}

	c.JSON(http.StatusOK, listSessionsResponse{Sessions: items})
}

func (a *App) handleGetSession(c *gin.Context) {
	sess, wsPath, ok := a.loadSessionWithWorkspace(c, c.Param("id"))
	if !ok {
		return
	}

	res, err := sessionResourceFromModel(sess, wsPath)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, res)
}

func (a *App) handleDeleteSession(c *gin.Context) {
	sessID := c.Param("id")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessID); !ok {
		return
	}

	a.runs.Cancel(sessID)
	if err := a.sessions.DeleteSession(c.Request.Context(), sessID); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}

func (a *App) handleGetMessages(c *gin.Context) {
	sessID := c.Param("id")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessID); !ok {
		return
	}

	items, err := a.sessions.LoadTranscript(c.Request.Context(), sessID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	out := make([]transcriptItem, 0, len(items))
	for _, item := range items {
		out = append(out, transcriptItemFromModel(item))
	}

	c.JSON(http.StatusOK, transcriptResponse{
		SessionID: sessID,
		Items:     out,
	})
}

func (a *App) handlePostMessage(c *gin.Context) {
	var req postMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "text is required")
		return
	}

	sess, wsPath, ok := a.loadSessionWithWorkspace(c, c.Param("id"))
	if !ok {
		return
	}

	runner, err := a.newRunner(sess, wsPath)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "runner_init_failed", err.Error())
		return
	}

	runCtx, finish, err := a.runs.Start(c.Request.Context(), sess.ID)
	if err != nil {
		writeError(c, http.StatusConflict, "session_running", err.Error())
		return
	}
	defer finish()

	if _, err := a.sessions.AppendUserMessage(c.Request.Context(), sess.ID, req.Text); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	title := req.Text
	if len(title) > 80 {
		title = title[:80] + "…"
	}
	if err := a.sessions.SetTitleIfEmpty(c.Request.Context(), sess.ID, title); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, "streaming_unsupported", "response writer does not support streaming")
		return
	}

	evCh := make(chan event.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(runCtx, session.AgentTurnFromSession(sess), evCh)
		close(evCh)
	}()

	for ev := range evCh {
		payload := payloadFromEvent(ev)
		if payload == nil {
			continue
		}
		if err := writeSSE(c.Writer, payload); err != nil {
			return
		}
		flusher.Flush()
	}

	_ = <-errCh
}

func (a *App) handleAbortSession(c *gin.Context) {
	sessID := c.Param("id")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessID); !ok {
		return
	}
	if !a.runs.Cancel(sessID) {
		writeError(c, http.StatusConflict, "session_not_running", "session is not currently running")
		return
	}
	c.JSON(http.StatusAccepted, statusResponse{Status: "aborting"})
}

func (a *App) resolveCreateWorkspace(c *gin.Context, workspaceID, workspacePath string) (db.Workspace, bool) {
	ctx := c.Request.Context()
	workspaceID = strings.TrimSpace(workspaceID)
	workspacePath = strings.TrimSpace(workspacePath)

	var byID db.Workspace
	var byPath db.Workspace
	var hasID bool
	var hasPath bool

	if workspaceID != "" {
		ws, err := a.sessions.GetWorkspace(ctx, workspaceID)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(c, http.StatusNotFound, "workspace_not_found", "workspace_id was not found")
			return db.Workspace{}, false
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return db.Workspace{}, false
		}
		byID = ws
		hasID = true
	}

	if workspacePath != "" {
		clean, ok := cleanWorkspacePath(c, workspacePath)
		if !ok {
			return db.Workspace{}, false
		}
		ws, err := a.sessions.LookupWorkspaceByPath(ctx, clean)
		if errors.Is(err, sql.ErrNoRows) {
			ws, err = a.sessions.EnsureWorkspace(ctx, clean)
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return db.Workspace{}, false
		}
		byPath = ws
		hasPath = true
	}

	if !hasID && !hasPath {
		writeError(c, http.StatusBadRequest, "workspace_scope_required", "workspace_id or workspace_path is required")
		return db.Workspace{}, false
	}
	if hasID && hasPath && byID.ID != byPath.ID {
		writeError(c, http.StatusBadRequest, "workspace_scope_mismatch", "workspace_id and workspace_path refer to different workspaces")
		return db.Workspace{}, false
	}
	if hasID {
		return byID, true
	}
	return byPath, true
}

func (a *App) resolveReadWorkspace(c *gin.Context, workspaceID, workspacePath string) (db.Workspace, bool) {
	ctx := c.Request.Context()
	workspaceID = strings.TrimSpace(workspaceID)
	workspacePath = strings.TrimSpace(workspacePath)

	if workspaceID == "" && workspacePath == "" {
		writeError(c, http.StatusBadRequest, "workspace_scope_required", "workspace_id or workspace_path is required")
		return db.Workspace{}, false
	}

	var byID db.Workspace
	var byPath db.Workspace
	var hasID bool
	var hasPath bool

	if workspaceID != "" {
		ws, err := a.sessions.GetWorkspace(ctx, workspaceID)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(c, http.StatusNotFound, "workspace_not_found", "workspace_id was not found")
			return db.Workspace{}, false
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return db.Workspace{}, false
		}
		byID = ws
		hasID = true
	}

	if workspacePath != "" {
		clean, ok := cleanWorkspacePath(c, workspacePath)
		if !ok {
			return db.Workspace{}, false
		}
		ws, err := a.sessions.LookupWorkspaceByPath(ctx, clean)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(c, http.StatusNotFound, "workspace_not_found", "workspace_path was not found")
			return db.Workspace{}, false
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return db.Workspace{}, false
		}
		byPath = ws
		hasPath = true
	}

	if hasID && hasPath && byID.ID != byPath.ID {
		writeError(c, http.StatusBadRequest, "workspace_scope_mismatch", "workspace_id and workspace_path refer to different workspaces")
		return db.Workspace{}, false
	}
	if hasID {
		return byID, true
	}
	return byPath, true
}

func (a *App) loadSessionWithWorkspace(c *gin.Context, sessionID string) (db.Session, string, bool) {
	sess, err := a.sessions.GetSession(c.Request.Context(), sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
		return db.Session{}, "", false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return db.Session{}, "", false
	}

	wsPath, err := a.sessions.WorkspacePath(c.Request.Context(), sess.WorkspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(c, http.StatusNotFound, "workspace_not_found", "workspace was not found")
		return db.Session{}, "", false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return db.Session{}, "", false
	}

	return sess, wsPath, true
}

func cleanWorkspacePath(c *gin.Context, workspacePath string) (string, bool) {
	if !filepath.IsAbs(workspacePath) {
		writeError(c, http.StatusBadRequest, "bad_request", "workspace_path must be absolute")
		return "", false
	}
	return filepath.Clean(workspacePath), true
}

func sessionResourceFromModel(sess db.Session, workspacePath string) (sessionResource, error) {
	usage, err := decodeTokenUsage(sess.TokenUsage)
	if err != nil {
		return sessionResource{}, err
	}
	return sessionResource{
		ID:            sess.ID,
		WorkspaceID:   sess.WorkspaceID,
		WorkspacePath: workspacePath,
		Title:         sess.Title,
		ModelID:       sess.ModelID,
		ProviderID:    sess.ProviderID,
		Status:        sess.Status,
		TokenUsage:    usage,
		CreatedAt:     sess.CreatedAt,
		UpdatedAt:     sess.UpdatedAt,
	}, nil
}

func decodeTokenUsage(raw string) (tokenUsageResource, error) {
	if strings.TrimSpace(raw) == "" {
		return tokenUsageResource{}, nil
	}

	var usage cometsdk.TokenUsage
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		return tokenUsageResource{}, fmt.Errorf("decode token usage: %w", err)
	}
	return tokenUsageResource{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CacheRead:    usage.CacheRead,
		CacheWrite:   usage.CacheWrite,
	}, nil
}

func transcriptItemFromModel(item session.TranscriptEntry) transcriptItem {
	switch item.Kind {
	case session.TranscriptKindUser:
		return transcriptItem{Type: "user", Text: item.Text}
	case session.TranscriptKindReasoning:
		return transcriptItem{Type: "reasoning", Text: item.Text}
	case session.TranscriptKindAssistant:
		return transcriptItem{Type: "assistant", Text: item.Text}
	case session.TranscriptKindTool:
		return transcriptItem{
			Type:       "tool",
			ToolName:   item.ToolName,
			ToolInput:  parseOpaqueJSON(item.ToolInput),
			ToolOutput: item.ToolOutput,
			ToolError:  item.ToolIsError,
		}
	default:
		return transcriptItem{Type: string(item.Kind), Text: item.Text}
	}
}

func parseOpaqueJSON(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

func payloadFromEvent(ev event.Event) any {
	switch ev.Kind {
	case event.KindReasoningStart:
		return sseReasoningStart{Type: string(event.KindReasoningStart)}
	case event.KindReasoningDelta:
		if ev.ReasoningDelta == nil {
			return nil
		}
		return sseReasoningDelta{Type: string(event.KindReasoningDelta), Text: ev.ReasoningDelta.Text}
	case event.KindTextDelta:
		if ev.TextDelta == nil {
			return nil
		}
		return sseTextDelta{Type: string(event.KindTextDelta), Delta: ev.TextDelta.Delta}
	case event.KindToolCall:
		if ev.ToolCall == nil {
			return nil
		}
		input := json.RawMessage(ev.ToolCall.Input)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		return sseToolCall{
			Type:  string(event.KindToolCall),
			ID:    ev.ToolCall.ID,
			Tool:  ev.ToolCall.Tool,
			Input: input,
		}
	case event.KindToolResult:
		if ev.ToolOut == nil {
			return nil
		}
		return sseToolResult{
			Type:   string(event.KindToolResult),
			ID:     ev.ToolOut.ID,
			Tool:   ev.ToolOut.Tool,
			Output: ev.ToolOut.Output,
			Error:  ev.ToolOut.Err,
		}
	case event.KindStepFinish:
		if ev.Step == nil {
			return nil
		}
		return sseStepFinish{
			Type: string(event.KindStepFinish),
			Usage: tokenUsageResource{
				InputTokens:  ev.Step.Usage.InputTokens,
				OutputTokens: ev.Step.Usage.OutputTokens,
				CacheRead:    ev.Step.Usage.CacheRead,
				CacheWrite:   ev.Step.Usage.CacheWrite,
			},
		}
	case event.KindError:
		if ev.Err == nil {
			return nil
		}
		return sseError{
			Type:    string(event.KindError),
			Message: ev.Err.Message,
			Code:    ev.Err.Code,
		}
	case event.KindDone:
		return sseDone{Type: string(event.KindDone)}
	default:
		return nil
	}
}

func writeSSE(w http.ResponseWriter, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", raw)
	return err
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}
