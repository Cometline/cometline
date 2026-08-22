package discord

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
)

const (
	maxMessageImages     = 6
	maxMessageImageBytes = 10 * 1024 * 1024
)

var supportedImageMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

var attachmentHTTPClient = &http.Client{Timeout: 30 * time.Second}

// deliveryChannelID returns the Discord channel to post in. Thread replies must
// target the thread channel ID, not the parent text channel.
func deliveryChannelID(msg gateway.OutboundMessage) string {
	if msg.ThreadID != "" {
		return msg.ThreadID
	}
	return msg.ChannelID
}

// discordRoutingIDs maps a Discord message channel to gateway routing keys.
// Thread messages use the parent channel as platform_channel_id and the thread
// ID as thread_id so each thread gets its own CometMind session.
func discordRoutingIDs(channelID, parentChannelID string) (routingChannelID, threadID string) {
	if parentChannelID != "" {
		return parentChannelID, channelID
	}
	return channelID, ""
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func stripBotMentions(content string, state *discordgo.State) string {
	text := content
	if state != nil && state.User != nil {
		text = strings.ReplaceAll(text, "<@"+state.User.ID+">", "")
		text = strings.ReplaceAll(text, "<@!"+state.User.ID+">", "")
	}
	return strings.TrimSpace(text)
}

func truncateLog(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func splitMessage(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	var out []string
	for len(text) > limit {
		out = append(out, text[:limit])
		text = text[limit:]
	}
	if text != "" {
		out = append(out, text)
	}
	return out
}

// routingInboundMessage builds a gateway inbound message from an interaction.
func routingInboundMessage(s *discordgo.Session, i *discordgo.InteractionCreate) gateway.InboundMessage {
	parentChannelID := ""
	if i.GuildID != "" {
		if ch, err := s.Channel(i.ChannelID); err == nil && ch != nil && ch.ParentID != "" {
			parentChannelID = ch.ParentID
		}
	}
	routingChannelID, threadID := discordRoutingIDs(i.ChannelID, parentChannelID)
	return gateway.InboundMessage{
		Platform:        PlatformName,
		GuildID:         i.GuildID,
		ParentChannelID: parentChannelID,
		UserID:          interactionUserID(i),
		ChannelID:       routingChannelID,
		ThreadID:        threadID,
		Mentioned:       true,
	}
}

func downloadAttachment(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := attachmentHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("image is larger than %d MB", maxBytes/(1024*1024))
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("image is empty")
	}
	return data, nil
}
