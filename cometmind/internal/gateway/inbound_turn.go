package gateway

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/media"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func (r *Router) prepareInbound(ctx context.Context, msg InboundMessage) (session.Session, string, bool, error) {
	if r == nil || r.Sessions == nil || r.Runner == nil {
		return session.Session{}, "", false, fmt.Errorf("gateway router is not configured")
	}
	if !r.allowed(msg) {
		if reason := r.blockReason(msg); reason != "" {
			logging.L().Info("gateway.message.ignored",
				"platform", msg.Platform,
				"user", msg.UserID,
				"channel", msg.ChannelID,
				"reason", reason,
			)
		}
		return session.Session{}, "", true, nil
	}
	if ignored := r.ignoreDuplicate(msg); ignored {
		return session.Session{}, "", true, nil
	}

	wsPath := r.Config.Gateway.Discord.WorkspacePath
	if wsPath == "" {
		return session.Session{}, "", false, fmt.Errorf("gateway workspace_path is not configured")
	}
	ws, err := r.Sessions.EnsureWorkspace(ctx, wsPath)
	if err != nil {
		return session.Session{}, "", false, err
	}
	sessID, err := r.resolveSession(ctx, msg, ws)
	if err != nil {
		return session.Session{}, "", false, err
	}
	sess, err := r.Sessions.GetSession(ctx, sessID)
	if err != nil {
		return session.Session{}, "", false, err
	}
	runPath, err := r.Sessions.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return session.Session{}, "", false, err
	}
	return sess, runPath, false, nil
}

func (r *Router) ignoreDuplicate(msg InboundMessage) bool {
	if msg.PlatformMessageID == "" {
		return false
	}
	key := msg.Platform + ":" + msg.PlatformMessageID
	seen := r.seenMessages
	if seen == nil {
		seen = defaultSeenPlatformMessages
	}
	if !seen.seenOrAdd(key) {
		return false
	}
	logging.L().Info("gateway.message.ignored",
		"platform", msg.Platform,
		"user", msg.UserID,
		"channel", msg.ChannelID,
		"message_id", msg.PlatformMessageID,
		"reason", "duplicate_platform_message",
	)
	return true
}

type inboundTurn struct {
	ctx             context.Context
	release         func()
	forwarder       *EventForwarder
	reply           strings.Builder
	images          []OutboundImage
	proposal        *JobProposalPayload
	sourceChannelID string
}

func (turn *inboundTurn) cleanup() {
	if turn != nil && turn.release != nil {
		turn.release()
	}
}

func (r *Router) runInboundTurn(ctx context.Context, msg InboundMessage, sess session.Session, runPath string) error {
	turn, err := r.beginInboundTurn(ctx, msg, sess, runPath)
	if err != nil {
		return err
	}
	defer turn.cleanup()

	err = r.Runner.RunTurn(turn.ctx, sess, runPath, msg, func(ev event.Event) {
		r.collectInboundEvent(turn, sess.ID, ev)
	})
	if errors.Is(err, context.Canceled) || errors.Is(turn.ctx.Err(), context.Canceled) {
		return r.cancelInboundTurn(sess.ID)
	}
	text := inboundReplyText(err, msg, turn.reply.String(), len(turn.images))
	if err := r.sendInboundReply(ctx, msg, text, turn.images); err != nil {
		return err
	}
	r.deliverInboundProposal(ctx, msg, sess.ID, runPath, turn.sourceChannelID, turn.proposal)
	return nil
}

func (r *Router) beginInboundTurn(ctx context.Context, msg InboundMessage, sess session.Session, runPath string) (*inboundTurn, error) {
	turn := &inboundTurn{ctx: ctx, sourceChannelID: deliveryChannelID(msg)}
	var releases []func()
	turn.release = func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	fail := func(err error) (*inboundTurn, error) {
		turn.cleanup()
		return nil, err
	}

	var finish func()
	if r.Turns != nil {
		var err error
		turn.ctx, finish, err = r.Turns.Start(ctx, sess.ID)
		if err != nil {
			return nil, err
		}
		if finish != nil {
			releases = append(releases, finish)
		}
	}
	blocks := contentBlocksFromInbound(msg)
	if _, err := r.Sessions.AppendUserMessageContent(ctx, sess.ID, blocks, "", nil); err != nil {
		return fail(err)
	}
	if err := r.Sessions.SetTitleIfEmpty(ctx, sess.ID, titleFromInbound(msg)); err != nil {
		return fail(err)
	}
	if r.Events != nil && r.Turns != nil {
		turn.forwarder = r.Events.Start(ctx, sess.ID, r.Turns.RunID(sess.ID), finish)
		if finish != nil && len(releases) > 0 {
			releases = releases[:len(releases)-1]
		}
		releases = append(releases, turn.forwarder.Close)
	}
	stopHeartbeat := jobs.StartHeartbeatDuringTurn(turn.ctx, r.Jobs, sess.ID)
	releases = append(releases, stopHeartbeat)
	if r.Typing != nil {
		releases = append(releases, r.Typing.KeepTyping(turn.ctx, deliveryChannelID(msg)))
	}
	logging.L().Info("gateway.agent_turn.start", "platform", msg.Platform, "session", sess.ID, "workspace", runPath)
	return turn, nil
}

func (r *Router) collectInboundEvent(turn *inboundTurn, sessionID string, ev event.Event) {
	turn.forwarder.Forward(ev)
	switch ev.Kind {
	case event.KindTextDelta:
		turn.reply.WriteString(ev.Delta)
	case event.KindTurnRecover:
		trimmed := truncateUTF16Suffix(turn.reply.String(), ev.TextChars)
		turn.reply.Reset()
		turn.reply.WriteString(trimmed)
	case event.KindError:
		if ev.Message != "" {
			turn.reply.WriteString("\n[error] ")
			turn.reply.WriteString(ev.Message)
			turn.reply.WriteByte('\n')
		}
	case event.KindAssistantImage:
		path, pathErr := media.AbsolutePath(sessionID, ev.ImageID)
		if pathErr != nil {
			logging.L().Warn("gateway.assistant_image.path", "session", sessionID, "image", ev.ImageID, "error", pathErr)
			break
		}
		turn.images = append(turn.images, OutboundImage{
			Path:      path,
			Filename:  filepath.Base(path),
			MediaType: ev.MediaType,
			Alt:       ev.Alt,
		})
	case event.KindAssistantVideo:
		note := "Generated a video. Open this session or Gallery in Cometline to watch it."
		if strings.TrimSpace(ev.Alt) != "" {
			note = "Generated a video (" + strings.TrimSpace(ev.Alt) + "). Open this session or Gallery in Cometline to watch it."
		}
		if turn.reply.Len() > 0 {
			turn.reply.WriteByte('\n')
		}
		turn.reply.WriteString(note)
	case event.KindToolResult:
		if ev.Tool == "propose_job" && ev.ToolErr == "" {
			if payload, ok := ParseJobProposalOutput(ev.Output); ok {
				turn.proposal = payload
			}
		}
	}
}

func (r *Router) cancelInboundTurn(sessionID string) error {
	if r.Subagents != nil {
		r.Subagents.CancelForParent(sessionID)
		_, _ = r.Subagents.Wait(context.Background(), sessionID, nil)
	}
	// /stop intentionally cancels the runtime context. The slash command
	// waits for this cleanup, so do not emit partial output or a noisy
	// provider cancellation error into the conversation channel.
	return nil
}

func inboundReplyText(err error, msg InboundMessage, reply string, imageCount int) string {
	if err != nil {
		logging.L().Error("gateway.agent_turn.failed",
			"platform", msg.Platform,
			"user", msg.UserID,
			"channel", msg.ChannelID,
			"error", err,
		)
		return fmt.Sprintf("Error: %v", err)
	}
	text := strings.TrimSpace(reply)
	if text == "" && imageCount == 0 {
		return "(no response)"
	}
	return text
}

func (r *Router) sendInboundReply(ctx context.Context, msg InboundMessage, text string, images []OutboundImage) error {
	if r.onReply == nil {
		return nil
	}
	logging.L().Info("gateway.reply", "platform", msg.Platform, "channel", msg.ChannelID, "bytes", len(text), "images", len(images))
	return r.onReply(ctx, OutboundMessage{
		Platform:  msg.Platform,
		UserID:    msg.UserID,
		ChannelID: msg.ChannelID,
		ThreadID:  msg.ThreadID,
		Text:      text,
		Images:    images,
	})
}

func (r *Router) deliverInboundProposal(ctx context.Context, msg InboundMessage, sessionID, runPath, sourceChannelID string, proposal *JobProposalPayload) {
	if proposal == nil || r.JobProposals == nil || r.DeliverJobProposal == nil {
		return
	}
	paths, pathErr := r.SuggestWorkspacePaths(ctx, "", 25)
	if pathErr != nil {
		logging.L().Warn("gateway.job_proposal.workspace_paths", "platform", msg.Platform, "error", pathErr)
		paths = nil
	}
	if runPath != "" {
		hasDefault := false
		for _, p := range paths {
			if p == runPath {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			paths = append([]string{runPath}, paths...)
		}
	}
	pending := r.JobProposals.Put(msg, *proposal, sessionID, sourceChannelID, runPath)
	out := OutboundMessage{
		Platform:  msg.Platform,
		UserID:    msg.UserID,
		ChannelID: msg.ChannelID,
		ThreadID:  msg.ThreadID,
	}
	if err := r.DeliverJobProposal(ctx, out, pending, paths); err != nil {
		logging.L().Error("gateway.job_proposal.deliver_failed", "platform", msg.Platform, "error", err)
	}
}
