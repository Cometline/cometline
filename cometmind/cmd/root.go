package cmd

import (
	"github.com/cometline/cometmind/internal/paths"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cometmind",
	Short: "CometMind — local session-first coding agent runtime",
}

// Execute runs the Cobra tree (called from main).
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("workspace", "w", "", "Explicit workspace root directory (defaults to current directory)")
}

// WorkspaceRoot returns the effective workspace directory.
func WorkspaceRoot() (string, error) {
	explicit, _ := rootCmd.PersistentFlags().GetString("workspace")
	return paths.ResolveWorkspace(explicit)
}
