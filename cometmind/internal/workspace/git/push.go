package git

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const pushTimeout = 120 * time.Second

// PushBranch pushes a local branch to a remote. Never force-pushes.
// When the branch has no upstream, -u is used to set it on the chosen remote.
func PushBranch(ctx context.Context, workspace, remote, branch string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	remotes, err := ListRemotes(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	remote, err = resolveRemote(remotes, remote)
	if err != nil {
		return MutationResult{}, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch, err = currentBranch(ctx, workspace)
		if err != nil {
			return MutationResult{}, err
		}
	}
	if err := requireLocalBranch(branch); err != nil {
		return MutationResult{}, err
	}
	hasUpstream, err := branchHasUpstream(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	args := []string{"push"}
	if !hasUpstream {
		args = append(args, "-u", remote, branch)
	} else {
		args = append(args, remote, branch)
	}
	ctx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	if _, err := runGit(ctx, workspace, args...); err != nil {
		if isPushRejected(err) {
			return MutationResult{}, fmt.Errorf(
				"push rejected: the remote branch is ahead; fetch or pull first, then push again",
			)
		}
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: fmt.Sprintf("pushed %s to %s", branch, remote)}, nil
}

// PushTags pushes one tag or all local tags to a remote. Never force-pushes.
func PushTags(ctx context.Context, workspace, remote, tag string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	remotes, err := ListRemotes(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	remote, err = resolveRemote(remotes, remote)
	if err != nil {
		return MutationResult{}, err
	}
	tag = strings.TrimSpace(tag)
	if tag != "" {
		if err := requireTagName(tag); err != nil {
			return MutationResult{}, err
		}
		if _, err := runGit(ctx, workspace, "rev-parse", "--verify", "--quiet", "refs/tags/"+tag); err != nil {
			return MutationResult{}, fmt.Errorf("tag %q does not exist", tag)
		}
	}
	args := []string{"push", remote}
	if tag == "" {
		args = append(args, "--tags")
	} else {
		args = append(args, tag)
	}
	ctx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()
	if _, err := runGit(ctx, workspace, args...); err != nil {
		if isPushRejected(err) {
			return MutationResult{}, fmt.Errorf(
				"push rejected: the remote ref is ahead; fetch first, then push again",
			)
		}
		return MutationResult{}, err
	}
	msg := "pushed all tags to " + remote
	if tag != "" {
		msg = fmt.Sprintf("pushed tag %s to %s", tag, remote)
	}
	return MutationResult{OK: true, Message: msg}, nil
}

func currentBranch(ctx context.Context, workspace string) (string, error) {
	out, err := runGit(ctx, workspace, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("cannot push from detached HEAD; check out a branch first")
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || strings.EqualFold(branch, "HEAD") {
		return "", fmt.Errorf("cannot push from detached HEAD; check out a branch first")
	}
	return branch, nil
}

func branchHasUpstream(ctx context.Context, workspace string) (bool, error) {
	_, err := runGit(ctx, workspace, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no upstream") || strings.Contains(msg, "unknown revision") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isPushRejected(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rejected") ||
		strings.Contains(msg, "non-fast-forward") ||
		strings.Contains(msg, "fetch first") ||
		strings.Contains(msg, "stale info")
}
