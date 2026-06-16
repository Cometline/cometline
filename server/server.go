package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	skillpkg "github.com/cometline/cometmind/internal/skills"
	"github.com/gin-gonic/gin"
)

const (
	maxMessageImages     = 6
	maxMessageImageBytes = 4 * 1024 * 1024
)

var supportedImageMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

type Runner interface {
	Run(context.Context, session.AgentTurn, chan<- event.Event) error
}

type RunnerFactory func(sess session.Session, workspacePath string) (Runner, error)

type Deps struct {
	Config    *config.Config
	Sessions  *session.Service
	Memory    *memory.Service
	NewRunner RunnerFactory
	Runs      *RunManager
	ACPMgr    *acp.SessionManager
}

type App struct {
	config    *config.Config
	sessions  *session.Service
	memory    *memory.Service
	newRunner RunnerFactory
	runs      *RunManager
	acpMgr    *acp.SessionManager
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
		memory:    deps.Memory,
		newRunner: deps.NewRunner,
		runs:      deps.Runs,
		acpMgr:    deps.ACPMgr,
	}

	r := gin.New()
	r.Use(localCORS())
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")
	api.GET("/health", app.handleHealth)
	api.POST("/workspaces", app.handleCreateWorkspace)
	api.POST("/sessions", app.handleCreateSession)
	api.GET("/sessions", app.handleListSessions)
	api.GET("/sessions/:id", app.handleGetSession)
	api.PATCH("/sessions/:id", app.handlePatchSession)
	api.DELETE("/sessions/:id", app.handleDeleteSession)
	api.GET("/sessions/:id/messages", app.handleGetMessages)
	api.GET("/sessions/:id/children", app.handleListChildSessions)
	api.POST("/sessions/:id/message", app.handlePostMessage)
	api.POST("/sessions/:id/respond", app.handleRespondToChildSession)
	api.POST("/sessions/:id/abort", app.handleAbortSession)
	api.GET("/skills", app.handleListSkills)
	api.POST("/skills/sync", app.handleSyncSkills)
	api.GET("/skills/:name", app.handleGetSkill)
	api.GET("/memories", app.handleListMemories)
	api.POST("/memories", app.handleCreateMemory)
	api.PATCH("/memories/:id", app.handlePatchMemory)
	api.DELETE("/memories/:id", app.handleDeleteMemory)
	api.POST("/memories/search", app.handleSearchMemories)
	api.GET("/memory/settings", app.handleGetMemorySettings)
	api.PUT("/memory/settings", app.handlePutMemorySettings)
	api.POST("/memory/compact", app.handleCompactMemory)
	api.GET("/memory/compact/preview", app.handleCompactPreview)

	return r, nil
}

func localCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if isAllowedLocalOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
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
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "app://")
}

type createWorkspaceRequest struct {
	WorkspacePath string `json:"workspace_path"`
}

type workspaceResource struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type createSessionRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspacePath string `json:"workspace_path"`
	ModelID       string `json:"model_id"`
	ProviderID    string `json:"provider_id"`
}

type patchSessionRequest struct {
	ModelID    string `json:"model_id"`
	ProviderID string `json:"provider_id"`
}

type postMessageRequest struct {
	Text   string              `json:"text"`
	Images []messageImageInput `json:"images,omitempty"`
}

type messageImageInput struct {
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
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
	ID               string             `json:"id"`
	WorkspaceID      string             `json:"workspace_id"`
	WorkspacePath    string             `json:"workspace_path"`
	Title            string             `json:"title"`
	ModelID          string             `json:"model_id"`
	ProviderID       string             `json:"provider_id"`
	Status           string             `json:"status"`
	TokenUsage       tokenUsageResource `json:"token_usage"`
	ParentSessionID  string             `json:"parent_session_id,omitempty"`
	Purpose          string             `json:"purpose,omitempty"`
	DelegationStatus string             `json:"delegation_status,omitempty"`
	OutputSummary    string             `json:"output_summary,omitempty"`
	ACPSessionID     string             `json:"acp_session_id,omitempty"`
	PendingQuestion  string             `json:"pending_question,omitempty"`
	CreatedAt        int64              `json:"created_at"`
	UpdatedAt        int64              `json:"updated_at"`
}

type listSessionsResponse struct {
	Sessions []sessionResource `json:"sessions"`
}

type transcriptItem struct {
	Type       string              `json:"type"`
	Text       string              `json:"text,omitempty"`
	Images     []messageImageInput `json:"images,omitempty"`
	ToolName   string              `json:"tool_name,omitempty"`
	ToolInput  any                 `json:"tool_input,omitempty"`
	ToolOutput string              `json:"tool_output,omitempty"`
	ToolError  bool                `json:"tool_error,omitempty"`
}

type transcriptResponse struct {
	SessionID string           `json:"session_id"`
	Items     []transcriptItem `json:"items"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type skillResource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	Internal    bool   `json:"internal"`
}

type listSkillsResponse struct {
	Skills []skillResource `json:"skills"`
	Errors []string        `json:"errors,omitempty"`
}

type skillDetailResponse struct {
	Skill   skillResource `json:"skill"`
	Content string        `json:"content"`
}

type syncSkillsResponse struct {
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

func (a *App) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

func (a *App) handleListSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	items := make([]skillResource, 0, len(reg.Skills))
	for _, skill := range reg.Skills {
		items = append(items, skillResourceFromModel(skill))
	}
	c.JSON(http.StatusOK, listSkillsResponse{Skills: items, Errors: reg.Errors})
}

func (a *App) handleGetSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	skill, content, err := reg.SkillMarkdown(c.Param("name"))
	if err != nil {
		writeError(c, http.StatusNotFound, "skill_not_found", err.Error())
		return
	}
	c.JSON(http.StatusOK, skillDetailResponse{Skill: skillResourceFromModel(skill), Content: content})
}

func (a *App) handleSyncSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	created, skipped, err := reg.SyncMirror(filepath.Join("~", ".cometmind", "skills"))
	if err != nil {
		writeError(c, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, syncSkillsResponse{Created: created, Skipped: skipped, Errors: reg.Errors})
}

func (a *App) skillsForRequest(c *gin.Context) skillpkg.Registry {
	workspacePath := strings.TrimSpace(c.Query("workspace_path"))
	if workspacePath == "" && strings.TrimSpace(c.Query("workspace_id")) != "" {
		if path, err := a.sessions.WorkspacePath(c.Request.Context(), strings.TrimSpace(c.Query("workspace_id"))); err == nil {
			workspacePath = path
		}
	}
	return skillpkg.Discover(workspacePath, a.config.SkillSettings())
}

func skillResourceFromModel(skill skillpkg.Skill) skillResource {
	return skillResource{
		Name:        skill.Name,
		Description: skill.Description,
		Path:        skill.Path,
		Source:      skill.Source,
		Internal:    skill.Internal,
	}
}

func (a *App) handleCreateWorkspace(c *gin.Context) {
	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	clean, ok := cleanWorkspacePath(c, req.WorkspacePath)
	if !ok {
		return
	}

	ws, err := a.sessions.EnsureWorkspace(c.Request.Context(), clean)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.JSON(http.StatusCreated, workspaceResource{ID: ws.ID, Path: ws.Path})
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

func (a *App) handleListChildSessions(c *gin.Context) {
	parentID := c.Param("id")
	_, wsPath, ok := a.loadSessionWithWorkspace(c, parentID)
	if !ok {
		return
	}

	children, err := a.sessions.ListChildSessions(c.Request.Context(), parentID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	items := make([]sessionResource, 0, len(children))
	for _, child := range children {
		res, err := sessionResourceFromModel(child, wsPath)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		items = append(items, res)
	}
	c.JSON(http.StatusOK, listSessionsResponse{Sessions: items})
}

func (a *App) handlePatchSession(c *gin.Context) {
	var req patchSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	sessID := c.Param("id")
	sess, err := a.sessions.UpdateSessionModel(
		c.Request.Context(),
		sessID,
		req.ModelID,
		req.ProviderID,
	)
	if errors.Is(err, session.ErrSessionNotFound) {
		writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	wsPath, err := a.sessions.WorkspacePath(c.Request.Context(), sess.WorkspaceID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
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
	blocks, err := contentBlocksFromRequest(req)
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if len(blocks) == 0 {
		writeError(c, http.StatusBadRequest, "bad_request", "text or image is required")
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

	if _, err := a.sessions.AppendUserMessageContent(c.Request.Context(), sess.ID, blocks); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	title := session.PlainTextFromContent(blocks)
	if title == "" {
		title = "Image"
	}
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
		if err := writeSSE(c.Writer, ev); err != nil {
			return
		}
		flusher.Flush()
	}

	_ = <-errCh
}

func contentBlocksFromRequest(req postMessageRequest) ([]session.ContentBlock, error) {
	if len(req.Images) > maxMessageImages {
		return nil, fmt.Errorf("at most %d images are allowed", maxMessageImages)
	}
	blocks := make([]session.ContentBlock, 0, 1+len(req.Images))
	if req.Text != "" {
		blocks = append(blocks, session.ContentBlock{Type: "text", Text: req.Text})
	}
	for i, img := range req.Images {
		mediaType := strings.ToLower(strings.TrimSpace(img.MediaType))
		if !supportedImageMediaTypes[mediaType] {
			return nil, fmt.Errorf("image %d has unsupported media_type", i+1)
		}
		data := strings.TrimSpace(img.Data)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("image %d data must be valid base64", i+1)
		}
		if len(decoded) == 0 {
			return nil, fmt.Errorf("image %d is empty", i+1)
		}
		if len(decoded) > maxMessageImageBytes {
			return nil, fmt.Errorf("image %d is larger than %d MB", i+1, maxMessageImageBytes/(1024*1024))
		}
		blocks = append(blocks, session.ContentBlock{Type: "image", MediaType: mediaType, Data: data})
	}
	return blocks, nil
}

func (a *App) handleAbortSession(c *gin.Context) {
	sessID := c.Param("id")
	sess, _, ok := a.loadSessionWithWorkspace(c, sessID)
	if !ok {
		return
	}
	if a.acpMgr != nil && sess.ParentSessionID != "" {
		_ = a.acpMgr.Cancel(sessID)
		_ = a.sessions.UpdateDelegationState(c.Request.Context(), sessID, "cancelled", "", "")
	}
	if sess.ParentSessionID == "" {
		children, err := a.sessions.ListChildSessions(c.Request.Context(), sessID)
		if err == nil && a.acpMgr != nil {
			for _, child := range children {
				switch child.DelegationStatus {
				case "running", "awaiting_user", "awaiting_permission":
					_ = a.acpMgr.Cancel(child.ID)
					_ = a.sessions.UpdateDelegationState(c.Request.Context(), child.ID, "cancelled", "", "")
				}
			}
		}
	}
	if !a.runs.Cancel(sessID) {
		if sess.ParentSessionID != "" {
			c.JSON(http.StatusAccepted, statusResponse{Status: "aborting"})
			return
		}
		writeError(c, http.StatusConflict, "session_not_running", "session is not currently running")
		return
	}
	c.JSON(http.StatusAccepted, statusResponse{Status: "aborting"})
}

type respondToChildRequest struct {
	Text               string `json:"text"`
	PermissionOptionID string `json:"permission_option_id"`
}

func (a *App) handleRespondToChildSession(c *gin.Context) {
	childID := c.Param("id")
	var req respondToChildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" && strings.TrimSpace(req.PermissionOptionID) == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "text or permission_option_id is required")
		return
	}

	child, _, ok := a.loadSessionWithWorkspace(c, childID)
	if !ok {
		return
	}
	if child.ParentSessionID == "" {
		writeError(c, http.StatusBadRequest, "not_child_session", "session is not a delegated child")
		return
	}
	switch child.DelegationStatus {
	case "running", "awaiting_user", "awaiting_permission":
	default:
		writeError(c, http.StatusConflict, "not_awaiting_input", "child session is not awaiting input")
		return
	}
	if a.acpMgr == nil {
		writeError(c, http.StatusServiceUnavailable, "acp_unavailable", "ACP manager is not configured")
		return
	}

	if text != "" {
		_, _ = a.sessions.AppendUserMessage(c.Request.Context(), childID, text)
	}
	_ = a.sessions.UpdateDelegationState(c.Request.Context(), childID, "running", child.OutputSummary, "")

	if err := a.acpMgr.Respond(childID, acp.RespondInput{
		Text:               text,
		PermissionOptionID: strings.TrimSpace(req.PermissionOptionID),
	}); err != nil {
		writeError(c, http.StatusConflict, "respond_failed", err.Error())
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
	_ = writeSSE(c.Writer, event.SubagentAwaitingInput(childID, "resumed", text, nil))
	flusher.Flush()
	_ = writeSSE(c.Writer, event.Done())
	flusher.Flush()
}

func (a *App) resolveCreateWorkspace(c *gin.Context, workspaceID, workspacePath string) (session.Workspace, bool) {
	ctx := c.Request.Context()
	workspaceID = strings.TrimSpace(workspaceID)
	workspacePath = strings.TrimSpace(workspacePath)

	var byID session.Workspace
	var byPath session.Workspace
	var hasID bool
	var hasPath bool

	if workspaceID != "" {
		ws, err := a.sessions.GetWorkspace(ctx, workspaceID)
		if errors.Is(err, session.ErrWorkspaceNotFound) {
			writeError(c, http.StatusNotFound, "workspace_not_found", "workspace_id was not found")
			return session.Workspace{}, false
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return session.Workspace{}, false
		}
		byID = ws
		hasID = true
	}

	if workspacePath != "" {
		clean, ok := cleanWorkspacePath(c, workspacePath)
		if !ok {
			return session.Workspace{}, false
		}
		ws, err := a.sessions.LookupWorkspaceByPath(ctx, clean)
		if errors.Is(err, session.ErrWorkspaceNotFound) {
			ws, err = a.sessions.EnsureWorkspace(ctx, clean)
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return session.Workspace{}, false
		}
		byPath = ws
		hasPath = true
	}

	if !hasID && !hasPath {
		writeError(c, http.StatusBadRequest, "workspace_scope_required", "workspace_id or workspace_path is required")
		return session.Workspace{}, false
	}
	if hasID && hasPath && byID.ID != byPath.ID {
		writeError(c, http.StatusBadRequest, "workspace_scope_mismatch", "workspace_id and workspace_path refer to different workspaces")
		return session.Workspace{}, false
	}
	if hasID {
		return byID, true
	}
	return byPath, true
}

func (a *App) resolveReadWorkspace(c *gin.Context, workspaceID, workspacePath string) (session.Workspace, bool) {
	ctx := c.Request.Context()
	workspaceID = strings.TrimSpace(workspaceID)
	workspacePath = strings.TrimSpace(workspacePath)

	if workspaceID == "" && workspacePath == "" {
		writeError(c, http.StatusBadRequest, "workspace_scope_required", "workspace_id or workspace_path is required")
		return session.Workspace{}, false
	}

	var byID session.Workspace
	var byPath session.Workspace
	var hasID bool
	var hasPath bool

	if workspaceID != "" {
		ws, err := a.sessions.GetWorkspace(ctx, workspaceID)
		if errors.Is(err, session.ErrWorkspaceNotFound) {
			writeError(c, http.StatusNotFound, "workspace_not_found", "workspace_id was not found")
			return session.Workspace{}, false
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return session.Workspace{}, false
		}
		byID = ws
		hasID = true
	}

	if workspacePath != "" {
		clean, ok := cleanWorkspacePath(c, workspacePath)
		if !ok {
			return session.Workspace{}, false
		}
		ws, err := a.sessions.LookupWorkspaceByPath(ctx, clean)
		if errors.Is(err, session.ErrWorkspaceNotFound) {
			writeError(c, http.StatusNotFound, "workspace_not_found", "workspace_path was not found")
			return session.Workspace{}, false
		}
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return session.Workspace{}, false
		}
		byPath = ws
		hasPath = true
	}

	if hasID && hasPath && byID.ID != byPath.ID {
		writeError(c, http.StatusBadRequest, "workspace_scope_mismatch", "workspace_id and workspace_path refer to different workspaces")
		return session.Workspace{}, false
	}
	if hasID {
		return byID, true
	}
	return byPath, true
}

func (a *App) loadSessionWithWorkspace(c *gin.Context, sessionID string) (session.Session, string, bool) {
	sess, err := a.sessions.GetSession(c.Request.Context(), sessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
		writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
		return session.Session{}, "", false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return session.Session{}, "", false
	}

	wsPath, err := a.sessions.WorkspacePath(c.Request.Context(), sess.WorkspaceID)
	if errors.Is(err, session.ErrWorkspaceNotFound) {
		writeError(c, http.StatusNotFound, "workspace_not_found", "workspace was not found")
		return session.Session{}, "", false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return session.Session{}, "", false
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

func sessionResourceFromModel(sess session.Session, workspacePath string) (sessionResource, error) {
	usage, err := decodeTokenUsage(sess.TokenUsage)
	if err != nil {
		return sessionResource{}, err
	}
	return sessionResource{
		ID:               sess.ID,
		WorkspaceID:      sess.WorkspaceID,
		WorkspacePath:    workspacePath,
		Title:            sess.Title,
		ModelID:          sess.ModelID,
		ProviderID:       sess.ProviderID,
		Status:           sess.Status,
		TokenUsage:       usage,
		ParentSessionID:  sess.ParentSessionID,
		Purpose:          sess.Purpose,
		DelegationStatus: sess.DelegationStatus,
		OutputSummary:    sess.OutputSummary,
		ACPSessionID:     sess.ACPSessionID,
		PendingQuestion:  sess.PendingQuestion,
		CreatedAt:        sess.CreatedAt,
		UpdatedAt:        sess.UpdatedAt,
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
		out := transcriptItem{Type: "user", Text: item.Text}
		for _, block := range item.Images {
			out.Images = append(out.Images, messageImageInput{MediaType: block.MediaType, Data: block.Data})
		}
		return out
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
