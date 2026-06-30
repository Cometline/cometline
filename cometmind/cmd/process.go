package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/spf13/cobra"
)

const (
	processModeServe          = "serve"
	processModeGatewayDiscord = "gateway-discord"
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Inspect and control long-lived CometMind processes",
}

var processStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show process metadata for serve and gateway",
	RunE: func(_ *cobra.Command, args []string) error {
		for _, mode := range targetProcessModes(args) {
			state, err := readProcessState(mode)
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
		count, err := signalProcesses(syscall.SIGTERM, targetProcessModes(args)...)
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
		count, err := signalProcesses(syscall.SIGTERM, targetProcessModes(args)...)
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

type processMetadata struct {
	Mode         string `json:"mode"`
	PID          int    `json:"pid"`
	StartedAt    string `json:"started_at"`
	DataDir      string `json:"data_dir"`
	SettingsPath string `json:"settings_path"`
}

type processState struct {
	Metadata processMetadata
	Present  bool
	Running  bool
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

func writeProcessMetadata(mode string) error {
	metaPath, err := paths.ProcessMetaPath(mode)
	if err != nil {
		return err
	}
	pidPath, err := paths.ProcessPIDPath(mode)
	if err != nil {
		return err
	}
	dataDir, err := paths.DataDir()
	if err != nil {
		return err
	}
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return err
	}
	meta := processMetadata{
		Mode:         mode,
		PID:          os.Getpid(),
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
		DataDir:      dataDir,
		SettingsPath: settingsPath,
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(metaPath, data, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(meta.PID)+"\n"), 0o600); err != nil {
		return err
	}
	return nil
}

func removeProcessMetadata(mode string) {
	if metaPath, err := paths.ProcessMetaPath(mode); err == nil {
		_ = os.Remove(metaPath)
	}
	if pidPath, err := paths.ProcessPIDPath(mode); err == nil {
		_ = os.Remove(pidPath)
	}
}

func targetProcessModes(args []string) []string {
	if len(args) == 0 {
		return []string{processModeServe, processModeGatewayDiscord}
	}
	return args
}

func signalProcesses(sig syscall.Signal, modes ...string) (int, error) {
	count := 0
	for _, mode := range modes {
		state, err := readProcessState(mode)
		if err != nil {
			return count, err
		}
		if !state.Running {
			continue
		}
		proc, err := os.FindProcess(state.Metadata.PID)
		if err != nil {
			return count, err
		}
		if err := proc.Signal(sig); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				continue
			}
			return count, err
		}
		count++
	}
	return count, nil
}

func readProcessState(mode string) (processState, error) {
	metaPath, err := paths.ProcessMetaPath(mode)
	if err != nil {
		return processState{}, err
	}
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return processState{}, nil
		}
		return processState{}, err
	}
	var meta processMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return processState{}, err
	}
	state := processState{Metadata: meta, Present: true}
	if meta.PID <= 0 {
		return state, nil
	}
	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return state, nil
	}
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		state.Running = true
		return state, nil
	} else if !strings.Contains(err.Error(), "operation not permitted") {
		return state, nil
	}
	state.Running = true
	return state, nil
}
