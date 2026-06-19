package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/runtime"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List and inspect local Agent Skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered skills",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()
		rt, reg, err := skillsRegistryForCommand(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()
		for _, skill := range reg.Skills {
			fmt.Printf("%s\t%s\n", skill.Name, skill.Description)
		}
		for _, msg := range reg.Errors {
			fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
		}
		return nil
	},
}

var skillsShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Print one skill's SKILL.md",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		rt, reg, err := skillsRegistryForCommand(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()
		_, markdown, err := reg.SkillMarkdown(args[0])
		if err != nil {
			return err
		}
		fmt.Println(strings.TrimRight(markdown, "\n"))
		return nil
	},
}

var skillsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Symlink discovered skills into ~/.cometmind/skills",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()
		rt, reg, err := skillsRegistryForCommand(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()
		created, skipped, err := reg.SyncMirror(filepath.Join("~", ".cometmind", "skills"))
		if err != nil {
			return err
		}
		fmt.Printf("created: %d\nskipped: %d\n", len(created), len(skipped))
		if len(created) > 0 {
			fmt.Printf("created_names: %s\n", strings.Join(created, ", "))
		}
		if len(skipped) > 0 {
			fmt.Printf("skipped_names: %s\n", strings.Join(skipped, ", "))
		}
		return nil
	},
}

var skillsDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a managed skill from ~/.cometmind/skills",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		rt, reg, err := skillsRegistryForCommand(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()
		skill, ok := reg.Find(args[0])
		if !ok {
			return fmt.Errorf("unknown skill: %s", args[0])
		}
		if err := skills.DeleteManagedSkill(skill); err != nil {
			return err
		}
		fmt.Printf("deleted %s\n", skill.Name)
		return nil
	},
}

var skillsExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a skill directory as zip bytes to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx := context.Background()
		rt, reg, err := skillsRegistryForCommand(ctx)
		if err != nil {
			return err
		}
		defer rt.Close()
		skill, ok := reg.Find(args[0])
		if !ok {
			return fmt.Errorf("unknown skill: %s", args[0])
		}
		data, err := skills.ExportSkill(skill)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd, skillsShowCmd, skillsSyncCmd, skillsDeleteCmd, skillsExportCmd)
	rootCmd.AddCommand(skillsCmd)
}

func skillsRegistryForCommand(ctx context.Context) (*runtime.Runtime, skills.Registry, error) {
	rt, err := runtime.New(ctx)
	if err != nil {
		return nil, skills.Registry{}, err
	}
	root, err := WorkspaceRoot()
	if err != nil {
		rt.Close()
		return nil, skills.Registry{}, err
	}
	return rt, rt.SkillsForWorkspace(root), nil
}
