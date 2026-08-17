package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
	"github.com/gin-gonic/gin"
)

type createSessionRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspacePath string `json:"workspace_path"`
	ModelID       string `json:"model_id"`
	ProviderID    string `json:"provider_id"`
}

type patchSessionRequest struct {
	ModelID    string  `json:"model_id"`
	ProviderID string  `json:"provider_id"`
	Pinned     *bool   `json:"pinned"`
	Title      *string `json:"title"`
	AgentMode  *string `json:"agent_mode"`
}

type forkSessionRequest struct {
	WorkspacePath string `json:"workspace_path"`
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
	if strings.EqualFold(strings.TrimSpace(c.Query("all")), "true") {
		a.listAllSessions(c)
		return
	}

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

func (a *App) listAllSessions(c *gin.Context) {
	ctx := c.Request.Context()
	workspaces, err := a.sessions.ListWorkspaces(ctx)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	pathByID := make(map[string]string, len(workspaces))
	for _, ws := range workspaces {
		pathByID[ws.ID] = ws.Path
	}

	list, err := a.sessions.ListAllSessions(ctx)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	items := make([]sessionResource, 0, len(list))
	for _, sess := range list {
		res, err := sessionResourceFromModel(sess, pathByID[sess.WorkspaceID])
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

	hasModel := strings.TrimSpace(req.ModelID) != "" || strings.TrimSpace(req.ProviderID) != ""
	hasPinned := req.Pinned != nil
	hasTitle := req.Title != nil
	hasAgentMode := req.AgentMode != nil
	if !hasModel && !hasPinned && !hasTitle && !hasAgentMode {
		writeError(c, http.StatusBadRequest, "bad_request", "at least one of model_id/provider_id, pinned, title, or agent_mode is required")
		return
	}
	if hasModel && (strings.TrimSpace(req.ModelID) == "" || strings.TrimSpace(req.ProviderID) == "") {
		writeError(c, http.StatusBadRequest, "bad_request", "model_id and provider_id must both be provided")
		return
	}

	var agentMode session.AgentMode
	if hasAgentMode {
		mode, err := session.ParseAgentMode(*req.AgentMode)
		if err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		agentMode = mode
	}

	sessID := c.Param("id")
	var sess session.Session
	var err error

	if hasModel {
		sess, err = a.sessions.UpdateSessionModel(
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
	}

	if hasPinned {
		sess, err = a.sessions.UpdateSessionPinned(c.Request.Context(), sessID, *req.Pinned)
		if errors.Is(err, session.ErrSessionNotFound) {
			writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
			return
		}
		if err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	if hasTitle {
		sess, err = a.sessions.UpdateSessionTitle(c.Request.Context(), sessID, *req.Title)
		if errors.Is(err, session.ErrSessionNotFound) {
			writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
			return
		}
		if err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
	}

	if hasAgentMode {
		sess, err = a.sessions.UpdateSessionAgentMode(c.Request.Context(), sessID, agentMode)
		if errors.Is(err, session.ErrSessionNotFound) {
			writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
			return
		}
		if err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
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

func (a *App) handleForkSession(c *gin.Context) {
	var req forkSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	clean, ok := cleanWorkspacePath(c, req.WorkspacePath)
	if !ok {
		return
	}
	if !validateWorkspaceDirectory(c, clean) {
		return
	}

	sessID := c.Param("id")
	forked, err := a.sessions.ForkSession(c.Request.Context(), sessID, clean)
	if errors.Is(err, session.ErrSessionNotFound) {
		writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
		return
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	wsPath, err := a.sessions.WorkspacePath(c.Request.Context(), forked.WorkspaceID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	res, err := sessionResourceFromModel(forked, wsPath)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.JSON(http.StatusCreated, res)
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

func (a *App) handleClearSession(c *gin.Context) {
	sessID := c.Param("id")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessID); !ok {
		return
	}
	if a.runs.Running(sessID) {
		writeError(c, http.StatusConflict, "session_running", "session is running")
		return
	}
	if children, err := a.sessions.ListChildSessions(c.Request.Context(), sessID); err == nil && a.acpMgr != nil {
		for _, child := range children {
			if child.DelegationStatus != session.DelegationRunning {
				continue
			}
			_ = a.acpMgr.Cancel(child.ID)
			_ = a.sessions.UpdateDelegationState(c.Request.Context(), child.ID, session.DelegationCancelled, "", "")

		}
	}
	if err := a.sessions.ClearSessionTranscript(c.Request.Context(), sessID); err != nil {
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

func (a *App) handleGetSessionMedia(c *gin.Context) {
	sessID := c.Param("id")
	mediaID := c.Param("mediaId")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessID); !ok {
		return
	}
	file, mediaType, err := media.Open(sessID, mediaID)
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "media not found")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if mediaType != "" {
		c.Header("Content-Type", mediaType)
	}
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), file)
}
