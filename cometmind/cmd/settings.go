package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/processctl"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Inspect and import Cometline settings",
}

var settingsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the Cometline settings file path",
	RunE: func(_ *cobra.Command, _ []string) error {
		path, err := paths.SettingsPath()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

var settingsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print saved Cometline settings as JSON",
	RunE: func(_ *cobra.Command, _ []string) error {
		data, _, err := readSavedSettingsJSON()
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

var settingsExportCmd = &cobra.Command{
	Use:   "export [file]",
	Short: "Export saved Cometline settings JSON",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		data, _, err := readSavedSettingsJSON()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			_, err = os.Stdout.Write(data)
			return err
		}
		if err := os.WriteFile(args[0], data, 0o600); err != nil {
			return err
		}
		fmt.Printf("exported settings to %s\n", args[0])
		return nil
	},
}

var settingsImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Validate and import Cometline settings JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		formatted, err := formatSettingsJSON(data)
		if err != nil {
			return err
		}
		if err := config.ValidateCometlineSettingsJSON(formatted); err != nil {
			return err
		}
		settingsPath, err := paths.SettingsPath()
		if err != nil {
			return err
		}
		if err := os.WriteFile(settingsPath, formatted, 0o600); err != nil {
			return err
		}
		fmt.Printf("imported settings to %s\n", settingsPath)
		return nil
	},
}

// reloadConfirmTimeout bounds how long `settings reload` waits for the
// running process to actually finish applying the reload (config re-read +
// MCP manager Reload) before giving up and reporting an unconfirmed reload.
// Must comfortably exceed Runtime.Reload's own internal reload context
// timeout (30s) plus MCP connect fan-out time.
const reloadConfirmTimeout = 35 * time.Second

var settingsReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Request running CometMind processes to reload settings and confirm the result",
	RunE: func(_ *cobra.Command, _ []string) error {
		modes := []string{processctl.ModeServe, processctl.ModeGatewayDiscord}
		baseline := make(map[string]int64, len(modes))
		targets := make([]string, 0, len(modes))
		for _, mode := range modes {
			state, err := processctl.ReadState(mode)
			if err != nil {
				return err
			}
			if !state.Running {
				continue
			}
			result, err := processctl.ReadReloadResult(mode)
			if err != nil {
				return err
			}
			baseline[mode] = result.Generation
			targets = append(targets, mode)
		}
		if len(targets) == 0 {
			return fmt.Errorf("no running CometMind processes found")
		}

		count, err := processctl.Signal(syscall.SIGHUP, targets...)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("no running CometMind processes found")
		}

		var failures []string
		for _, mode := range targets {
			result, confirmed := processctl.WaitForReloadGeneration(mode, baseline[mode], reloadConfirmTimeout)
			if !confirmed {
				failures = append(failures, fmt.Sprintf("%s: reload did not confirm within %s", mode, reloadConfirmTimeout))
				continue
			}
			if !result.Success {
				failures = append(failures, fmt.Sprintf("%s: %s", mode, result.Error))
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("settings reload failed for %d process(es):\n%s", len(failures), joinLines(failures))
		}
		fmt.Printf("confirmed settings reload for %d process(es)\n", count)
		return nil
	},
}

func joinLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += "  - " + line
	}
	return out
}

func init() {
	settingsCmd.AddCommand(settingsPathCmd, settingsShowCmd, settingsExportCmd, settingsImportCmd, settingsReloadCmd)
	rootCmd.AddCommand(settingsCmd)
}

func readSavedSettingsJSON() ([]byte, string, error) {
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("settings file does not exist at %s; run `cometmind init` or open Cometline first", settingsPath)
		}
		return nil, "", err
	}
	formatted, err := formatSettingsJSON(data)
	if err != nil {
		return nil, "", err
	}
	return formatted, settingsPath, nil
}

func formatSettingsJSON(data []byte) ([]byte, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse settings JSON: %w", err)
	}
	formatted, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(formatted, '\n'), nil
}
