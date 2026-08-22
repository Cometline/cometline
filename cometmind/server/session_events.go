package server

import (
	"net/http"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/runstate"
	"github.com/gin-gonic/gin"
)

type ingestSessionEventRequest struct {
	RunID    string       `json:"run_id"`
	Sequence uint64       `json:"sequence,omitempty"`
	Start    bool         `json:"start,omitempty"`
	Finish   bool         `json:"finish,omitempty"`
	Event    *event.Event `json:"event,omitempty"`
}

func (a *App) handleSessionEvents(c *gin.Context) {
	sessionID := c.Param("id")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessionID); !ok {
		return
	}
	runID, _, ok := a.runs.Current(c.Request.Context(), sessionID)
	if !ok {
		writeError(c, http.StatusConflict, "session_not_running", "session is not currently running")
		return
	}

	sub, ok := a.sessionEvents.Subscribe(sessionID, runID)
	if !ok {
		writeError(c, http.StatusConflict, "session_run_mismatch", "session run changed before subscription")
		return
	}
	defer sub.Close()
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, "streaming_unsupported", "response writer does not support streaming")
		return
	}
	write := func(ev event.Event) bool {
		if err := writeSSE(c.Writer, ev); err != nil {
			logging.L().Info("session_events.sse_client_gone", "session", sessionID, "error", err)
			return false
		}
		flusher.Flush()
		return true
	}
	for _, ev := range sub.Replay {
		if !write(ev) || ev.Kind == event.KindDone {
			return
		}
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev, ok := <-sub.Events:
			if !ok {
				return
			}
			if !write(ev) || ev.Kind == event.KindDone {
				return
			}
		}
	}
}

func (a *App) handleIngestSessionEvent(c *gin.Context) {
	sessionID := c.Param("id")
	if _, _, ok := a.loadSessionWithWorkspace(c, sessionID); !ok {
		return
	}
	var req ingestSessionEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "run_id and a valid event payload are required")
		return
	}
	actions := 0
	if req.Start {
		actions++
	}
	if req.Finish {
		actions++
	}
	if req.Event != nil {
		actions++
	}
	if req.RunID == "" || actions != 1 || (req.Event != nil && req.Sequence == 0) {
		writeError(c, http.StatusBadRequest, "bad_request", "run_id and a valid event payload are required")
		return
	}
	currentRunID, owner, ok := a.runs.Current(c.Request.Context(), sessionID)
	leaseCurrent := ok && currentRunID == req.RunID && owner == runstate.OwnerGateway
	if !leaseCurrent {
		if req.Finish && a.sessionEvents.Finished(sessionID, req.RunID) {
			c.Status(http.StatusNoContent)
			return
		}
		writeError(c, http.StatusConflict, "session_run_mismatch", "gateway run is no longer current")
		return
	}
	if req.Start {
		started := a.sessionEvents.Start(sessionID, req.RunID)
		if started && a.events != nil {
			a.events.Publish(event.RunStarted(sessionID))
		}
	}
	if req.Event != nil {
		accepted, _ := a.sessionEvents.PublishSequenced(sessionID, req.RunID, req.Sequence, *req.Event)
		if !accepted {
			writeError(c, http.StatusConflict, "session_run_mismatch", "session event stream is not current")
			return
		}
	}
	if req.Finish {
		released, err := a.runs.Release(c.Request.Context(), sessionID, req.RunID)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "internal_error", "failed to release gateway run")
			return
		}
		if !released {
			writeError(c, http.StatusConflict, "session_run_mismatch", "gateway run is no longer current")
			return
		}
	}
	c.Status(http.StatusNoContent)
}
