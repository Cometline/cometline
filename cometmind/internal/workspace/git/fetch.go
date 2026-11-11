package git

import (
	"context"
	"time"
)

const fetchTimeout = 120 * time.Second

// Fetch runs git fetch --all --prune for the workspace repository.
func Fetch(ctx context.Context, workspace string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	if _, err := runGit(ctx, workspace, "fetch", "--all", "--prune"); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "fetched remotes"}, nil
}
