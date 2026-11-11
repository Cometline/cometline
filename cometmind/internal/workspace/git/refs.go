package git

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Checkout switches HEAD to a local branch or commit.
// A dirty worktree is refused so the panel cannot drop uncommitted work.
func Checkout(ctx context.Context, workspace, ref string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return MutationResult{}, fmt.Errorf("ref is required")
	}
	if !isSafeRevision(ref) {
		return MutationResult{}, fmt.Errorf("invalid ref: %q", ref)
	}
	dirty, err := runGit(ctx, workspace, "status", "--porcelain")
	if err != nil {
		return MutationResult{}, err
	}
	if strings.TrimSpace(string(dirty)) != "" {
		return MutationResult{}, fmt.Errorf("worktree is dirty; commit or discard changes before checkout")
	}
	if _, err := runGit(ctx, workspace, "checkout", ref); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "checked out " + ref}, nil
}

// CreateBranch creates a local branch. start defaults to HEAD.
func CreateBranch(ctx context.Context, workspace, name, start string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	if err := validateBranchName(ctx, workspace, name); err != nil {
		return MutationResult{}, err
	}
	start = strings.TrimSpace(start)
	args := []string{"branch", name}
	if start != "" {
		if !isSafeRevision(start) {
			return MutationResult{}, fmt.Errorf("invalid start point: %q", start)
		}
		args = append(args, start)
	}
	if _, err := runGit(ctx, workspace, args...); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "created branch " + name}, nil
}

// RenameBranch renames a local branch, including the current one.
func RenameBranch(ctx context.Context, workspace, oldName, newName string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	if err := requireLocalBranch(oldName); err != nil {
		return MutationResult{}, err
	}
	if err := validateBranchName(ctx, workspace, newName); err != nil {
		return MutationResult{}, err
	}
	if _, err := runGit(ctx, workspace, "show-ref", "--verify", "--quiet", "refs/heads/"+oldName); err != nil {
		return MutationResult{}, fmt.Errorf("branch %q does not exist", oldName)
	}
	if _, err := runGit(ctx, workspace, "branch", "-m", oldName, newName); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "renamed branch to " + newName}, nil
}

// DeleteBranch deletes a local branch that is fully merged.
// The current branch and HEAD are refused. This never uses -D.
func DeleteBranch(ctx context.Context, workspace, name string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	if err := requireLocalBranch(name); err != nil {
		return MutationResult{}, err
	}
	_, headBranch := headIdentity(ctx, workspace)
	if name == headBranch {
		return MutationResult{}, fmt.Errorf("refusing to delete the current branch %q", name)
	}
	if _, err := runGit(ctx, workspace, "branch", "-d", name); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "deleted branch " + name}, nil
}

// CreateTag creates a lightweight or annotated tag. sha defaults to HEAD.
// When message is non-empty, an annotated tag is created.
func CreateTag(ctx context.Context, workspace, name, sha, message string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	if err := validateTagName(ctx, workspace, name); err != nil {
		return MutationResult{}, err
	}
	sha = strings.TrimSpace(sha)
	if sha != "" && !isSafeRevision(sha) {
		return MutationResult{}, fmt.Errorf("invalid commit: %q", sha)
	}
	message = strings.TrimSpace(message)
	var args []string
	if message != "" {
		args = []string{"tag", "-a", name, "-m", message}
		if sha != "" {
			args = append(args, sha)
		}
	} else {
		args = []string{"tag", name}
		if sha != "" {
			args = append(args, sha)
		}
	}
	if _, err := runGit(ctx, workspace, args...); err != nil {
		return MutationResult{}, err
	}
	kind := "lightweight tag"
	if message != "" {
		kind = "annotated tag"
	}
	return MutationResult{OK: true, Message: fmt.Sprintf("created %s %s", kind, name)}, nil
}

// RenameTag creates the new tag on the old target, then deletes the old name.
// If delete fails, the new tag is removed so both names are not left behind.
func RenameTag(ctx context.Context, workspace, oldName, newName string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	if err := requireTagName(oldName); err != nil {
		return MutationResult{}, err
	}
	if err := validateTagName(ctx, workspace, newName); err != nil {
		return MutationResult{}, err
	}
	if _, err := runGit(ctx, workspace, "rev-parse", "--verify", "--quiet", "refs/tags/"+oldName); err != nil {
		return MutationResult{}, fmt.Errorf("tag %q does not exist", oldName)
	}
	if _, err := runGit(ctx, workspace, "tag", newName, "refs/tags/"+oldName); err != nil {
		return MutationResult{}, err
	}
	if _, err := runGit(ctx, workspace, "tag", "-d", oldName); err != nil {
		_, _ = runGit(ctx, workspace, "tag", "-d", newName)
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "renamed tag to " + newName}, nil
}

// DeleteTag deletes a local tag.
func DeleteTag(ctx context.Context, workspace, name string) (MutationResult, error) {
	workspace, err := requireRepo(ctx, workspace)
	if err != nil {
		return MutationResult{}, err
	}
	if err := requireTagName(name); err != nil {
		return MutationResult{}, err
	}
	if _, err := runGit(ctx, workspace, "tag", "-d", name); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true, Message: "deleted tag " + name}, nil
}

func requireRepo(ctx context.Context, workspace string) (string, error) {
	workspace = filepath.Clean(workspace)
	if err := ensureDir(workspace); err != nil {
		return "", err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return "", fmt.Errorf("git is not installed or not on PATH")
	}
	if _, err := resolveGitRoot(ctx, workspace); err != nil {
		if errorsIsNotRepo(err) {
			return "", fmt.Errorf("not a git repository")
		}
		return "", err
	}
	return workspace, nil
}

func requireLocalBranch(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "HEAD") {
		return fmt.Errorf("invalid branch name: %q", name)
	}
	if strings.Contains(name, "refs/") || strings.HasPrefix(name, "refs") {
		return fmt.Errorf("invalid branch name: %q", name)
	}
	return nil
}

func requireTagName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "HEAD") {
		return fmt.Errorf("invalid tag name: %q", name)
	}
	if strings.Contains(name, "refs/") {
		return fmt.Errorf("invalid tag name: %q", name)
	}
	return nil
}

func validateBranchName(ctx context.Context, workspace, name string) error {
	if err := requireLocalBranch(name); err != nil {
		return err
	}
	if _, err := runGit(ctx, workspace, "check-ref-format", "--branch", name); err != nil {
		return fmt.Errorf("invalid branch name: %q", name)
	}
	return nil
}

func validateTagName(ctx context.Context, workspace, name string) error {
	if err := requireTagName(name); err != nil {
		return err
	}
	if _, err := runGit(ctx, workspace, "check-ref-format", "refs/tags/"+name); err != nil {
		return fmt.Errorf("invalid tag name: %q", name)
	}
	return nil
}

func isSafeRevision(ref string) bool {
	if ref == "" || len(ref) > 200 {
		return false
	}
	if strings.ContainsAny(ref, " \t\n\r~^:?*[\\") {
		return false
	}
	if strings.HasPrefix(ref, "-") || strings.Contains(ref, "..") {
		return false
	}
	return true
}
