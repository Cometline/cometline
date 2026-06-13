package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/tools"
	"github.com/cometline/cometmind/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive Bubble Tea UI for CometMind sessions",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	root, err := WorkspaceRoot()
	if err != nil {
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

	svc := session.New(sqlDB)
	ws, err := svc.EnsureWorkspace(ctx, root)
	if err != nil {
		return err
	}

	wsPath, err := svc.WorkspacePath(ctx, ws.ID)
	if err != nil {
		return err
	}

	if _, err := provider.New(cfg); err != nil {
		return fmt.Errorf("provider: %w", err)
	}
	_ = tools.NewRegistry(wsPath)

	deps := &tui.Deps{
		Config:        cfg,
		Sessions:      svc,
		WorkspacePath: wsPath,
		WorkspaceID:   ws.ID,
	}

	app := tui.NewApp(deps)
	p := tea.NewProgram(app, tea.WithAltScreen())
	app.SetProgram(p)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
