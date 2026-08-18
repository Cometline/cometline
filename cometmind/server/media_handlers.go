package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
	"github.com/gin-gonic/gin"
)

type sessionMediaResource struct {
	ID               string `json:"id"`
	SessionID        string `json:"session_id,omitempty"`
	StorageSessionID string `json:"storage_session_id"`
	WorkspaceID      string `json:"workspace_id,omitempty"`
	Kind             string `json:"kind"`
	MediaType        string `json:"media_type"`
	Alt              string `json:"alt,omitempty"`
	Prompt           string `json:"prompt,omitempty"`
	Model            string `json:"model,omitempty"`
	ProviderID       string `json:"provider_id,omitempty"`
	Source           string `json:"source"`
	SourceMediaID    string `json:"source_media_id,omitempty"`
	Status           string `json:"status"`
	SessionDeleted   bool   `json:"session_deleted"`
	ByteSize         int64  `json:"byte_size"`
	DurationMs       *int64 `json:"duration_ms,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	URL              string `json:"url"`
}

type importMediaRequest struct {
	SessionID string `json:"session_id"`
}

func mediaToResource(item session.MediaRecord) sessionMediaResource {
	return sessionMediaResource{
		ID:               item.ID,
		SessionID:        item.SessionID,
		StorageSessionID: item.StorageSessionID,
		WorkspaceID:      item.WorkspaceID,
		Kind:             item.Kind,
		MediaType:        item.MediaType,
		Alt:              item.Alt,
		Prompt:           item.Prompt,
		Model:            item.Model,
		ProviderID:       item.ProviderID,
		Source:           item.Source,
		SourceMediaID:    item.SourceMediaID,
		Status:           item.Status,
		SessionDeleted:   item.SessionID == "",
		ByteSize:         item.ByteSize,
		DurationMs:       item.DurationMs,
		CreatedAt:        item.CreatedAt,
		URL:              "/api/v1/media/" + url.PathEscape(item.ID) + "/content",
	}
}

func (a *App) handleListMedia(c *gin.Context) {
	items, err := a.sessions.ListMedia(c.Request.Context(), session.MediaListFilter{
		WorkspaceID: c.Query("workspace_id"),
		SessionID:   c.Query("session_id"),
		Kind:        c.Query("kind"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "kind must be") {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]sessionMediaResource, 0, len(items))
	for _, item := range items {
		out = append(out, mediaToResource(item))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (a *App) handleImportMedia(c *gin.Context) {
	var req importMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	item, err := a.sessions.ImportMedia(c.Request.Context(), req.SessionID, c.Param("id"))
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mediaToResource(item))
}

func (a *App) handleDeleteMedia(c *gin.Context) {
	item, err := a.sessions.DeleteMedia(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeMediaError(c, err)
		return
	}
	c.JSON(http.StatusOK, mediaToResource(item))
}

func (a *App) handleGetMediaContent(c *gin.Context) {
	item, err := a.sessions.GetMedia(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeMediaError(c, err)
		return
	}
	if item.Status != "ready" {
		writeError(c, http.StatusNotFound, "not_found", "media not found")
		return
	}
	writeMediaContent(c, item.StorageSessionID, item.ID)
}

func writeMediaContent(c *gin.Context, storageSessionID, mediaID string) {
	file, mediaType, err := media.Open(storageSessionID, mediaID)
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

func writeMediaError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, session.ErrSessionNotFound), strings.Contains(err.Error(), "media not found"):
		writeError(c, http.StatusNotFound, "not_found", err.Error())
	case strings.Contains(err.Error(), "not available"):
		writeError(c, http.StatusConflict, "conflict", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
