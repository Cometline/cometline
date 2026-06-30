package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

func init() {
	gatewayRunCmd.Flags().StringVar(&gatewayPlatform, "platform", "discord", "Platform adapter to start")
	gatewayCmd.AddCommand(gatewayRunCmd)
	rootCmd.AddCommand(gatewayCmd)
}

func runGateway(_ *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	defer signal.Stop(hupCh)

	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()
	if gatewayPlatform == "discord" {
		if err := writeProcessMetadata(processModeGatewayDiscord); err != nil {
			return err
		}
		defer removeProcessMetadata(processModeGatewayDiscord)
	}
	go handleReloadSignal(ctx, hupCh, func(reloadCtx context.Context) error {
		return rt.Reload(reloadCtx)
	})

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
			return router.EnsureThreadSession(ctx, discordgw.PlatformName, userID, parentChannelID, threadID)
		})
		adapter.SetChangeWorkspaceHandler(func(ctx context.Context, msg gateway.InboundMessage, path string) (string, error) {
			return router.ChangeWorkspace(ctx, msg, path)
		})
		adapter.SetClearHandler(func(ctx context.Context, msg gateway.InboundMessage) (string, error) {
			return router.HandleClearSlash(ctx, msg)
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
