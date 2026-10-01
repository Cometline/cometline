package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Cometline/cometline/cometmind/internal/runtime"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/spf13/cobra"
)

var sessionListAll bool
var sessionListJSON bool
var sessionListWorkspaceID string

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Inspect persisted sessions",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions for a workspace or across all workspaces",
	RunE:  runSessionList,
}

var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session and its transcript",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionDelete,
}

var sessionRenameCmd = &cobra.Command{
	Use:   "rename <session-id>",
	Short: "Rename a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionRename,
}

var sessionSetModelCmd = &cobra.Command{
	Use:   "set-model <session-id>",
	Short: "Switch the model for a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionSetModel,
}

var sessionRenameName string
var sessionSetModelID string
var sessionSetProviderID string

func init() {
	sessionListCmd.Flags().BoolVar(&sessionListAll, "all", false, "List top-level sessions across all workspaces")
	sessionListCmd.Flags().BoolVar(&sessionListJSON, "json", false, "Emit SessionListResponse JSON")
	sessionListCmd.Flags().StringVar(&sessionListWorkspaceID, "workspace-id", "", "List sessions for a workspace by id")
	sessionRenameCmd.Flags().StringVar(&sessionRenameName, "name", "", "New session title")
	_ = sessionRenameCmd.MarkFlagRequired("name")
	sessionSetModelCmd.Flags().StringVar(&sessionSetModelID, "model", "", "Model id")
	sessionSetModelCmd.Flags().StringVar(&sessionSetProviderID, "provider", "", "Provider id")
	_ = sessionSetModelCmd.MarkFlagRequired("model")
	_ = sessionSetModelCmd.MarkFlagRequired("provider")
	sessionCmd.AddCommand(sessionListCmd, sessionDeleteCmd, sessionRenameCmd, sessionSetModelCmd)
	rootCmd.AddCommand(sessionCmd)
}

func runSessionList(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	all, _ := cmd.Flags().GetBool("all")
	jsonOut, _ := cmd.Flags().GetBool("json")
	workspaceID, _ := cmd.Flags().GetString("workspace-id")

	if all && workspaceID != "" {
		return fmt.Errorf("--all cannot be used with --workspace-id")
	}

	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()

	var sessions []session.Session
	var pathByID map[string]string

	switch {
	case all:
		sessions, pathByID, err = listAllSessions(ctx, rt)
	case workspaceID != "":
		sessions, pathByID, err = listByWorkspaceID(ctx, rt, workspaceID)
	default:
		sessions, pathByID, err = listByWorkspacePath(ctx, rt)
	}
	if err != nil {
		return err
	}

	wireSessions, err := session.APISessionList(sessions, pathByID)
	if err != nil {
		return err
	}

	if jsonOut {
		data, err := json.MarshalIndent(sessionListResponse{Sessions: wireSessions}, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		_, err = os.Stdout.Write(data)
		return err
	}

	return printSessionTable(wireSessions, all)
}

func runSessionDelete(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	sessionID := strings.TrimSpace(args[0])

	rt, err := openRuntime(ctx)
	if err != nil {
		return err
	}
	defer closeRuntime(rt)

	if err := requireSession(ctx, rt, sessionID); err != nil {
		return err
	}
	if err := rt.Sessions.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	fmt.Printf("Deleted session %s\n", sessionID)
	return nil
}

func runSessionRename(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	sessionID := strings.TrimSpace(args[0])
	name := strings.TrimSpace(sessionRenameName)
	if name == "" {
		return fmt.Errorf("--name is required")
	}

	rt, err := openRuntime(ctx)
	if err != nil {
		return err
	}
	defer closeRuntime(rt)

	if err := requireSession(ctx, rt, sessionID); err != nil {
		return err
	}
	sess, err := rt.Sessions.UpdateSessionTitle(ctx, sessionID, name)
	if err != nil {
		return err
	}
	fmt.Printf("Renamed session %s to %q\n", sess.ID, sess.Title)
	return nil
}

func runSessionSetModel(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	sessionID := strings.TrimSpace(args[0])
	modelID := strings.TrimSpace(sessionSetModelID)
	providerID := strings.TrimSpace(sessionSetProviderID)
	if modelID == "" || providerID == "" {
		return fmt.Errorf("--model and --provider are required")
	}

	rt, err := openRuntime(ctx)
	if err != nil {
		return err
	}
	defer closeRuntime(rt)

	if err := requireSession(ctx, rt, sessionID); err != nil {
		return err
	}
	sess, err := rt.Sessions.UpdateSessionModel(ctx, sessionID, modelID, providerID)
	if err != nil {
		return err
	}
	fmt.Printf("Session %s now uses %s (%s)\n", sess.ID, sess.ModelID, sess.ProviderID)
	return nil
}

func listAllSessions(ctx context.Context, rt *runtime.Runtime) ([]session.Session, map[string]string, error) {
	workspaces, err := rt.Sessions.ListWorkspaces(ctx)
	if err != nil {
		return nil, nil, err
	}
	pathByID := make(map[string]string, len(workspaces))
	for _, ws := range workspaces {
		pathByID[ws.ID] = ws.Path
	}
	list, err := rt.Sessions.ListAllSessions(ctx)
	return list, pathByID, err
}

func listByWorkspaceID(ctx context.Context, rt *runtime.Runtime, id string) ([]session.Session, map[string]string, error) {
	ws, err := rt.Sessions.GetWorkspace(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	list, err := rt.Sessions.ListSessions(ctx, ws.ID)
	return list, map[string]string{ws.ID: ws.Path}, err
}

func listByWorkspacePath(ctx context.Context, rt *runtime.Runtime) ([]session.Session, map[string]string, error) {
	root, err := WorkspaceRoot()
	if err != nil {
		return nil, nil, err
	}
	ws, err := rt.Sessions.LookupWorkspaceByPath(ctx, root)
	if errors.Is(err, session.ErrWorkspaceNotFound) {
		return nil, nil, fmt.Errorf("workspace not registered at %s — run `cometmind init`", root)
	}
	if err != nil {
		return nil, nil, err
	}
	list, err := rt.Sessions.ListSessions(ctx, ws.ID)
	return list, map[string]string{ws.ID: ws.Path}, err
}

type sessionListResponse struct {
	Sessions []session.WireSession `json:"sessions"`
}

func printSessionTable(sessions []session.WireSession, includeWorkspace bool) error {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if includeWorkspace {
		fmt.Fprintln(tw, "ID\tWORKSPACE\tTITLE\tPROVIDER\tMODEL\tSTATUS\tPIN\tUPDATED")
		for _, s := range sessions {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
				s.ID, s.WorkspacePath, s.Title, s.ProviderID, s.ModelID, s.Status, pinLabel(s.Pinned), s.UpdatedAt)
		}
	} else {
		fmt.Fprintln(tw, "ID\tTITLE\tPROVIDER\tMODEL\tSTATUS\tPIN\tUPDATED")
		for _, s := range sessions {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
				s.ID, s.Title, s.ProviderID, s.ModelID, s.Status, pinLabel(s.Pinned), s.UpdatedAt)
		}
	}
	return tw.Flush()
}

func pinLabel(pinned bool) string {
	if pinned {
		return "1"
	}
	return "0"
}
