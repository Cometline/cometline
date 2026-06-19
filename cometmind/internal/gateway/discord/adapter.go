package discord

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/gateway"
	skillpkg "github.com/cometline/cometmind/internal/skills"
)

const platformName = "discord"

const (
	maxMessageImages     = 6
	maxMessageImageBytes = 4 * 1024 * 1024
)

var supportedImageMediaTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
}

var attachmentHTTPClient = &http.Client{Timeout: 30 * time.Second}

// Adapter connects CometMind to Discord via discordgo.
type Adapter struct {
	Config    config.DiscordGatewayConfig
	Session   *discordgo.Session
	onInbound func(context.Context, gateway.InboundMessage)
	onThread  func(context.Context, string, string, string) error
	onChange  func(context.Context, gateway.InboundMessage, string) (string, error)
	onSuggest func(context.Context, string) ([]string, error)

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

// SetChangeWorkspaceHandler registers the callback used for /change slash commands.
func (a *Adapter) SetChangeWorkspaceHandler(fn func(context.Context, gateway.InboundMessage, string) (string, error)) {
	a.onChange = fn
}

// SetWorkspaceSuggestHandler registers autocomplete suggestions for /change path.
func (a *Adapter) SetWorkspaceSuggestHandler(fn func(context.Context, string) ([]string, error)) {
	a.onSuggest = fn
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
	a.Session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		if err := a.registerCommands(s, r); err != nil {
			log.Printf("discord: slash command registration failed: %v", err)
		} else {
			log.Printf("discord: slash commands registered (thread, create-skill, change)")
		}
	})
	if err := a.Session.Open(); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = a.Stop(context.Background())
	}()
	return nil
}

func applicationCommands() []*discordgo.ApplicationCommand {
	return []*discordgo.ApplicationCommand{
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
		{
			Name:        "create-skill",
			Description: "Build a new Agent Skill in ~/.cometmind/skills",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "request",
					Description: "What the skill should do (optional)",
					Required:    false,
				},
			},
		},
		{
			Name:        "change",
			Description: "Switch workspace for this thread's session",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "path",
					Description:  "Absolute path to project root",
					Required:     true,
					Autocomplete: true,
				},
			},
		},
	}
}

func (a *Adapter) registerCommands(s *discordgo.Session, ready *discordgo.Ready) error {
	appID := ""
	if ready != nil && ready.Application.ID != "" {
		appID = ready.Application.ID
	}
	if appID == "" && s.State != nil && s.State.User != nil {
		appID = s.State.User.ID
	}
	if appID == "" {
		return fmt.Errorf("discord application id is not available")
	}
	_, err := s.ApplicationCommandBulkOverwrite(appID, "", applicationCommands())
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

func (a *Adapter) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommandAutocomplete:
		a.handleAutocomplete(s, i)
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()
		switch data.Name {
		case "thread":
			a.handleThreadCommand(s, i, data)
		case "create-skill":
			a.handleCreateSkillCommand(s, i, data)
		case "change":
			a.handleChangeCommand(s, i, data)
		}
	}
}

func (a *Adapter) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name != "change" || a.onSuggest == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	query := ""
	for _, opt := range data.Options {
		if opt.Name == "path" && opt.Focused {
			query = opt.StringValue()
			break
		}
	}

	paths, err := a.onSuggest(context.Background(), query)
	if err != nil {
		log.Printf("discord: workspace autocomplete failed: %v", err)
		paths = nil
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(paths))
	for _, path := range paths {
		if len(choices) >= 25 {
			break
		}
		name := path
		if len(name) > 100 {
			name = "…" + name[len(name)-99:]
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: path,
		})
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func (a *Adapter) handleChangeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	path := ""
	for _, opt := range data.Options {
		if opt.Name == "path" && opt.Type == discordgo.ApplicationCommandOptionString {
			path = strings.TrimSpace(opt.StringValue())
		}
	}
	if path == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Workspace path is required.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if a.onChange == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Workspace switching is not configured.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	msg := routingInboundMessage(s, i)
	text, err := a.onChange(context.Background(), msg, path)
	if err != nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to change workspace: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: text,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func routingInboundMessage(s *discordgo.Session, i *discordgo.InteractionCreate) gateway.InboundMessage {
	parentChannelID := ""
	if i.GuildID != "" {
		if ch, err := s.Channel(i.ChannelID); err == nil && ch != nil && ch.ParentID != "" {
			parentChannelID = ch.ParentID
		}
	}
	routingChannelID, threadID := discordRoutingIDs(i.ChannelID, parentChannelID)
	return gateway.InboundMessage{
		Platform:        platformName,
		GuildID:         i.GuildID,
		ParentChannelID: parentChannelID,
		UserID:          interactionUserID(i),
		ChannelID:       routingChannelID,
		ThreadID:        threadID,
		Mentioned:       true,
	}
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

	if !isForumLikeChannelType(parentType) {
		if _, err := s.ChannelMessageSend(thread.ID, welcome); err != nil {
			log.Printf("discord: thread welcome message failed: %v", err)
		}
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
	if a.Config.RequireMention && !mentioned && threadID == "" {
		log.Printf("discord: ignoring message in channel %s (mention required)", m.ChannelID)
		return
	}

	text := strings.TrimSpace(stripBotMentions(m.Content, s.State))
	images, err := imageAttachments(context.Background(), m.Attachments)
	if err != nil {
		log.Printf("discord: ignoring message in channel %s (attachments unsupported: %v)", m.ChannelID, err)
		return
	}
	if text == "" && len(images) == 0 {
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
		"discord: inbound user=%s channel=%s thread=%s parent=%s guild=%s text=%q images=%d",
		m.Author.ID,
		routingChannelID,
		threadID,
		parentChannelID,
		m.GuildID,
		truncateLog(text, 80),
		len(images),
	)
	a.onInbound(context.Background(), gateway.InboundMessage{
		Platform:        platformName,
		GuildID:         m.GuildID,
		ParentChannelID: parentChannelID,
		UserID:          m.Author.ID,
		ChannelID:       routingChannelID,
		ThreadID:        threadID,
		Text:            text,
		Images:          images,
		Mentioned:       mentioned,
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

func (a *Adapter) handleCreateSkillCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	request := ""
	for _, opt := range data.Options {
		if opt.Name == "request" && opt.Type == discordgo.ApplicationCommandOptionString {
			if v := strings.TrimSpace(opt.StringValue()); v != "" {
				request = v
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Creating skill… CometMind will reply in this channel.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	if a.onInbound == nil {
		return
	}

	parentChannelID := ""
	if i.GuildID != "" {
		if ch, err := s.Channel(i.ChannelID); err == nil && ch != nil && ch.ParentID != "" {
			parentChannelID = ch.ParentID
		}
	}
	routingChannelID, threadID := discordRoutingIDs(i.ChannelID, parentChannelID)
	userID := interactionUserID(i)
	if userID == "" {
		return
	}

	a.onInbound(context.Background(), gateway.InboundMessage{
		Platform:        platformName,
		GuildID:         i.GuildID,
		ParentChannelID: parentChannelID,
		UserID:          userID,
		ChannelID:       routingChannelID,
		ThreadID:        threadID,
		Text:            skillpkg.ExpandCreateSkillCommand(request),
		Mentioned:       true,
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
