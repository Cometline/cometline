package discord

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
	"github.com/cometline/cometmind/internal/logging"
)

func (a *Adapter) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}
	if a.onInbound == nil {
		return
	}
	mentioned := false
	if m.GuildID == "" {
		mentioned = true
	} else if s.State != nil && s.State.User != nil {
		for _, u := range m.Mentions {
			if u.ID == s.State.User.ID {
				mentioned = true
				break
			}
		}
	}

	parentChannelID := ""
	if m.GuildID != "" {
		if ch, err := s.Channel(m.ChannelID); err == nil && ch != nil && ch.ParentID != "" {
			parentChannelID = ch.ParentID
		}
	}
	routingChannelID, threadID := discordRoutingIDs(m.ChannelID, parentChannelID)
	if a.Config.RequireMention && !mentioned && threadID == "" {
		logging.L().Info("discord.message.ignored", "channel", m.ChannelID, "reason", "mention_required")
		return
	}

	text := strings.TrimSpace(stripBotMentions(m.Content, s.State))
	images, err := imageAttachments(context.Background(), m.Attachments)
	if err != nil {
		logging.L().Info("discord.message.ignored", "channel", m.ChannelID, "reason", "attachments_unsupported", "error", err)
		return
	}
	if text == "" && len(images) == 0 {
		if strings.TrimSpace(m.Content) != "" {
			logging.L().Info("discord.message.ignored", "channel", m.ChannelID, "reason", "only_mentions")
		} else if m.GuildID != "" {
			logging.L().Info("discord.message.ignored",
				"channel", m.ChannelID,
				"reason", "empty_content_enable_message_content_intent",
			)
		}
		return
	}
	logging.L().Info("discord.message.inbound",
		"user", m.Author.ID,
		"channel", routingChannelID,
		"thread", threadID,
		"parent", parentChannelID,
		"guild", m.GuildID,
		"text", truncateLog(text, 80),
		"images", len(images),
	)
	a.onInbound(context.Background(), gateway.InboundMessage{
		Platform:          PlatformName,
		PlatformMessageID: m.ID,
		GuildID:           m.GuildID,
		ParentChannelID:   parentChannelID,
		UserID:            m.Author.ID,
		ChannelID:         routingChannelID,
		ThreadID:          threadID,
		Text:              text,
		Images:            images,
		Mentioned:         mentioned,
	})
}

func imageAttachments(ctx context.Context, attachments []*discordgo.MessageAttachment) ([]gateway.InboundImage, error) {
	images := make([]gateway.InboundImage, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}
		mediaType := strings.ToLower(strings.TrimSpace(attachment.ContentType))
		if mediaType == "" {
			mediaType = mediaTypeFromFilename(attachment.Filename)
		}
		if !supportedImageMediaTypes[mediaType] {
			continue
		}
		if len(images) >= maxMessageImages {
			return nil, fmt.Errorf("at most %d images are allowed", maxMessageImages)
		}
		if attachment.Size > maxMessageImageBytes {
			return nil, fmt.Errorf("image %q is larger than %d MB", attachment.Filename, maxMessageImageBytes/(1024*1024))
		}
		data, err := downloadAttachment(ctx, attachment.URL, maxMessageImageBytes)
		if err != nil {
			return nil, fmt.Errorf("download image %q: %w", attachment.Filename, err)
		}
		images = append(images, gateway.InboundImage{
			MediaType: mediaType,
			Data:      base64.StdEncoding.EncodeToString(data),
		})
	}
	return images, nil
}

func mediaTypeFromFilename(filename string) string {
	switch strings.ToLower(strings.TrimSpace(filename[strings.LastIndex(filename, ".")+1:])) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}
