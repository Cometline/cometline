package cmd

import (
	"os"
	"path/filepath"

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
	if w, err := rootCmd.PersistentFlags().GetString("workspace"); err == nil && w != "" {
		return absDir(w)
	}
	return os.Getwd()
}

func absDir(path string) (string, error) {
	if path == "" {
		return os.Getwd()
	}
	return absPath(path)
}

func absPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(wd, path)
	}
	return filepath.Clean(path), nil
}
