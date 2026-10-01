package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/runtime"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Inspect and configure default models",
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List enabled models across configured providers",
	RunE:  runModelList,
}

var modelSetCmd = &cobra.Command{
	Use:   "set <provider-id> <model-id>",
	Short: "Set the default model in Cometline settings",
	Args:  cobra.ExactArgs(2),
	RunE:  runModelSet,
}

func init() {
	modelCmd.AddCommand(modelListCmd, modelSetCmd)
	rootCmd.AddCommand(modelCmd)
}

func runModelList(_ *cobra.Command, _ []string) error {
	models, err := config.ListConfiguredModels()
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tMODEL ID\tNAME")
	for _, m := range models {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.ProviderID, m.ModelID, m.Name)
	}
	return tw.Flush()
}

func runModelSet(_ *cobra.Command, args []string) error {
	providerID := strings.TrimSpace(args[0])
	modelID := strings.TrimSpace(args[1])
	if err := config.UpdateDefaultModel(providerID, modelID); err != nil {
		return err
	}
	fmt.Printf("Default model set to %s (%s)\n", modelID, providerID)
	return nil
}

func openRuntime(ctx context.Context) (*runtime.Runtime, error) {
	rt, err := runtime.New(ctx)
	if err != nil {
		return nil, err
	}
	return rt, nil
}

func closeRuntime(rt *runtime.Runtime) {
	if rt != nil {
		rt.Close()
	}
}

func requireSession(ctx context.Context, rt *runtime.Runtime, sessionID string) error {
	if _, err := rt.Sessions.GetSession(ctx, sessionID); err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	return nil
}
