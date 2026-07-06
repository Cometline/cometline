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
	Short: "Restart running CometMind processes",
	Long:  "Stops the target process and relaunches it using its recorded command arguments.",
	RunE: func(_ *cobra.Command, args []string) error {
		modes, err := processctl.TargetModes(args)
		if err != nil {
			return err
		}
		restarted := 0
		for _, mode := range modes {
			state, err := processctl.ReadState(mode)
			if err != nil {
				return err
			}
			if !state.Running && !state.Present {
				continue
			}
			if err := processctl.Restart(mode, 10*time.Second); err != nil {
				return fmt.Errorf("restart %s: %w", mode, err)
			}
			fmt.Printf("restarted %s\n", mode)
			restarted++
		}
		if restarted == 0 {
			return fmt.Errorf("no running CometMind processes found")
		}
		return nil
	},
}

func init() {
	processCmd.AddCommand(processStatusCmd, processStopCmd, processRestartCmd)
	rootCmd.AddCommand(processCmd)
}

// handleReloadSignal waits for SIGHUP and runs reload, recording the outcome
// via processctl.WriteReloadResult under mode so a separate short-lived CLI
// invocation (`cometmind settings reload`) can confirm the reload actually
// completed — and why it failed, if it did — instead of only knowing the
// signal was delivered.
func handleReloadSignal(ctx context.Context, hupCh <-chan os.Signal, mode string, reload func(context.Context) error) {
	var generation int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-hupCh:
			generation++
			reloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := reload(reloadCtx)
			cancel()
			result := processctl.ReloadResult{
				Generation: generation,
				Success:    err == nil,
				FinishedAt: time.Now().UTC().Format(time.RFC3339),
			}
			if err != nil {
				logging.L().Error("runtime.reload_failed", "error", err)
				result.Error = err.Error()
			}
			if werr := processctl.WriteReloadResult(mode, result); werr != nil {
				logging.L().Warn("runtime.reload_result_write_failed", "error", werr)
			}
		}
	}
}
