package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
	wikifiles "github.com/cometline/cometmind/internal/wiki/files"
	wikilinks "github.com/cometline/cometmind/internal/wiki/links"
	"github.com/gin-gonic/gin"
)

type writeWikiFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (a *App) handleListWikiFiles(c *gin.Context) {
	root, err := paths.WikiDir()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil {
			writeError(c, http.StatusBadRequest, "bad_request", "limit must be an integer")
			return
		}
	}

	result, err := wikifiles.ListMarkdownFiles(c.Request.Context(), root, wikifiles.ListOptions{
		Query: strings.TrimSpace(c.Query("q")),
		Limit: limit,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, workspaceFileListResponse{Files: result.Files, Truncated: result.Truncated})
}

func (a *App) handleReadWikiFileContent(c *gin.Context) {
	root, err := paths.WikiDir()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	result, err := readWorkspaceFilePreview(root, c.Query("path"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) handleListWikiFileBacklinks(c *gin.Context) {
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "path is required")
		return
	}

	root, err := paths.WikiDir()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	index, err := wikilinks.BuildBacklinkIndex(c.Request.Context(), root)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"backlinks": wikilinks.BacklinksFor(index, path),
	})
}

func (a *App) handleWriteWikiFileContent(c *gin.Context) {
	var req writeWikiFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if wikifiles.IsWriteProtected(req.Path) {
		writeError(c, http.StatusBadRequest, "bad_request", "wiki path is read-only")
		return
	}

	root, err := paths.WikiDir()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if err := writeWorkspaceFileContent(root, req.Path, req.Content); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	c.Status(http.StatusNoContent)
}
