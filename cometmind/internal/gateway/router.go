package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
)

const defaultStopWaitTimeout = 30 * time.Second

// Runner executes agent turns for gateway inbound messages.
type Runner interface {
	RunTurn(ctx context.Context, sess session.Session, workspacePath string, msg InboundMessage, onEvent func(event.Event)) error
}

// Router maps platform identities to CometMind sessions and runs turns.
type Router struct {
	Sessions           session.SessionStore
	Config             *config.Config
	Jobs               *jobs.Service
	Runner             Runner
	Typing             TypingIndicator
	Turns              *TurnRunTracker
	Events             *EventBridge
	Subagents          *subagent.Orchestrator
	StopWaitTimeout    time.Duration
	JobProposals       *JobProposalStore
	DeliverJobProposal func(ctx context.Context, msg OutboundMessage, proposal *PendingJobProposal, workspacePaths []string) error
	seenMessages       *seenPlatformMessages
	onReply            func(context.Context, OutboundMessage) error
}

// SetReplyHandler registers the callback used to deliver outbound messages.
func (r *Router) SetReplyHandler(fn func(context.Context, OutboundMessage) error) {
	r.onReply = fn
}

// HandleInbound routes one external message through the CometMind runtime.
func (r *Router) HandleInbound(ctx context.Context, msg InboundMessage) error {
	sess, runPath, skip, err := r.prepareInbound(ctx, msg)
	if err != nil || skip {
		return err
	}
	return r.runInboundTurn(ctx, msg, sess, runPath)
}

func (r *Router) resolveSession(ctx context.Context, msg InboundMessage, ws session.Workspace) (string, error) {
	mapped, err := r.Sessions.LookupGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID)
	if err == nil {
		return mapped.CometmindSessionID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	modelID, providerID := r.gatewaySessionModel()
	sess, err := r.Sessions.NewSession(ctx, ws.ID, modelID, providerID)
	if err != nil {
		return "", err
	}
	if _, err := r.Sessions.UpsertGatewaySession(ctx, msg.Platform, msg.UserID, msg.ChannelID, msg.ThreadID, sess.ID, ws.ID); err != nil {
		return "", err
	}
	return sess.ID, nil
}

// EnsureThreadSession creates a fresh CometMind session for a newly created platform thread.
func (r *Router) EnsureThreadSession(ctx context.Context, platform, userID, parentChannelID, threadID string) error {
	if r == nil || r.Sessions == nil {
		return fmt.Errorf("gateway router is not configured")
	}
	platform = strings.TrimSpace(platform)
	userID = strings.TrimSpace(userID)
	parentChannelID = strings.TrimSpace(parentChannelID)
	threadID = strings.TrimSpace(threadID)
	if platform == "" || userID == "" || parentChannelID == "" || threadID == "" {
		return fmt.Errorf("platform, user_id, parent_channel_id, and thread_id are required")
	}

	wsPath := r.Config.Gateway.Discord.WorkspacePath
	if wsPath == "" {
		return fmt.Errorf("gateway workspace_path is not configured")
	}
	ws, err := r.Sessions.EnsureWorkspace(ctx, wsPath)
	if err != nil {
		return err
	}

	if _, err := r.Sessions.LookupGatewaySession(ctx, platform, userID, parentChannelID, threadID); err == nil {
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	modelID, providerID := r.gatewaySessionModel()
	sess, err := r.Sessions.NewSession(ctx, ws.ID, modelID, providerID)
	if err != nil {
		return err
	}
	_, err = r.Sessions.UpsertGatewaySession(ctx, platform, userID, parentChannelID, threadID, sess.ID, ws.ID)
	return err
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
	if cfg.RequireMention && !msg.Mentioned && msg.ThreadID == "" {
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

func (r *Router) gatewaySessionModel() (modelID, providerID string) {
	cfg := r.Config.Gateway.Discord
	modelID = strings.TrimSpace(cfg.Model)
	providerID = strings.TrimSpace(cfg.Provider)
	if modelID == "" {
		modelID = r.Config.DefaultModelID
	}
	if providerID == "" {
		providerID = r.Config.DefaultProviderID
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
