package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools/sandbox"
	"github.com/gin-gonic/gin"
)

const (
	maxMessageImages     = 6
	maxMessageImageBytes = 4 * 1024 * 1024
	maxMessageFilePaths  = 8
	maxMessageFileBytes  = 256 * 1024
)

var supportedImageMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

type postMessageRequest struct {
	Text        string              `json:"text"`
	DisplayText string              `json:"display_text,omitempty"`
	Images      []messageImageInput `json:"images,omitempty"`
	FilePaths   []string            `json:"file_paths,omitempty"`
}

type messageImageInput struct {
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

func (a *App) handlePostMessage(c *gin.Context) {
	var req postMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	req.Text = strings.TrimSpace(req.Text)

	sess, wsPath, ok := a.loadSessionWithWorkspace(c, c.Param("id"))
	if !ok {
		return
	}

	blocks, err := contentBlocksFromRequest(req, wsPath)
	if err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if len(blocks) == 0 {
		writeError(c, http.StatusBadRequest, "bad_request", "text or image is required")
		return
	}
	logging.L().Info("message.received", "session", sess.ID, "provider", sess.ProviderID, "model", sess.ModelID, "text_bytes", len(req.Text), "images", len(req.Images), "files", len(req.FilePaths))
	started := time.Now()

	runner, err := a.newRunner(sess, wsPath)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "runner_init_failed", err.Error())
		return
	}

	// Keep the agent lifetime owned by the server rather than the SSE request.
	// Reconnects and transient network drops must not cancel the model/tool loop;
	// the explicit DELETE /runs/current endpoint remains the cancellation path.
	runCtx, finish, err := a.runs.Start(a.runContext, sess.ID)
	if err != nil {
		writeError(c, http.StatusConflict, "session_running", err.Error())
		return
	}
	defer func() {
		finish()
	}()

	if a.jobs != nil {
		if job, ok, _ := a.jobs.JobForSession(c.Request.Context(), sess.ID); ok {
			_ = a.jobs.Heartbeat(c.Request.Context(), job.ID, sess.ID)
		}
	}

	if _, err := a.sessions.AppendUserMessageContent(c.Request.Context(), sess.ID, blocks, strings.TrimSpace(req.DisplayText)); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	// Generate the session title from the first user message (no-op after the
	// first turn). Failures are non-fatal and leave a plain-text fallback.
	a.maybeGenerateTitle(c.Request.Context(), sess, blocks, strings.TrimSpace(req.DisplayText))

	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, http.StatusInternalServerError, "streaming_unsupported", "response writer does not support streaming")
		return
	}

	// Persistence must outlive request cancel: when the client disconnects or
	// aborts, Request.Context is canceled and AppendErrorMessage would no-op,
	// leaving the transcript as a silent empty turn (avatar-only UI).
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 10*time.Second)
	defer persistCancel()

	evCh := make(chan event.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(runCtx, session.AgentTurnFromSession(sess), evCh)
		close(evCh)
	}()

	clientGone := false
	errorPersisted := false
	for ev := range evCh {
		if ev.Kind == event.KindError && strings.TrimSpace(ev.Message) != "" {
			msg := userFacingMessageError(ev.Message)
			ev.Message = msg
			if !errorPersisted {
				if _, err := a.sessions.AppendErrorMessage(persistCtx, sess.ID, msg); err != nil {
					logging.L().Warn("message.error_persist_failed", "session", sess.ID, "error", err)
				} else {
					errorPersisted = true
				}
			}
		}
		if !clientGone {
			if err := writeSSE(c.Writer, ev); err != nil {
				clientGone = true
				logging.L().Info("message.sse_client_gone", "session", sess.ID, "error", err)
				// Keep draining so later error events still reach SQLite.
				continue
			}
			flusher.Flush()
		}
	}

	if err := <-errCh; err != nil {
		if a.jobs != nil {
			_ = a.jobs.ReleaseForSession(persistCtx, sess.ID, err.Error())
		}
		logging.L().Error("message.failed", "session", sess.ID, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		if !errorPersisted {
			msg := userFacingMessageError(err.Error())
			if _, perr := a.sessions.AppendErrorMessage(persistCtx, sess.ID, msg); perr != nil {
				logging.L().Warn("message.error_persist_failed", "session", sess.ID, "error", perr)
			}
			if !clientGone {
				_ = writeSSE(c.Writer, event.Errorf(msg, "llm"))
				flusher.Flush()
			}
		}
		// The runner may already have emitted the error event. Always terminate
		// the SSE contract explicitly so clients can distinguish a failed run
		// from an unexpectedly broken connection.
		if !clientGone {
			_ = writeSSE(c.Writer, event.Done())
			flusher.Flush()
		}
		return
	}
	logging.L().Info("message.completed", "session", sess.ID, "duration_ms", time.Since(started).Milliseconds())
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
	if strings.Contains(lower, "deadline exceeded") {
		return "The model timed out before finishing. Send the message again to continue."
	}
	return raw
}

func contentBlocksFromRequest(req postMessageRequest, workspacePath string) ([]session.ContentBlock, error) {
	if len(req.Images) > maxMessageImages {
		return nil, fmt.Errorf("at most %d images are allowed", maxMessageImages)
	}
	if len(req.FilePaths) > maxMessageFilePaths {
		return nil, fmt.Errorf("at most %d file paths are allowed", maxMessageFilePaths)
	}

	var fileAppend strings.Builder
	seen := make(map[string]bool, len(req.FilePaths))
	for _, rel := range req.FilePaths {
		rel = strings.TrimSpace(rel)
		if rel == "" {
			continue
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true

		abs, err := sandbox.ResolveWorkspacePath(workspacePath, rel)
		if err != nil {
			fileAppend.WriteString(fmt.Sprintf("\n\n<!-- Could not include %s: %s -->", rel, err.Error()))
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			fileAppend.WriteString(fmt.Sprintf("\n\n<!-- Could not include %s: %s -->", rel, err.Error()))
			continue
		}
		if info.IsDir() {
			fileAppend.WriteString(fmt.Sprintf("\n\n<!-- Could not include %s: path is a directory -->", rel))
			continue
		}
		if info.Size() > maxMessageFileBytes {
			fileAppend.WriteString(fmt.Sprintf("\n\n<!-- Could not include %s: file is larger than %d KB -->", rel, maxMessageFileBytes/1024))
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			fileAppend.WriteString(fmt.Sprintf("\n\n<!-- Could not include %s: %s -->", rel, err.Error()))
			continue
		}
		fileAppend.WriteString(fmt.Sprintf("\n\n[File: %s]\n```\n%s\n```", rel, string(data)))
	}

	blocks := make([]session.ContentBlock, 0, 1+len(req.Images))
	text := req.Text
	if fileAppend.Len() > 0 {
		text += fileAppend.String()
	}
	if text != "" {
		blocks = append(blocks, session.ContentBlock{Type: "text", Text: text})
	}
	for i, img := range req.Images {
		mediaType := strings.ToLower(strings.TrimSpace(img.MediaType))
		if !supportedImageMediaTypes[mediaType] {
			return nil, fmt.Errorf("image %d has unsupported media_type", i+1)
		}
		data := strings.TrimSpace(img.Data)
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("image %d data must be valid base64", i+1)
		}
		if len(decoded) == 0 {
			return nil, fmt.Errorf("image %d is empty", i+1)
		}
		if len(decoded) > maxMessageImageBytes {
			return nil, fmt.Errorf("image %d is larger than %d MB", i+1, maxMessageImageBytes/(1024*1024))
		}
		blocks = append(blocks, session.ContentBlock{Type: "image", MediaType: mediaType, Data: data})
	}
	return blocks, nil
}

func (a *App) handleAbortSession(c *gin.Context) {
	sessID := c.Param("id")
	sess, _, ok := a.loadSessionWithWorkspace(c, sessID)
	if !ok {
		return
	}
	if a.acpMgr != nil && sess.ParentSessionID != "" {
		_ = a.acpMgr.Cancel(sessID)
		_ = a.sessions.UpdateDelegationState(c.Request.Context(), sessID, session.DelegationCancelled, "", "")
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
					_ = a.sessions.UpdateDelegationState(c.Request.Context(), child.ID, session.DelegationCancelled, "", "")
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
