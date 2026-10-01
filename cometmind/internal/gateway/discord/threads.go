package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/bwmarrin/discordgo"
	"go.uber.org/zap"
)

const threadArchiveMinutes = 60

func isForumLikeChannelType(t discordgo.ChannelType) bool {
	return t == discordgo.ChannelTypeGuildForum || t == discordgo.ChannelTypeGuildMedia
}

func supportsPublicThreadCreation(t discordgo.ChannelType) bool {
	return t == discordgo.ChannelTypeGuildText || t == discordgo.ChannelTypeGuildNews
}

// threadCreationParent resolves the channel that can host a new thread/post.
// When invoked inside an existing thread, the parent text/forum channel is returned.
func threadCreationParent(ch *discordgo.Channel) (parentID string, parentType discordgo.ChannelType, ok bool) {
	if ch == nil {
		return "", 0, false
	}
	if ch.IsThread() && ch.ParentID != "" {
		return ch.ParentID, 0, true
	}
	return ch.ID, ch.Type, true
}

func createCometMindThread(
	s *discordgo.Session,
	sourceChannelID, threadName, welcome string,
) (parentChannelID string, thread *discordgo.Channel, parentType discordgo.ChannelType, err error) {
	ch, err := s.Channel(sourceChannelID)
	if err != nil {
		return "", nil, 0, fmt.Errorf("could not resolve channel: %w", err)
	}
	parentID, parentType, ok := threadCreationParent(ch)
	if !ok {
		return "", nil, 0, fmt.Errorf("could not resolve channel")
	}
	if parentType == 0 {
		parent, err := s.Channel(parentID)
		if err != nil {
			return "", nil, 0, fmt.Errorf("could not resolve thread parent: %w", err)
		}
		if parent == nil {
			return "", nil, 0, fmt.Errorf("could not resolve thread parent")
		}
		parentType = parent.Type
	}

	switch {
	case isForumLikeChannelType(parentType):
		thread, err = s.ForumThreadStart(parentID, threadName, threadArchiveMinutes, welcome)
	case supportsPublicThreadCreation(parentType):
		thread, err = s.ThreadStart(
			parentID,
			threadName,
			discordgo.ChannelTypeGuildPublicThread,
			threadArchiveMinutes,
		)
	default:
		return "", nil, 0, fmt.Errorf(
			"channel type %d does not support thread creation; use a text or forum channel",
			parentType,
		)
	}
	if err != nil {
		return "", nil, 0, err
	}
	return parentID, thread, parentType, nil
}

func (a *Adapter) handleThreadCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if i.GuildID == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Threads can only be created inside a server channel.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	threadName := "cometmind"
	for _, opt := range data.Options {
		if opt.Name == "name" && opt.Type == discordgo.ApplicationCommandOptionString {
			if v := strings.TrimSpace(opt.StringValue()); v != "" {
				threadName = v
			}
		}
	}

	welcome := "New CometMind session started in this thread. Send a message here to talk to the agent."
	parentChannelID, thread, parentType, err := createCometMindThread(s, i.ChannelID, threadName, welcome)
	if err != nil {
		logging.L().Error("discord.thread.create_failed", zap.Error(err))
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to create thread: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if !isForumLikeChannelType(parentType) {
		if _, err := s.ChannelMessageSend(thread.ID, welcome); err != nil {
			logging.L().Warn("discord.thread.welcome_failed", zap.Error(err))
		}
	}

	if a.onThread != nil {
		userID := interactionUserID(i)
		if userID != "" {
			if err := a.onThread(context.Background(), userID, parentChannelID, thread.ID); err != nil {
				logging.L().Error("discord.thread.session_setup_failed", zap.Error(err))
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Created thread <#%s>. Each thread is a separate CometMind session.", thread.ID),
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
