package server

import (
	"net/http"

	"github.com/cometline/cometmind/internal/logging"
	"github.com/gin-gonic/gin"
)

// handleEvents streams runtime events that are not tied to a request-scoped
// message stream. It is intentionally global because memory writes can finish
// after the POST /messages stream has already sent done.
func (a *App) handleEvents(c *gin.Context) {
	if a.events == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event stream unavailable"})
		return
	}

	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}
	sub := a.events.Subscribe()
	defer sub.Close()
	flusher.Flush()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev, ok := <-sub.Events:
			if !ok {
				return
			}
			if err := writeSSE(c.Writer, ev); err != nil {
				logging.L().Info("events.sse_client_gone", "error", err)
				return
			}
			flusher.Flush()
		}
	}
}
