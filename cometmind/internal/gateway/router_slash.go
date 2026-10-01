package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChangeWorkspace reassigns the CometMind session mapped to a platform identity.
func (r *Router) ChangeWorkspace(ctx context.Context, msg InboundMessage, workspacePath string) (string, error) {
	if r == nil || r.Sessions == nil {
		return "", fmt.Errorf("gateway router is not configured")
	}
	if !r.allowed(msg) {
		return "", fmt.Errorf("not allowed")
	}

	workspacePath = strings.TrimSpace(workspacePath)
	if workspacePath == "" {
		return "", fmt.Errorf("workspace path is required")
	}
	if !filepath.IsAbs(workspacePath) {
		return "", fmt.Errorf("workspace path must be absolute")
	}
	workspacePath = filepath.Clean(workspacePath)
	info, err := os.Stat(workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("workspace path does not exist")
		}
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace path must be a directory")
	}

	mapped, err := r.Sessions.LookupGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no active session in this channel; send a message first")
	}
	if err != nil {
		return "", err
	}

	sess, err := r.Sessions.ChangeSessionWorkspace(ctx, mapped.CometmindSessionID, workspacePath)
	if err != nil {
		return "", err
	}
	runPath, err := r.Sessions.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Switched workspace to `%s`.", runPath), nil
}

// HandleClearSlash clears the transcript for the session mapped to the current
// platform channel or thread while preserving the session identity and title.
func (r *Router) HandleClearSlash(ctx context.Context, msg InboundMessage) (string, error) {
	if r == nil || r.Sessions == nil {
		return "", fmt.Errorf("gateway router is not configured")
	}
	if !r.allowed(msg) {
		return "", fmt.Errorf("not allowed")
	}

	mapped, err := r.Sessions.LookupGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no active session in this channel; send a message first")
	}
	if err != nil {
		return "", err
	}
	if r.Turns != nil && r.Turns.Running(mapped.CometmindSessionID) {
		return "", fmt.Errorf("session is running")
	}
	if r.Events != nil {
		if err := r.Events.ClearSession(ctx, mapped.CometmindSessionID); err != nil {
			return "", err
		}
	} else {
		if err := r.Sessions.ClearSessionTranscript(ctx, mapped.CometmindSessionID); err != nil {
			return "", err
		}
	}
	return "Cleared this CometMind conversation transcript.", nil
}

// HandleStopSlash cancels the active turn for the session mapped to the
// current platform channel or thread. It does not alter the transcript.
func (r *Router) HandleStopSlash(ctx context.Context, msg InboundMessage) (string, error) {
	if r == nil || r.Sessions == nil {
		return "", fmt.Errorf("gateway router is not configured")
	}
	if !r.allowed(msg) {
		return "", fmt.Errorf("not allowed")
	}
	if r.Turns == nil {
		return "There is no active turn to stop.", nil
	}

	mapped, err := r.Sessions.LookupGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID)
	if errors.Is(err, sql.ErrNoRows) {
		return "There is no active turn to stop.", nil
	}
	if err != nil {
		return "", err
	}
	done, ok := r.Turns.Stop(mapped.CometmindSessionID)
	if !ok {
		return "There is no active turn to stop.", nil
	}
	if r.Subagents != nil {
		r.Subagents.CancelForParent(mapped.CometmindSessionID)
	}
	timeout := r.StopWaitTimeout
	if timeout <= 0 {
		timeout = defaultStopWaitTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-done:
		return "Stopped the active turn.", nil
	case <-waitCtx.Done():
		return "Stop requested, but the turn is still cleaning up.", nil
	}
}

// SuggestWorkspacePaths returns workspace roots matching query for autocomplete UIs.
func (r *Router) SuggestWorkspacePaths(ctx context.Context, query string, limit int) ([]string, error) {
	if r == nil || r.Sessions == nil {
		return nil, fmt.Errorf("gateway router is not configured")
	}
	if limit <= 0 {
		limit = 25
	}

	seen := make(map[string]struct{})
	var out []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}

	query = strings.ToLower(strings.TrimSpace(query))
	for _, path := range recentWorkspacePaths(r.Config.Gateway.Discord.WorkspacePath) {
		if query == "" || strings.Contains(strings.ToLower(path), query) {
			add(path)
		}
		if len(out) >= limit {
			return out, nil
		}
	}

	list, err := r.Sessions.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, ws := range list {
		if query == "" || strings.Contains(strings.ToLower(ws.Path), query) {
			add(ws.Path)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
