package server

import (
	"net/http"
	"strings"

	"github.com/cometline/cometmind/internal/session"
	workspacegit "github.com/cometline/cometmind/internal/workspace/git"
	"github.com/gin-gonic/gin"
)

type workspaceGitPathsRequest struct {
	WorkspaceID   string   `json:"workspace_id"`
	WorkspacePath string   `json:"workspace_path"`
	Paths         []string `json:"paths"`
}

type workspaceGitCommitRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspacePath string `json:"workspace_path"`
	Message       string `json:"message"`
}

func (a *App) handleWorkspaceGitStatus(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}

	scope, err := workspacegit.ParseScope(c.Query("scope"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := workspacegit.Status(c.Request.Context(), ws.Path, scope)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitDiff(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}

	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "path is required")
		return
	}

	scope, err := workspacegit.ParseScope(c.Query("scope"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := workspacegit.Diff(c.Request.Context(), ws.Path, path, scope)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitStage(c *gin.Context) {
	req, ws, ok := a.bindGitPathsRequest(c)
	if !ok {
		return
	}
	result, err := workspacegit.Stage(c.Request.Context(), ws.Path, req.Paths)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitUnstage(c *gin.Context) {
	req, ws, ok := a.bindGitPathsRequest(c)
	if !ok {
		return
	}
	result, err := workspacegit.Unstage(c.Request.Context(), ws.Path, req.Paths)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitDiscard(c *gin.Context) {
	req, ws, ok := a.bindGitPathsRequest(c)
	if !ok {
		return
	}
	result, err := workspacegit.Discard(c.Request.Context(), ws.Path, req.Paths)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitCommit(c *gin.Context) {
	var req workspaceGitCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, req.WorkspaceID, req.WorkspacePath)
	if !ok {
		return
	}
	result, err := workspacegit.Commit(c.Request.Context(), ws.Path, req.Message)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) bindGitPathsRequest(c *gin.Context) (workspaceGitPathsRequest, session.Workspace, bool) {
	var req workspaceGitPathsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return req, session.Workspace{}, false
	}
	ws, ok := a.resolveCreateWorkspace(c, req.WorkspaceID, req.WorkspacePath)
	if !ok {
		return req, session.Workspace{}, false
	}
	return req, ws, true
}

func writeGitClientOrServerError(c *gin.Context, err error) {
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(msg, "escapes") ||
		strings.Contains(msg, "invalid path") ||
		strings.Contains(msg, "path is empty") ||
		strings.Contains(lower, "required") ||
		strings.Contains(lower, "nothing staged") ||
		strings.Contains(lower, "not a git repository") ||
		strings.Contains(lower, "refusing to discard") ||
		strings.Contains(lower, "too many paths") ||
		strings.Contains(lower, "exceeds") {
		writeError(c, http.StatusBadRequest, "bad_request", msg)
		return
	}
	writeError(c, http.StatusInternalServerError, "internal_error", msg)
}
