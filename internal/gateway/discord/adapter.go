package discord

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/gateway"
)

const platformName = "discord"

// Adapter connects CometMind to Discord via discordgo.
type Adapter struct {
	Config    config.DiscordGatewayConfig
	Session   *discordgo.Session
	onInbound func(context.Context, gateway.InboundMessage)
	onThread  func(context.Context, string, string, string) error

	mu sync.Mutex
}

// New creates a Discord adapter from config.
func New(cfg config.DiscordGatewayConfig) (*Adapter, error) {
	token, err := resolveBotToken(cfg)
	if err != nil {
		return nil, err
	}
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	s.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentMessageContent |
		discordgo.IntentsGuilds
	return &Adapter{Config: cfg, Session: s}, nil
}

func resolveBotToken(cfg config.DiscordGatewayConfig) (string, error) {
	if token := strings.TrimSpace(cfg.BotToken); token != "" {
		return token, nil
	}
	env := strings.TrimSpace(cfg.BotTokenEnv)
	if looksLikeDiscordBotToken(env) {
		return env, nil
	}
	if env == "" {
		env = "DISCORD_BOT_TOKEN"
	}
	token := strings.TrimSpace(os.Getenv(env))
	if token == "" {
		return "", fmt.Errorf(
			"discord bot token is not configured (set bot_token in config.toml or export %q)",
			env,
		)
	}
	return token, nil
}

// looksLikeDiscordBotToken detects when bot_token_env was set to the token itself.
func looksLikeDiscordBotToken(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	return len(parts[0]) >= 18 && len(parts[1]) >= 4 && len(parts[2]) >= 20
}

func (a *Adapter) SetInboundHandler(fn func(context.Context, gateway.InboundMessage)) {
	a.onInbound = fn
}

// SetThreadCreatedHandler registers a callback invoked when /thread creates a new thread.
// The callback receives userID, parentChannelID, and threadID.
func (a *Adapter) SetThreadCreatedHandler(fn func(context.Context, string, string, string) error) {
	a.onThread = fn
}

// KeepTyping sends ChannelTyping periodically until stop is called.
func (a *Adapter) KeepTyping(ctx context.Context, channelID string) func() {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		_ = a.Session.ChannelTyping(channelID)
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				_ = a.Session.ChannelTyping(channelID)
			}
		}
	}()
	return func() { close(stop) }
}

func (a *Adapter) Start(ctx context.Context) error {
	a.Session.AddHandler(a.onMessageCreate)
	a.Session.AddHandler(a.onInteractionCreate)
	if err := a.Session.Open(); err != nil {
		return err
	}
	if err := a.registerCommands(); err != nil {
		log.Printf("discord: slash command registration failed: %v", err)
	}
	go func() {
		<-ctx.Done()
		_ = a.Stop(context.Background())
	}()
	return nil
}

func (a *Adapter) registerCommands() error {
	if a.Session.State == nil || a.Session.State.User == nil {
		return fmt.Errorf("discord session user is not ready")
	}
	_, err := a.Session.ApplicationCommandBulkOverwrite(a.Session.State.User.ID, "", []*discordgo.ApplicationCommand{
		{
			Name:        "thread",
			Description: "Start a new CometMind conversation in a thread",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "name",
					Description: "Thread name (optional)",
					Required:    false,
				},
			},
		},
	})
	return err
}

func (a *Adapter) Stop(ctx context.Context) error {
	if a.Session != nil {
		return a.Session.Close()
	}
	return nil
}

func (a *Adapter) Deliver(ctx context.Context, msg gateway.OutboundMessage) error {
	dest := deliveryChannelID(msg)
	for _, chunk := range splitMessage(msg.Text, 1900) {
		if _, err := a.Session.ChannelMessageSend(dest, chunk); err != nil {
			return err
		}
	}
	return nil
}

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

func (a *Adapter) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	if data.Name != "thread" {
		return
	}
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

	parentChannelID := i.ChannelID
	if ch, err := s.Channel(i.ChannelID); err == nil && ch != nil && ch.ParentID != "" {
		parentChannelID = ch.ParentID
	}

	thread, err := s.ThreadStart(
		parentChannelID,
		threadName,
		discordgo.ChannelTypeGuildPublicThread,
		60,
	)
	if err != nil {
		log.Printf("discord: thread create failed: %v", err)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to create thread: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	welcome := "New CometMind session started in this thread. Send a message here to talk to the agent."
	if _, err := s.ChannelMessageSend(thread.ID, welcome); err != nil {
		log.Printf("discord: thread welcome message failed: %v", err)
	}

	if a.onThread != nil {
		userID := interactionUserID(i)
		if userID != "" {
			if err := a.onThread(context.Background(), userID, parentChannelID, thread.ID); err != nil {
				log.Printf("discord: thread session setup failed: %v", err)
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

	text := strings.TrimSpace(stripBotMentions(m.Content, s.State))
	if text == "" {
		if strings.TrimSpace(m.Content) != "" {
			log.Printf("discord: ignoring message in channel %s (only mentions, no text)", m.ChannelID)
		} else if m.GuildID != "" {
			log.Printf(
				"discord: ignoring guild message in channel %s (empty content); enable Message Content Intent in the Discord Developer Portal",
				m.ChannelID,
			)
		}
		return
	}
	log.Printf(
		"discord: inbound user=%s channel=%s thread=%s parent=%s guild=%s text=%q",
		m.Author.ID,
		routingChannelID,
		threadID,
		parentChannelID,
		m.GuildID,
		truncateLog(text, 80),
	)
	a.onInbound(context.Background(), gateway.InboundMessage{
		Platform:        platformName,
		GuildID:         m.GuildID,
		ParentChannelID: parentChannelID,
		UserID:          m.Author.ID,
		ChannelID:       routingChannelID,
		ThreadID:        threadID,
		Text:            text,
		Mentioned:       mentioned,
	})
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
