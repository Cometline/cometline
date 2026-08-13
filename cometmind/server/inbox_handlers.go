package server

import (
	"errors"
	"net/http"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/gin-gonic/gin"
)

type inboxMessageResource struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	WorkspaceID     string `json:"workspace_id,omitempty"`
	JobID           string `json:"job_id,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	Status          string `json:"status"`
	ArchiveReason   string `json:"archive_reason,omitempty"`
	UserReply       string `json:"user_reply,omitempty"`
	ProcessedAt     *int64 `json:"processed_at,omitempty"`
	ProcessError    string `json:"process_error,omitempty"`
	ProcessAttempts int64  `json:"process_attempts"`
	ArchivedAt      *int64 `json:"archived_at,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type replyInboxRequest struct {
	Content string `json:"content"`
}

func inboxToResource(m inbox.Message) inboxMessageResource {
	return inboxMessageResource{
		ID:              m.ID,
		Title:           m.Title,
		Body:            m.Body,
		WorkspaceID:     m.WorkspaceID,
		JobID:           m.JobID,
		SessionID:       m.SessionID,
		Status:          m.Status,
		ArchiveReason:   m.ArchiveReason,
		UserReply:       m.UserReply,
		ProcessedAt:     m.ProcessedAt,
		ProcessError:    m.ProcessError,
		ProcessAttempts: m.ProcessAttempts,
		ArchivedAt:      m.ArchivedAt,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func (a *App) handleListInboxMessages(c *gin.Context) {
	if a.inbox == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inbox unavailable"})
		return
	}
	status := c.Query("status")
	items, err := a.inbox.List(c.Request.Context(), inbox.ListFilter{Status: status})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]inboxMessageResource, 0, len(items))
	for _, m := range items {
		out = append(out, inboxToResource(m))
	}
	c.JSON(http.StatusOK, gin.H{"messages": out})
}

func (a *App) handleGetInboxSummary(c *gin.Context) {
	if a.inbox == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inbox unavailable"})
		return
	}
	n, err := a.inbox.CountOpen(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"open_count": n})
}

func (a *App) handleReplyInboxMessage(c *gin.Context) {
	if a.inbox == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inbox unavailable"})
		return
	}
	var req replyInboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid request"}})
		return
	}
	msg, err := a.inbox.Reply(c.Request.Context(), c.Param("id"), req.Content)
	if err != nil {
		switch {
		case errors.Is(err, inbox.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "inbox_not_found", "message": "inbox message not found"}})
		case errors.Is(err, inbox.ErrNotOpen):
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "inbox_not_open", "message": "inbox message is not open"}})
		case errors.Is(err, inbox.ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": err.Error()}})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	openCount, _ := a.inbox.CountOpen(c.Request.Context())
	if a.events != nil {
		a.events.Publish(event.InboxMessageArchived(msg.ID, openCount, msg.ArchiveReason))
	}
	c.JSON(http.StatusOK, inboxToResource(msg))
}

func (a *App) handleDismissInboxMessage(c *gin.Context) {
	if a.inbox == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "inbox unavailable"})
		return
	}
	msg, err := a.inbox.Dismiss(c.Request.Context(), c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, inbox.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "inbox_not_found", "message": "inbox message not found"}})
		case errors.Is(err, inbox.ErrNotOpen):
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "inbox_not_open", "message": "inbox message is not open"}})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	openCount, _ := a.inbox.CountOpen(c.Request.Context())
	if a.events != nil {
		a.events.Publish(event.InboxMessageArchived(msg.ID, openCount, msg.ArchiveReason))
	}
	c.JSON(http.StatusOK, inboxToResource(msg))
}
