package cmd

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/processctl"
	"github.com/spf13/cobra"
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Inspect and control long-lived CometMind processes",
}

var processStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show process metadata for serve and gateway",
	RunE: func(_ *cobra.Command, args []string) error {
		modes, err := processctl.TargetModes(args)
		if err != nil {
			return err
		}
		for _, mode := range modes {
			state, err := processctl.ReadState(mode)
			if err != nil {
				return err
			}
			if !state.Present {
				fmt.Printf("%s: not running\n", mode)
				continue
			}
			status := "stale"
			if state.Running {
				status = "running"
			}
			fmt.Printf("%s: %s pid=%d started_at=%s data_dir=%s settings=%s\n", mode, status, state.Metadata.PID, state.Metadata.StartedAt, state.Metadata.DataDir, state.Metadata.SettingsPath)
		}
		return nil
	},
}

var processStopCmd = &cobra.Command{
	Use:   "stop [serve|gateway-discord]",
	Short: "Gracefully stop running CometMind processes",
	RunE: func(_ *cobra.Command, args []string) error {
		modes, err := processctl.TargetModes(args)
		if err != nil {
			return err
		}
		count, err := processctl.Signal(syscall.SIGTERM, modes...)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("no running CometMind processes found")
		}
		fmt.Printf("requested stop for %d process(es)\n", count)
		return nil
	},
}

var processRestartCmd = &cobra.Command{
	Use:   "restart [serve|gateway-discord]",
	Short: "Request graceful restart for running CometMind processes",
	Long:  "This sends SIGTERM to the target process. If the process is supervised by Electron, Docker, systemd, or another process manager, it should come back automatically.",
	RunE: func(_ *cobra.Command, args []string) error {
		modes, err := processctl.TargetModes(args)
		if err != nil {
			return err
		}
		count, err := processctl.Signal(syscall.SIGTERM, modes...)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("no running CometMind processes found")
		}
		fmt.Printf("requested restart for %d process(es); supervised processes should restart automatically\n", count)
		return nil
	},
}

func init() {
	processCmd.AddCommand(processStatusCmd, processStopCmd, processRestartCmd)
	rootCmd.AddCommand(processCmd)
}

func handleReloadSignal(ctx context.Context, hupCh <-chan os.Signal, reload func(context.Context) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-hupCh:
			reloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := reload(reloadCtx); err != nil {
				logging.L().Error("runtime.reload_failed", "error", err)
			} else {
				logging.L().Info("runtime.reload_requested")
			}
			cancel()
		}
	}
}
