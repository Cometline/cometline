package cmd

import (
	"context"
	"fmt"

	"github.com/cometline/cometmind/internal/runtime"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create ~/.cometmind config and database if missing (optional convenience)",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, _ []string) error {
	ctx := context.Background()
	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()

	ws, err := rt.WorkspaceForCommand(ctx, "")
	if err != nil {
		return err
	}
	fmt.Printf("CometMind ready.\nWorkspace %s registered as %s (%s)\n", ws.Name, ws.ID, ws.Path)
	return nil
}
