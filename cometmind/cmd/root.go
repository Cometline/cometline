package cmd

import (
	"os"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/paths"
	"github.com/spf13/cobra"
)

var logLevelFlag string

var rootCmd = &cobra.Command{
	Use:   "cometmind",
	Short: "CometMind — local session-first coding agent runtime",
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		raw := strings.TrimSpace(os.Getenv("COMETMIND_LOG_LEVEL"))
		if raw == "" {
			raw = logLevelFlag
		}
		level, err := logging.ParseLevel(raw)
		if err != nil {
			return err
		}
		logging.Init(level)
		return nil
	},
}

// Execute runs the Cobra tree (called from main).
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("workspace", "w", "", "Explicit workspace root directory (defaults to current directory)")
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "error", "Log level: debug, info, warn, or error")
}

// WorkspaceRoot returns the effective workspace directory.
func WorkspaceRoot() (string, error) {
	explicit, _ := rootCmd.PersistentFlags().GetString("workspace")
	return paths.ResolveWorkspace(explicit)
}

// WorkspaceFlag returns the raw --workspace flag value (empty uses cwd).
func WorkspaceFlag() string {
	explicit, _ := rootCmd.PersistentFlags().GetString("workspace")
	return explicit
}
