package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Inspect persisted sessions for the current workspace",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions for the workspace rooted at the current directory (or --workspace)",
	RunE:  runSessionList,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd)
	rootCmd.AddCommand(sessionCmd)
}

func runSessionList(_ *cobra.Command, _ []string) error {
	ctx := context.Background()
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
	list, err := svc.ListSessions(ctx, ws.ID)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTITLE\tMODEL\tUPDATED")
	for _, s := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", s.ID, s.Title, s.ModelID, s.UpdatedAt)
	}
	return tw.Flush()
}
