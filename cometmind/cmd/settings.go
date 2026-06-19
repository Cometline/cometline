package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/paths"
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

func init() {
	settingsCmd.AddCommand(settingsPathCmd, settingsShowCmd, settingsExportCmd, settingsImportCmd)
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
