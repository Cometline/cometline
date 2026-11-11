package git

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// ListRemotes returns configured remote names for the workspace repository.
func ListRemotes(ctx context.Context, workspace string) ([]string, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return nil, err
	}
	out, err := runGit(ctx, workspace, "remote")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	remotes := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}

func resolveRemote(remotes []string, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		if !slices.Contains(remotes, requested) {
			return "", fmt.Errorf("remote %q does not exist", requested)
		}
		return requested, nil
	}
	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remotes configured")
	}
	if slices.Contains(remotes, "origin") {
		return "origin", nil
	}
	return remotes[0], nil
}
