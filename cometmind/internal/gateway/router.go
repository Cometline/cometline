package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

// Runner executes agent turns for gateway inbound messages.
type Runner interface {
	RunTurn(ctx context.Context, sess session.Session, workspacePath, text string, onEvent func(event.Event)) error
}

// Router maps platform identities to CometMind sessions and runs turns.
type Router struct {
	Sessions *session.Service
	Config   *config.Config
	Runner   Runner
	ACPMgr   *acp.SessionManager
	Typing   TypingIndicator
	onReply  func(context.Context, OutboundMessage) error
}

// SetReplyHandler registers the callback used to deliver outbound messages.
func (r *Router) SetReplyHandler(fn func(context.Context, OutboundMessage) error) {
	r.onReply = fn
}

// HandleInbound routes one external message through the CometMind runtime.
func (r *Router) HandleInbound(ctx context.Context, msg InboundMessage) error {
	if r == nil || r.Sessions == nil || r.Runner == nil {
		return fmt.Errorf("gateway router is not configured")
	}
	if !r.allowed(msg) {
		if reason := r.blockReason(msg); reason != "" {
			log.Printf("discord: ignoring message from user=%s channel=%s: %s", msg.UserID, msg.ChannelID, reason)
		}
		return nil
	}

	wsPath := r.Config.Gateway.Discord.WorkspacePath
	if wsPath == "" {
		return fmt.Errorf("gateway workspace_path is not configured")
	}
	ws, err := r.Sessions.EnsureWorkspace(ctx, wsPath)
	if err != nil {
		return err
	}

	sessID, err := r.resolveSession(ctx, msg, ws)
	if err != nil {
		return err
	}
	sess, err := r.Sessions.GetSession(ctx, sessID)
	if err != nil {
		return err
	}

	runPath, err := r.Sessions.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return err
	}

	if child, err := r.Sessions.GetActiveChildForParent(ctx, sess.ID); err == nil {
		switch child.DelegationStatus {
		case "awaiting_user", "awaiting_permission":
			return r.replyToAwaitingChild(ctx, msg, child)
		}
	}

	blocks := contentBlocksFromInbound(msg)
	if _, err := r.Sessions.AppendUserMessageContent(ctx, sess.ID, blocks); err != nil {
		return err
	}
	if err := r.Sessions.SetTitleIfEmpty(ctx, sess.ID, titleFromInbound(msg)); err != nil {
		return err
	}

	if r.Typing != nil {
		stopTyping := r.Typing.KeepTyping(ctx, deliveryChannelID(msg))
		defer stopTyping()
	}

	log.Printf("discord: running agent turn session=%s workspace=%s", sess.ID, runPath)
	var reply strings.Builder
	err = r.Runner.RunTurn(ctx, sess, runPath, msg.Text, func(ev event.Event) {
		switch ev.Kind {
		case event.KindTextDelta:
			reply.WriteString(ev.Delta)
		case event.KindError:
			if ev.Message != "" {
				reply.WriteString("\n[error] ")
				reply.WriteString(ev.Message)
				reply.WriteByte('\n')
			}
		case event.KindSubagentProgress:
			if ev.ProgressText != "" {
				reply.WriteString("\n[subagent] ")
				reply.WriteString(ev.ProgressText)
				reply.WriteByte('\n')
			}
		case event.KindSubagentFinished:
			if ev.Summary != "" {
				reply.WriteString("\n[subagent done] ")
				reply.WriteString(ev.Summary)
				reply.WriteByte('\n')
			}
		case event.KindSubagentAwaitingInput:
			if ev.Question != "" {
				reply.WriteString("\n[subagent question] ")
				reply.WriteString(ev.Question)
				reply.WriteByte('\n')
			}
		}
	})
	var text string
	if err != nil {
		text = fmt.Sprintf("Error: %v", err)
		log.Printf("discord: agent turn failed user=%s channel=%s: %v", msg.UserID, msg.ChannelID, err)
	} else {
		text = strings.TrimSpace(reply.String())
		if text == "" {
			text = "(no response)"
		}
	}
	if r.onReply != nil {
		log.Printf("discord: replying to channel=%s (%d bytes)", msg.ChannelID, len(text))
		return r.onReply(ctx, OutboundMessage{
			Platform:  msg.Platform,
			UserID:    msg.UserID,
			ChannelID: msg.ChannelID,
			ThreadID:  msg.ThreadID,
			Text:      text,
		})
	}
	return nil
}

func (r *Router) replyToAwaitingChild(ctx context.Context, msg InboundMessage, child session.Session) error {
	if r.ACPMgr == nil {
		return fmt.Errorf("ACP manager is not configured")
	}
	if _, err := r.Sessions.AppendUserMessage(ctx, child.ID, msg.Text); err != nil {
		return err
	}
	_ = r.Sessions.UpdateDelegationState(ctx, child.ID, "running", child.OutputSummary, "")
	if err := r.ACPMgr.Respond(child.ID, acp.RespondInput{Text: msg.Text}); err != nil {
		return err
	}
	if r.onReply == nil {
		return nil
	}
	question := child.PendingQuestion
	if question == "" {
		question = "the coder"
	}
	text := fmt.Sprintf("Got it — sent your reply to OpenCode (%s).", question)
	return r.onReply(ctx, OutboundMessage{
		Platform:  msg.Platform,
		UserID:    msg.UserID,
		ChannelID: msg.ChannelID,
		ThreadID:  msg.ThreadID,
		Text:      text,
	})
}

func (r *Router) resolveSession(ctx context.Context, msg InboundMessage, ws session.Workspace) (string, error) {
	mapped, err := r.Sessions.LookupGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID)
	if err == nil {
		return mapped.CometmindSessionID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	modelID, providerID := r.discordSessionModel()
	sess, err := r.Sessions.NewSession(ctx, ws.ID, modelID, providerID)
	if err != nil {
		return "", err
	}
	if _, err := r.Sessions.UpsertGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID, sess.ID, ws.ID); err != nil {
		return "", err
	}
	return sess.ID, nil
}

// EnsureThreadSession creates a fresh CometMind session for a newly created Discord thread.
func (r *Router) EnsureThreadSession(ctx context.Context, userID, parentChannelID, threadID string) error {
	if r == nil || r.Sessions == nil {
		return fmt.Errorf("gateway router is not configured")
	}
	userID = strings.TrimSpace(userID)
	parentChannelID = strings.TrimSpace(parentChannelID)
	threadID = strings.TrimSpace(threadID)
	if userID == "" || parentChannelID == "" || threadID == "" {
		return fmt.Errorf("user_id, parent_channel_id, and thread_id are required")
	}

	wsPath := r.Config.Gateway.Discord.WorkspacePath
	if wsPath == "" {
		return fmt.Errorf("gateway workspace_path is not configured")
	}
	ws, err := r.Sessions.EnsureWorkspace(ctx, wsPath)
	if err != nil {
		return err
	}

	if _, err := r.Sessions.LookupGatewaySession(ctx, "discord", userID, parentChannelID, threadID); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	modelID, providerID := r.discordSessionModel()
	sess, err := r.Sessions.NewSession(ctx, ws.ID, modelID, providerID)
	if err != nil {
		return err
	}
	_, err = r.Sessions.UpsertGatewaySession(ctx, "discord", userID, parentChannelID, threadID, sess.ID, ws.ID)
	return err
}

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

func deliveryChannelID(msg InboundMessage) string {
	if msg.ThreadID != "" {
		return msg.ThreadID
	}
	return msg.ChannelID
}

func contentBlocksFromInbound(msg InboundMessage) []session.ContentBlock {
	blocks := make([]session.ContentBlock, 0, 1+len(msg.Images))
	if msg.Text != "" {
		blocks = append(blocks, session.ContentBlock{Type: "text", Text: msg.Text})
	}
	for _, img := range msg.Images {
		blocks = append(blocks, session.ContentBlock{Type: "image", MediaType: img.MediaType, Data: img.Data})
	}
	return blocks
}

func titleFromInbound(msg InboundMessage) string {
	title := strings.TrimSpace(msg.Text)
	if title == "" && len(msg.Images) > 0 {
		title = "Image message"
	}
	if len(title) > 80 {
		title = title[:80] + "…"
	}
	return title
}

func (r *Router) allowed(msg InboundMessage) bool {
	return r.blockReason(msg) == ""
}

func (r *Router) blockReason(msg InboundMessage) string {
	cfg := r.Config.Gateway.Discord
	if msg.Platform == "discord" && cfg.RequireMention && !msg.Mentioned && msg.ThreadID == "" {
		return "mention required"
	}
	if len(cfg.AllowedUsers) > 0 && !contains(cfg.AllowedUsers, msg.UserID) {
		return "user not in allowed_users"
	}
	// Guild channel allowlist only; DMs use per-user channel IDs that won't match guild channels.
	// Thread channels inherit access from their parent channel ID.
	if len(cfg.AllowedChannels) > 0 && msg.GuildID != "" {
		if !contains(cfg.AllowedChannels, msg.ChannelID) &&
			!contains(cfg.AllowedChannels, msg.ParentChannelID) &&
			!contains(cfg.AllowedChannels, msg.ThreadID) {
			return "channel not in allowed_channels"
		}
	}
	return ""
}

func (r *Router) discordSessionModel() (modelID, providerID string) {
	cfg := r.Config.Gateway.Discord
	modelID = strings.TrimSpace(cfg.Model)
	providerID = strings.TrimSpace(cfg.Provider)
	if modelID == "" {
		modelID = r.Config.Model
	}
	if providerID == "" {
		providerID = r.Config.Provider
	}
	return modelID, providerID
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
