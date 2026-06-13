package cmd

import (
	"context"
	"fmt"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
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
	if _, err := config.Load(); err != nil {
		return err
	}
	dbpath, err := paths.DBPath()
	if err != nil {
		return err
	}
	sqlDB, err := store.OpenSQLite(ctx, dbpath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	root, err := WorkspaceRoot()
	if err != nil {
		return err
	}
	svc := session.New(sqlDB)
	ws, err := svc.EnsureWorkspace(ctx, root)
	if err != nil {
		return err
	}
	fmt.Printf("CometMind ready.\nWorkspace %s registered as %s (%s)\n", ws.Name, ws.ID, ws.Path)
	return nil
}
