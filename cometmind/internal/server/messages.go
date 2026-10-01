package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/agent"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	maxMessageImages     = 6
	maxMessageImageBytes = 10 * 1024 * 1024
	maxMessageFilePaths  = 32
	maxMessageFileBytes  = 256 * 1024
	// Workspace/wiki image preview (FilePreview + markdown embeds). Same cap as
	// chat attachments so a file that can be sent can also be opened in the panel.
	maxWorkspaceImagePreviewBytes = maxMessageImageBytes
	maxWebContextChars            = 50000
	maxWebContextTotal            = 100000
	runtimeWikiPrefix             = "@runtime/wiki/"
)

var supportedImageMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

type postMessageRequest struct {
	Text            string               `json:"text"`
	DisplayText     string               `json:"display_text,omitempty"`
	Images          []messageImageInput  `json:"images,omitempty"`
	FilePaths       []string             `json:"file_paths,omitempty"`
	WebContext      *webPageContextInput `json:"web_context,omitempty"`
	WebContexts     []webContextInput    `json:"web_contexts,omitempty"`
	ReasoningEffort string               `json:"reasoning_effort,omitempty"`
	AgentMode       string               `json:"agent_mode,omitempty"`
}

type webPageContextInput struct {
	Title   string `json:"title,omitempty"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type webContextInput struct {
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Source  string `json:"source"`
	Content string `json:"content"`
}

type messageImageInput struct {
	ID        string `json:"id,omitempty"`
	MediaType string `json:"media_type"`
	Data      string `json:"data,omitempty"`
	Alt       string `json:"alt,omitempty"`
	Name      string `json:"name,omitempty"`
	Size      int    `json:"size,omitempty"`
}

type preparedPostMessage struct {
	req      postMessageRequest
	sess     session.Session
	wsPath   string
	mode     session.AgentMode
	blocks   []session.ContentBlock
	contexts []session.MessageContextRef
	started  time.Time
}

func (a *App) handlePostMessage(c *gin.Context) {
	prepared, ok := a.preparePostMessage(c)
	if !ok {
		return
	}
	a.runPostMessage(c, prepared)
}

func (a *App) preparePostMessage(c *gin.Context) (preparedPostMessage, bool) {
	req, sess, wsPath, ok := a.bindPostMessage(c)
	if !ok {
		return preparedPostMessage{}, false
	}
	mode, ok := messageAgentMode(c, req.AgentMode, sess.AgentMode)
	if !ok {
		return preparedPostMessage{}, false
	}
	blocks, contexts, err := contentBlocksFromRequest(req, wsPath)
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return preparedPostMessage{}, false
	}
	if len(blocks) == 0 {
		writeError(c, http.StatusBadRequest, "bad_request", "text or image is required")
		return preparedPostMessage{}, false
	}
	logging.L().Info("message.received", zap.String("session", sess.ID), zap.String("provider", sess.ProviderID), zap.String("model", sess.ModelID), zap.Int("text_bytes", len(req.Text)), zap.Int("images", len(req.Images)), zap.Int("files", len(req.FilePaths)), zap.String("agent_mode", string(mode)))
	return preparedPostMessage{req: req, sess: sess, wsPath: wsPath, mode: mode, blocks: blocks, contexts: contexts, started: time.Now()}, true
}

func (a *App) bindPostMessage(c *gin.Context) (postMessageRequest, session.Session, string, bool) {
	var req postMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return postMessageRequest{}, session.Session{}, "", false
	}
	req.Text = strings.TrimSpace(req.Text)
	sess, wsPath, ok := a.loadSessionWithWorkspace(c, c.Param("id"))
	if !ok {
		return postMessageRequest{}, session.Session{}, "", false
	}
	return req, sess, wsPath, true
}

// messageAgentMode prefers the per-message mode. When it is omitted, the
// session's persisted preference applies. New sessions default to auto.
func messageAgentMode(c *gin.Context, requested, persisted string) (session.AgentMode, bool) {
	raw := requested
	if strings.TrimSpace(raw) == "" {
		raw = persisted
	}
	mode, err := session.ParseAgentMode(raw)
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return "", false
	}
	return mode, true
}

func (a *App) runPostMessage(c *gin.Context, prepared preparedPostMessage) {
	runner, runCtx, runID, finishRun, ok := a.beginMessageRun(c, prepared)
	if !ok {
		return
	}
	defer finishRun()
	a.touchSessionJob(c, prepared.sess.ID)
	if !a.persistPostMessage(c, prepared) {
		return
	}
	a.streamMessageTurn(runCtx, c, prepared, runner, runID, finishRun)
}

func (a *App) beginMessageRun(c *gin.Context, prepared preparedPostMessage) (Runner, context.Context, string, func(), bool) {
	runner, err := a.newRunner(prepared.sess, prepared.wsPath, prepared.mode)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "runner_init_failed", err.Error())
		return nil, nil, "", nil, false
	}
	// The agent lifetime belongs to the server, not the SSE request. A renderer
	// disconnect must not cancel the model/tool loop; DELETE /runs/current does.
	runCtx, finish, err := a.runs.Start(a.runContext, prepared.sess.ID)
	if err != nil {
		writeError(c, http.StatusConflict, "session_running", err.Error())
		return nil, nil, "", nil, false
	}
	runFinished := false
	finishRun := func() {
		if runFinished {
			return
		}
		runFinished = true
		finish()
	}
	runID, _, running := a.runs.Current(c.Request.Context(), prepared.sess.ID)
	if !running {
		finishRun()
		writeError(c, http.StatusInternalServerError, "run_state_failed", "failed to read the active session run")
		return nil, nil, "", nil, false
	}
	a.sessionEvents.Start(prepared.sess.ID, runID)
	if a.events != nil {
		a.events.Publish(event.RunStarted(prepared.sess.ID))
	}
	return runner, runCtx, runID, finishRun, true
}

func (a *App) touchSessionJob(c *gin.Context, sessionID string) {
	if a.jobs == nil {
		return
	}
	job, ok, _ := a.jobs.JobForSession(c.Request.Context(), sessionID)
	if ok {
		_ = a.jobs.Heartbeat(c.Request.Context(), job.ID, sessionID)
	}
}

func (a *App) persistPostMessage(c *gin.Context, prepared preparedPostMessage) bool {
	display := strings.TrimSpace(prepared.req.DisplayText)
	_, err := a.sessions.AppendUserMessageContent(c.Request.Context(), prepared.sess.ID, prepared.blocks, display, prepared.contexts)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return false
	}
	// Title generation is a no-op after the first turn. Failures leave the fallback.
	a.maybeGenerateTitle(c.Request.Context(), prepared.sess, prepared.blocks, display)
	return true
}

func (a *App) streamMessageTurn(runCtx context.Context, c *gin.Context, prepared preparedPostMessage, runner Runner, runID string, finishRun func()) {
	flusher, ok := beginMessageSSE(c)
	if !ok {
		return
	}
	clientGone := false
	errorPersisted := false
	turn := session.AgentTurnFromSession(prepared.sess)
	turn.ReasoningEffort = strings.TrimSpace(prepared.req.ReasoningEffort)
	runErr := agent.RunHostedTurn(runCtx, runner, turn, func(ev event.Event) {
		clientGone, errorPersisted = a.forwardMessageEvent(c, prepared.sess.ID, runID, flusher, ev, clientGone, errorPersisted)
	})
	a.finishMessageTurn(c, prepared, runID, flusher, runErr, clientGone, errorPersisted, finishRun)
}

func beginMessageSSE(c *gin.Context) (http.Flusher, bool) {
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, "streaming_unsupported", "response writer does not support streaming")
		return nil, false
	}
	return flusher, true
}

func (a *App) forwardMessageEvent(c *gin.Context, sessionID, runID string, flusher http.Flusher, ev event.Event, clientGone, errorPersisted bool) (bool, bool) {
	if ev.Kind == event.KindDone {
		return clientGone, errorPersisted
	}
	if ev.Kind == event.KindError && strings.TrimSpace(ev.Message) != "" {
		ev.Message = userFacingMessageError(ev.Message)
		if !errorPersisted {
			errorPersisted = a.persistMessageError(c, sessionID, ev.Message)
		}
	}
	a.sessionEvents.Publish(sessionID, runID, ev)
	if clientGone {
		return true, errorPersisted
	}
	if err := writeSSE(c.Writer, ev); err != nil {
		logging.L().Info("message.sse_client_gone", zap.String("session", sessionID), zap.Error(err))
		return true, errorPersisted
	}
	flusher.Flush()
	return false, errorPersisted
}

func (a *App) persistMessageError(c *gin.Context, sessionID, msg string) bool {
	persistCtx, persistCancel := messagePersistenceContext(c.Request.Context())
	defer persistCancel()
	if _, err := a.sessions.AppendErrorMessage(persistCtx, sessionID, msg); err != nil {
		logging.L().Warn("message.error_persist_failed", zap.String("session", sessionID), zap.Error(err))
		return false
	}
	return true
}

func (a *App) finishMessageTurn(c *gin.Context, prepared preparedPostMessage, runID string, flusher http.Flusher, runErr error, clientGone, errorPersisted bool, finishRun func()) {
	if runErr != nil {
		a.recordMessageFailure(c, prepared, runID, flusher, runErr, clientGone, errorPersisted)
		finishRun()
		publishDone(c, a.sessionEvents, prepared.sess.ID, runID, flusher, clientGone)
		return
	}
	finishRun()
	publishDone(c, a.sessionEvents, prepared.sess.ID, runID, flusher, clientGone)
	logging.L().Info("message.completed", zap.String("session", prepared.sess.ID), zap.Int64("duration_ms", time.Since(prepared.started).Milliseconds()))
}

func (a *App) recordMessageFailure(c *gin.Context, prepared preparedPostMessage, runID string, flusher http.Flusher, runErr error, clientGone, errorPersisted bool) {
	if a.jobs != nil {
		persistCtx, persistCancel := messagePersistenceContext(c.Request.Context())
		_ = a.jobs.ReleaseForSession(persistCtx, prepared.sess.ID, runErr.Error())
		persistCancel()
	}
	logging.L().Error("message.failed", zap.String("session", prepared.sess.ID), zap.Int64("duration_ms", time.Since(prepared.started).Milliseconds()), zap.Error(runErr))
	if errorPersisted {
		return
	}
	msg := userFacingMessageError(runErr.Error())
	_ = a.persistMessageError(c, prepared.sess.ID, msg)
	errEvent := event.Errorf(msg, "llm")
	a.sessionEvents.Publish(prepared.sess.ID, runID, errEvent)
	if !clientGone {
		_ = writeSSE(c.Writer, errEvent)
		flusher.Flush()
	}
}

func publishDone(c *gin.Context, hub *event.SessionHub, sessionID, runID string, flusher http.Flusher, clientGone bool) {
	doneEvent := event.Done()
	hub.Publish(sessionID, runID, doneEvent)
	if !clientGone {
		_ = writeSSE(c.Writer, doneEvent)
		flusher.Flush()
	}
}

// messagePersistenceContext gives each persistence operation its own timeout.
// Creating one before the agent run causes it to expire during long model/tool
// turns, exactly when it is needed to record a late failure. It also detaches
// from request cancellation so a disconnected SSE client cannot suppress the
// transcript error.
func messagePersistenceContext(requestCtx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(requestCtx), 10*time.Second)
}

// userFacingMessageError maps raw runner/provider errors into transcript copy.
func userFacingMessageError(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "The request failed."
	}
	lower := strings.ToLower(raw)
	if raw == context.Canceled.Error() ||
		strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context cancelled") {
		return "Response interrupted. Send the message again to continue."
	}
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return "The model timed out before finishing. Send another message to continue from here."
	}
	return raw
}

func (a *App) handleAbortSession(c *gin.Context) {
	sessID := c.Param("id")
	sess, _, ok := a.loadSessionWithWorkspace(c, sessID)
	if !ok {
		return
	}
	if a.acpMgr != nil && sess.ParentSessionID != "" {
		_ = a.acpMgr.Cancel(sessID)
		_ = a.sessions.UpdateDelegationState(c.Request.Context(), sessID, session.DelegationCancelled, "")
	}
	if a.subagentOrch != nil && sess.ParentSessionID != "" {
		a.subagentOrch.CancelChild(sessID)
	}
	if sess.ParentSessionID == "" {
		if a.subagentOrch != nil {
			a.subagentOrch.CancelForParent(sessID)
		}
		children, err := a.sessions.ListChildSessions(c.Request.Context(), sessID)
		if err == nil && a.acpMgr != nil {
			for _, child := range children {
				switch child.DelegationStatus {
				case session.DelegationRunning, session.DelegationPending:
					_ = a.acpMgr.Cancel(child.ID)
					_ = a.sessions.UpdateDelegationState(c.Request.Context(), child.ID, session.DelegationCancelled, "")
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
