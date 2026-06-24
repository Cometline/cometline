package cmd

import (
	"fmt"

	"github.com/cometline/cometmind/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Validate CometMind configuration",
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate cometline-settings.json",
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := config.ValidateCurrentSettings(); err != nil {
			return err
		}
		fmt.Println("settings OK")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configValidateCmd)
	rootCmd.AddCommand(configCmd)
}
