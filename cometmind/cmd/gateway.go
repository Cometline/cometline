package cmd

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/cometline/cometmind/internal/gateway"
	discordgw "github.com/cometline/cometmind/internal/gateway/discord"
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

	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()

	router := &gateway.Router{
		Sessions: rt.Sessions,
		Config:   rt.Config,
		ACPMgr:   rt.ACPManager(),
		Runner: gateway.AgentRunner{
			NewRunner: func(sess session.Session, workspacePath string) (gateway.TurnRunner, error) {
				return rt.RunnerFor(sess, workspacePath)
			},
		},
	}

	switch gatewayPlatform {
	case "discord":
		adapter, err := discordgw.New(rt.Config.Gateway.Discord)
		if err != nil {
			return err
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
		adapter.SetInboundHandler(func(ctx context.Context, msg gateway.InboundMessage) {
			if err := router.HandleInbound(ctx, msg); err != nil {
				log.Printf("discord: handle inbound failed: %v", err)
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
