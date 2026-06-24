package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/gateway"
	discordgw "github.com/cometline/cometmind/internal/gateway/discord"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/runtime"
	"github.com/cometline/cometmind/internal/session"
	"github.com/spf13/cobra"
)

var (
	gatewayPlatform string
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Run the messaging gateway (Discord, etc.)",
}

var gatewayRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the messaging gateway",
	RunE:  runGateway,
}

var gatewayShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print Discord gateway configuration (token redacted)",
	RunE:  runGatewayShow,
}

var gatewaySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update Discord gateway settings in cometline-settings.json",
	RunE:  runGatewaySet,
}

var (
	gatewaySetWorkspace       string
	gatewaySetProvider        string
	gatewaySetModel           string
	gatewaySetBotTokenEnv     string
	gatewaySetAllowedUsers    []string
	gatewaySetAllowedChannels []string
	gatewaySetRequireMention  bool
	gatewaySetEnabled         bool
)

func init() {
	gatewayRunCmd.Flags().StringVar(&gatewayPlatform, "platform", "discord", "Platform adapter to start")
	gatewaySetCmd.Flags().StringVar(&gatewaySetWorkspace, "workspace", "", "Default workspace path for Discord sessions")
	gatewaySetCmd.Flags().StringVar(&gatewaySetProvider, "provider", "", "Provider id for Discord sessions")
	gatewaySetCmd.Flags().StringVar(&gatewaySetModel, "model", "", "Model id for Discord sessions")
	gatewaySetCmd.Flags().StringVar(&gatewaySetBotTokenEnv, "bot-token-env", "", "Environment variable name for the Discord bot token")
	gatewaySetCmd.Flags().StringSliceVar(&gatewaySetAllowedUsers, "allowed-user", nil, "Allowed Discord user id (repeatable)")
	gatewaySetCmd.Flags().StringSliceVar(&gatewaySetAllowedChannels, "allowed-channel", nil, "Allowed Discord channel id (repeatable)")
	gatewaySetCmd.Flags().BoolVar(&gatewaySetRequireMention, "require-mention", true, "Require @mention outside threads")
	gatewaySetCmd.Flags().BoolVar(&gatewaySetEnabled, "enabled", true, "Enable Discord gateway")
	gatewayCmd.AddCommand(gatewayRunCmd, gatewayShowCmd, gatewaySetCmd)
	rootCmd.AddCommand(gatewayCmd)
}

func runGateway(_ *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()

	turns := gateway.NewTurnRunTracker()
	rt.SetSessionRunningChecker(turns.Running)
	rt.StartJobsMaintenance(ctx)

	router := &gateway.Router{
		Sessions:     rt.Sessions,
		Config:       rt.Config,
		Jobs:         rt.Jobs,
		Turns:        turns,
		JobProposals: gateway.NewJobProposalStore(),
		Runner: gateway.AgentRunner{
			NewRunner: func(sess session.Session, workspacePath string, msg gateway.InboundMessage) (gateway.TurnRunner, error) {
				channelID := msg.ChannelID
				if msg.ThreadID != "" {
					channelID = msg.ThreadID
				}
				return rt.RunnerForGateway(sess, workspacePath, jobs.PlatformDiscord, channelID)
			},
		},
	}

	switch gatewayPlatform {
	case "discord":
		adapter, err := discordgw.New(rt.Config.Gateway.Discord)
		if err != nil {
			return err
		}
		if n := rt.Jobs.Notifier(); n != nil {
			n.Register(gateway.DiscordJobNotifier{Reply: func(ctx context.Context, msg gateway.OutboundMessage) error {
				return adapter.Deliver(ctx, msg)
			}})
		}
		router.SetReplyHandler(func(ctx context.Context, msg gateway.OutboundMessage) error {
			return adapter.Deliver(ctx, msg)
		})
		router.Typing = adapter
		adapter.SetThreadCreatedHandler(func(ctx context.Context, userID, parentChannelID, threadID string) error {
			return router.EnsureThreadSession(ctx, userID, parentChannelID, threadID)
		})
		adapter.SetChangeWorkspaceHandler(func(ctx context.Context, msg gateway.InboundMessage, path string) (string, error) {
			return router.ChangeWorkspace(ctx, msg, path)
		})
		adapter.SetWorkspaceSuggestHandler(func(ctx context.Context, query string) ([]string, error) {
			return router.SuggestWorkspacePaths(ctx, query, 25)
		})
		adapter.SetJobsHandler(func(ctx context.Context, msg gateway.InboundMessage, jobID string) (string, string, error) {
			return router.HandleJobsSlash(ctx, msg, jobID)
		})
		adapter.SetJobSuggestHandler(func(ctx context.Context, query string) ([]jobs.Job, error) {
			items, err := rt.Jobs.ListReady(ctx)
			if err != nil {
				return nil, err
			}
			query = strings.ToLower(strings.TrimSpace(query))
			if query == "" {
				return items, nil
			}
			filtered := make([]jobs.Job, 0, len(items))
			for _, job := range items {
				if strings.Contains(strings.ToLower(job.ID), query) || strings.Contains(strings.ToLower(job.Description), query) {
					filtered = append(filtered, job)
				}
			}
			return filtered, nil
		})
		router.DeliverJobProposal = adapter.DeliverJobProposal
		adapter.SetCreateJobHandler(func(ctx context.Context, msg gateway.InboundMessage, description, dod, workspace string) (string, error) {
			return router.HandleCreateJobSlash(ctx, msg, description, dod, workspace)
		})
		adapter.SetJobProposalHandlers(
			router.HandleJobProposalSelect,
			router.HandleJobProposalConfirm,
			router.HandleJobProposalCancel,
		)
		adapter.SetInboundHandler(func(ctx context.Context, msg gateway.InboundMessage) {
			if err := router.HandleInbound(ctx, msg); err != nil {
				logging.L().Error("discord.handle_inbound.failed", "error", err)
			}
		})
		if err := adapter.Start(ctx); err != nil {
			return err
		}
		fmt.Printf("cometmind gateway: discord connected (workspace %q)\n", rt.Config.Gateway.Discord.WorkspacePath)
		<-ctx.Done()
		return adapter.Stop(context.Background())
	default:
		return fmt.Errorf("unsupported platform %q", gatewayPlatform)
	}
}

func runGatewayShow(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadDiscordGateway()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.BotToken) != "" {
		cfg.BotToken = "<redacted>"
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func runGatewaySet(cmd *cobra.Command, _ []string) error {
	patch := config.DiscordGatewayPatch{}
	changed := false
	if cmd.Flags().Changed("workspace") {
		patch.WorkspacePath = &gatewaySetWorkspace
		changed = true
	}
	if cmd.Flags().Changed("provider") {
		patch.ProviderID = &gatewaySetProvider
		changed = true
	}
	if cmd.Flags().Changed("model") {
		patch.ModelID = &gatewaySetModel
		changed = true
	}
	if cmd.Flags().Changed("bot-token-env") {
		patch.BotTokenEnv = &gatewaySetBotTokenEnv
		changed = true
	}
	if cmd.Flags().Changed("require-mention") {
		patch.RequireMention = &gatewaySetRequireMention
		changed = true
	}
	if cmd.Flags().Changed("enabled") {
		patch.Enabled = &gatewaySetEnabled
		changed = true
	}
	if cmd.Flags().Changed("allowed-user") {
		patch.AllowedUsers = gatewaySetAllowedUsers
		changed = true
	}
	if cmd.Flags().Changed("allowed-channel") {
		patch.AllowedChannels = gatewaySetAllowedChannels
		changed = true
	}
	if !changed {
		return fmt.Errorf("no gateway fields to update; pass at least one flag")
	}
	if err := config.UpdateDiscordGateway(patch); err != nil {
		return err
	}
	fmt.Println("Discord gateway settings updated")
	return nil
}
