package discord

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/gateway"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
)

// PlatformName is the normalized platform identifier for Discord.
const PlatformName = "discord"

// Adapter connects CometMind to Discord via discordgo.
type Adapter struct {
	Config               config.DiscordGatewayConfig
	Session              *discordgo.Session
	onInbound            func(context.Context, gateway.InboundMessage)
	onThread             func(context.Context, string, string, string) error
	onChange             func(context.Context, gateway.InboundMessage, string) (string, error)
	onClear              func(context.Context, gateway.InboundMessage) (string, error)
	onStop               func(context.Context, gateway.InboundMessage) (string, error)
	onSuggest            func(context.Context, string) ([]string, error)
	onJobs               func(context.Context, gateway.InboundMessage, string) (string, string, error)
	jobSuggest           func(context.Context, string) ([]jobs.Job, error)
	onCreateJob          func(context.Context, gateway.InboundMessage, string, string, string) (string, error)
	onJobProposalSelect  func(string, string) (string, error)
	onJobProposalConfirm func(context.Context, gateway.InboundMessage, string) (string, error)
	onJobProposalCancel  func(string) error

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

// SetClearHandler registers the callback used for /clear slash commands.
func (a *Adapter) SetClearHandler(fn func(context.Context, gateway.InboundMessage) (string, error)) {
	a.onClear = fn
}

// SetStopHandler registers the callback used for /stop slash commands.
func (a *Adapter) SetStopHandler(fn func(context.Context, gateway.InboundMessage) (string, error)) {
	a.onStop = fn
}

// SetWorkspaceSuggestHandler registers autocomplete suggestions for /change path.
func (a *Adapter) SetWorkspaceSuggestHandler(fn func(context.Context, string) ([]string, error)) {
	a.onSuggest = fn
}

// SetJobsHandler registers the callback used for /jobs slash commands.
func (a *Adapter) SetJobsHandler(fn func(context.Context, gateway.InboundMessage, string) (string, string, error)) {
	a.onJobs = fn
}

// SetJobSuggestHandler registers job autocomplete for /jobs.
func (a *Adapter) SetJobSuggestHandler(fn func(context.Context, string) ([]jobs.Job, error)) {
	a.jobSuggest = fn
}

// SetCreateJobHandler registers the callback used for /create-job slash commands.
func (a *Adapter) SetCreateJobHandler(fn func(context.Context, gateway.InboundMessage, string, string, string) (string, error)) {
	a.onCreateJob = fn
}

// SetJobProposalHandlers registers callbacks for job proposal component interactions.
func (a *Adapter) SetJobProposalHandlers(
	onSelect func(string, string) (string, error),
	onConfirm func(context.Context, gateway.InboundMessage, string) (string, error),
	onCancel func(string) error,
) {
	a.onJobProposalSelect = onSelect
	a.onJobProposalConfirm = onConfirm
	a.onJobProposalCancel = onCancel
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
			logging.L().Error("discord.slash_commands.register_failed", "error", err)
		} else {
			logging.L().Info("discord.slash_commands.registered")
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
			Name:        "clear",
			Description: "Clear this channel's CometMind conversation transcript",
		},
		{
			Name:        "stop",
			Description: "Stop the active CometMind turn in this channel",
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
		{
			Name:        "jobs",
			Description: "List ready jobs or claim one to run",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "job",
					Description:  "Job ID to claim (optional)",
					Required:     false,
					Autocomplete: true,
				},
			},
		},
		createJobApplicationCommand(),
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
	_ = ctx
	if a.Session != nil {
		return a.Session.Close()
	}
	return nil
}

func (a *Adapter) Deliver(ctx context.Context, msg gateway.OutboundMessage) error {
	_ = ctx
	dest := deliveryChannelID(msg)
	for _, chunk := range splitMessage(msg.Text, 1900) {
		if _, err := a.Session.ChannelMessageSend(dest, chunk); err != nil {
			return err
		}
	}
	return nil
}
