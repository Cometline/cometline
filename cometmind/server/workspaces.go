package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/session"
	workspacefiles "github.com/cometline/cometmind/internal/workspace/files"
	"github.com/gin-gonic/gin"
)

type createWorkspaceRequest struct {
	WorkspacePath string `json:"workspace_path"`
}

type workspaceResource struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	SessionCount int64  `json:"session_count"`
}

type listWorkspacesResponse struct {
	Workspaces []workspaceResource `json:"workspaces"`
}

type pruneWorkspacesResponse struct {
	Pruned int `json:"pruned"`
}

type workspaceFileListResponse struct {
	Files     []string `json:"files"`
	Truncated bool     `json:"truncated"`
}

type writeWorkspaceFileRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	WorkspacePath string `json:"workspace_path"`
	Path          string `json:"path"`
	Content       string `json:"content"`
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

	c.JSON(http.StatusCreated, workspaceResource{ID: ws.ID, Path: ws.Path, SessionCount: 0})
}

func (a *App) handleListWorkspaces(c *gin.Context) {
	list, err := a.sessions.ListWorkspaces(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	items := make([]workspaceResource, 0, len(list))
	for _, ws := range list {
		count, err := a.sessions.CountSessionsForWorkspace(c.Request.Context(), ws.ID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		items = append(items, workspaceResource{ID: ws.ID, Path: ws.Path, SessionCount: count})
	}
	c.JSON(http.StatusOK, listWorkspacesResponse{Workspaces: items})
}

func (a *App) handleDeleteWorkspace(c *gin.Context) {
	clean, ok := cleanWorkspacePath(c, c.Query("workspace_path"))
	if !ok {
		return
	}
	if err := a.sessions.DeleteWorkspaceByPath(c.Request.Context(), clean); err != nil {
		if errors.Is(err, session.ErrWorkspaceHasSessions) {
			writeError(c, http.StatusConflict, "workspace_has_sessions", "workspace still has sessions")
			return
		}
		if strings.TrimSpace(err.Error()) == "workspace path is required" {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) handlePruneWorkspaces(c *gin.Context) {
	pruned, err := a.sessions.PruneMissingWorkspaces(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, pruneWorkspacesResponse{Pruned: pruned})
}

func parseBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func (a *App) handleListWorkspaceFiles(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}

	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", "limit must be an integer")
			return
		}
	}

	result, err := workspacefiles.ListFiles(c.Request.Context(), ws.Path, workspacefiles.ListOptions{
		Query: strings.TrimSpace(c.Query("q")),
		Limit: limit,
		Index: parseBoolQuery(c.Query("index")),
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, workspaceFileListResponse{Files: result.Files, Truncated: result.Truncated})
}

func (a *App) handleListWorkspaceFileChildren(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}

	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", "limit must be an integer")
			return
		}
	}

	result, err := workspacefiles.ListDirectory(c.Request.Context(), ws.Path, c.Query("directory"), workspacefiles.ListOptions{
		Limit: limit,
	})
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusOK, workspaceFileListResponse{Files: result.Files, Truncated: result.Truncated})
}

func (a *App) handleReadWorkspaceFileContent(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}

	result, err := readWorkspaceFilePreview(ws.Path, c.Query("path"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWriteWorkspaceFileContent(c *gin.Context) {
	var req writeWorkspaceFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	ws, ok := a.resolveCreateWorkspace(c, req.WorkspaceID, req.WorkspacePath)
	if !ok {
		return
	}

	if err := writeWorkspaceFileContent(ws.Path, req.Path, req.Content); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	c.Status(http.StatusNoContent)
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

func cleanWorkspacePath(c *gin.Context, workspacePath string) (string, bool) {
	if !filepath.IsAbs(workspacePath) {
		writeError(c, http.StatusBadRequest, "bad_request", "workspace_path must be absolute")
		return "", false
	}
	return filepath.Clean(workspacePath), true
}

func validateWorkspaceDirectory(c *gin.Context, workspacePath string) bool {
	info, err := os.Stat(workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(c, http.StatusBadRequest, "bad_request", "workspace_path does not exist")
			return false
		}
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return false
	}
	if !info.IsDir() {
		writeError(c, http.StatusBadRequest, "bad_request", "workspace_path must be a directory")
		return false
	}
	return true
}
