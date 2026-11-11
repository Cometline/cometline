package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
	"github.com/Cometline/cometline/cometmind/internal/session"
	workspacegit "github.com/Cometline/cometline/cometmind/internal/workspace/git"
	"github.com/gin-gonic/gin"
)

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

	var result workspacegit.DiffResult
	if sha := strings.TrimSpace(c.Query("sha")); sha != "" {
		result, err = workspacegit.CommitPathDiff(c.Request.Context(), ws.Path, sha, path)
	} else {
		result, err = workspacegit.Diff(c.Request.Context(), ws.Path, path, scope)
	}
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

func (a *App) handleWorkspaceGitGraph(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}
	limit, offset, err := parseGraphPage(c.Query("limit"), c.Query("offset"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	result, err := workspacegit.Graph(c.Request.Context(), ws.Path, workspacegit.GraphQuery{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitCommitDetail(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}
	detail, err := workspacegit.CommitInfo(c.Request.Context(), ws.Path, c.Query("sha"))
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (a *App) handleWorkspaceGitRemotes(c *gin.Context) {
	ws, ok := a.resolveCreateWorkspace(c, c.Query("workspace_id"), c.Query("workspace_path"))
	if !ok {
		return
	}
	remotes, err := workspacegit.ListRemotes(c.Request.Context(), ws.Path)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, apigen.WorkspaceGitRemotes{Remotes: remotes})
}

func (a *App) handleWorkspaceGitPush(c *gin.Context) {
	var req apigen.WorkspaceGitPushRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.PushBranch(
		c.Request.Context(),
		ws.Path,
		derefString(req.Remote),
		derefString(req.Branch),
	)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitPushTags(c *gin.Context) {
	var req apigen.WorkspaceGitPushTagsRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.PushTags(
		c.Request.Context(),
		ws.Path,
		derefString(req.Remote),
		derefString(req.Tag),
	)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitFetch(c *gin.Context) {
	var req apigen.WorkspaceGitFetchRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.Fetch(c.Request.Context(), ws.Path)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitCheckout(c *gin.Context) {
	var req apigen.WorkspaceGitCheckoutRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.Checkout(c.Request.Context(), ws.Path, req.Ref)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitCreateBranch(c *gin.Context) {
	var req apigen.WorkspaceGitBranchCreateRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.CreateBranch(c.Request.Context(), ws.Path, req.Name, derefString(req.StartPoint))
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitRenameBranch(c *gin.Context) {
	var req apigen.WorkspaceGitRefRenameRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.RenameBranch(c.Request.Context(), ws.Path, req.Name, req.NewName)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitDeleteBranch(c *gin.Context) {
	var req apigen.WorkspaceGitRefDeleteRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.DeleteBranch(c.Request.Context(), ws.Path, req.Name)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitCreateTag(c *gin.Context) {
	var req apigen.WorkspaceGitTagCreateRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.CreateTag(
		c.Request.Context(),
		ws.Path,
		req.Name,
		derefString(req.Sha),
		derefString(req.Message),
	)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitRenameTag(c *gin.Context) {
	var req apigen.WorkspaceGitRefRenameRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.RenameTag(c.Request.Context(), ws.Path, req.Name, req.NewName)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitDeleteTag(c *gin.Context) {
	var req apigen.WorkspaceGitRefDeleteRequest
	if !bindGitJSON(c, &req) {
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return
	}
	result, err := workspacegit.DeleteTag(c.Request.Context(), ws.Path, req.Name)
	if err != nil {
		writeGitClientOrServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleWorkspaceGitCommit(c *gin.Context) {
	var req apigen.WorkspaceGitCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
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

func (a *App) bindGitPathsRequest(c *gin.Context) (apigen.WorkspaceGitPathsRequest, session.Workspace, bool) {
	var req apigen.WorkspaceGitPathsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return req, session.Workspace{}, false
	}
	ws, ok := a.resolveCreateWorkspace(c, derefString(req.WorkspaceId), derefString(req.WorkspacePath))
	if !ok {
		return req, session.Workspace{}, false
	}
	return req, ws, true
}

func parseGraphPage(limitRaw, offsetRaw string) (int, int, error) {
	limit := 0
	offset := 0
	if strings.TrimSpace(limitRaw) != "" {
		n, err := strconv.Atoi(limitRaw)
		if err != nil || n < 1 {
			return 0, 0, errInvalidPage("limit must be a positive integer")
		}
		limit = n
	}
	if strings.TrimSpace(offsetRaw) != "" {
		n, err := strconv.Atoi(offsetRaw)
		if err != nil || n < 0 {
			return 0, 0, errInvalidPage("offset must be a non-negative integer")
		}
		offset = n
	}
	return limit, offset, nil
}

type pageError string

func (e pageError) Error() string { return string(e) }

func errInvalidPage(msg string) error { return pageError(msg) }

func bindGitJSON(c *gin.Context, dest any) bool {
	if err := c.ShouldBindJSON(dest); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return false
	}
	return true
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
		strings.Contains(lower, "refusing to delete") ||
		strings.Contains(lower, "invalid branch") ||
		strings.Contains(lower, "invalid tag") ||
		strings.Contains(lower, "invalid ref") ||
		strings.Contains(lower, "invalid start") ||
		strings.Contains(lower, "invalid commit") ||
		strings.Contains(lower, "dirty") ||
		strings.Contains(lower, "not fully merged") ||
		strings.Contains(lower, "already exists") ||
		strings.Contains(lower, "limit must") ||
		strings.Contains(lower, "offset must") ||
		strings.Contains(lower, "too many paths") ||
		strings.Contains(lower, "exceeds") ||
		strings.Contains(lower, "push rejected") ||
		strings.Contains(lower, "detached head") ||
		strings.Contains(lower, "no git remotes") ||
		strings.Contains(lower, "remote") && strings.Contains(lower, "does not exist") {
		writeError(c, http.StatusBadRequest, "bad_request", msg)
		return
	}
	writeError(c, http.StatusInternalServerError, "internal_error", msg)
}
