package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

const MaxCommitMessageBytes = 16 * 1024

// MutationResult is a simple success payload for write operations.
type MutationResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// CommitResult is returned after a successful commit.
type CommitResult struct {
	OK      bool   `json:"ok"`
	SHA     string `json:"sha,omitempty"`
	Message string `json:"message,omitempty"`
}

// Stage adds paths to the index (git add).
func Stage(ctx context.Context, workspace string, relPaths []string) (MutationResult, error) {
	return mutatePaths(ctx, workspace, relPaths, func(ctx context.Context, workspace string, gitPaths []string) error {
		args := append([]string{"add", "--"}, gitPaths...)
		_, err := runGit(ctx, workspace, args...)
		return err
	})
}

// Unstage removes paths from the index (git restore --staged).
func Unstage(ctx context.Context, workspace string, relPaths []string) (MutationResult, error) {
	return mutatePaths(ctx, workspace, relPaths, func(ctx context.Context, workspace string, gitPaths []string) error {
		args := append([]string{"restore", "--staged", "--"}, gitPaths...)
		_, err := runGit(ctx, workspace, args...)
		return err
	})
}

// Discard reverts local changes for paths.
//
// - Untracked files are deleted (directories are rejected).
// - Tracked paths are restored to HEAD in both the index and worktree.
func Discard(ctx context.Context, workspace string, relPaths []string) (MutationResult, error) {
	workspace = filepath.Clean(workspace)
	if err := ensureDir(workspace); err != nil {
		return MutationResult{}, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return MutationResult{}, fmt.Errorf("git is not installed or not on PATH")
	}
	_, relPrefix, err := resolveGitRoot(ctx, workspace)
	if err != nil {
		if errorsIsNotRepo(err) {
			return MutationResult{}, fmt.Errorf("not a git repository")
		}
		return MutationResult{}, err
	}

	relPaths, err = normalizeRelPaths(relPaths)
	if err != nil {
		return MutationResult{}, err
	}

	var tracked []string
	for _, rel := range relPaths {
		if _, err := sandbox.ResolveWorkspacePath(workspace, rel); err != nil {
			return MutationResult{}, err
		}
		gitPath := toGitPath(rel, relPrefix)
		statusOut, err := runGit(ctx, workspace, "status", "--porcelain=v1", "-uall", "--", gitPath)
		if err != nil {
			return MutationResult{}, err
		}
		line := strings.TrimSpace(string(statusOut))
		if line == "" {
			// Nothing to discard.
			continue
		}
		if strings.HasPrefix(line, "??") {
			abs, err := sandbox.ResolveWorkspacePath(workspace, rel)
			if err != nil {
				return MutationResult{}, err
			}
			info, err := os.Lstat(abs)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return MutationResult{}, err
			}
			if info.IsDir() {
				return MutationResult{}, fmt.Errorf("refusing to discard untracked directory: %s", rel)
			}
			if err := os.Remove(abs); err != nil {
				return MutationResult{}, fmt.Errorf("remove untracked file %s: %w", rel, err)
			}
			continue
		}
		tracked = append(tracked, gitPath)
	}

	if len(tracked) > 0 {
		args := append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, tracked...)
		if _, err := runGit(ctx, workspace, args...); err != nil {
			return MutationResult{}, err
		}
	}
	return MutationResult{OK: true}, nil
}

// Commit creates a commit from the current index with the given message.
// Does not auto-stage; only staged changes are committed.
func Commit(ctx context.Context, workspace, message string) (CommitResult, error) {
	workspace = filepath.Clean(workspace)
	if err := ensureDir(workspace); err != nil {
		return CommitResult{}, err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return CommitResult{}, fmt.Errorf("commit message is required")
	}
	if len(message) > MaxCommitMessageBytes {
		return CommitResult{}, fmt.Errorf("commit message exceeds %d bytes", MaxCommitMessageBytes)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return CommitResult{}, fmt.Errorf("git is not installed or not on PATH")
	}
	if _, _, err := resolveGitRoot(ctx, workspace); err != nil {
		if errorsIsNotRepo(err) {
			return CommitResult{}, fmt.Errorf("not a git repository")
		}
		return CommitResult{}, err
	}

	// Refuse empty commits explicitly (clearer than git's default).
	staged, err := runGit(ctx, workspace, "diff", "--cached", "--name-only")
	if err != nil {
		return CommitResult{}, err
	}
	if len(bytesTrimSpace(staged)) == 0 {
		return CommitResult{}, fmt.Errorf("nothing staged to commit")
	}

	if _, err := runGit(ctx, workspace, "commit", "-m", message); err != nil {
		return CommitResult{}, err
	}
	shaOut, err := runGit(ctx, workspace, "rev-parse", "--short", "HEAD")
	if err != nil {
		return CommitResult{OK: true, Message: "committed"}, nil
	}
	return CommitResult{
		OK:      true,
		SHA:     strings.TrimSpace(string(shaOut)),
		Message: "committed",
	}, nil
}

type pathMutator func(ctx context.Context, workspace string, gitPaths []string) error

func mutatePaths(ctx context.Context, workspace string, relPaths []string, mut pathMutator) (MutationResult, error) {
	workspace = filepath.Clean(workspace)
	if err := ensureDir(workspace); err != nil {
		return MutationResult{}, err
	}
	if _, err := exec.LookPath("git"); err != nil {
		return MutationResult{}, fmt.Errorf("git is not installed or not on PATH")
	}
	_, relPrefix, err := resolveGitRoot(ctx, workspace)
	if err != nil {
		if errorsIsNotRepo(err) {
			return MutationResult{}, fmt.Errorf("not a git repository")
		}
		return MutationResult{}, err
	}

	relPaths, err = normalizeRelPaths(relPaths)
	if err != nil {
		return MutationResult{}, err
	}

	gitPaths := make([]string, 0, len(relPaths))
	for _, rel := range relPaths {
		if _, err := sandbox.ResolveWorkspacePath(workspace, rel); err != nil {
			return MutationResult{}, err
		}
		gitPaths = append(gitPaths, toGitPath(rel, relPrefix))
	}

	if err := mut(ctx, workspace, gitPaths); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{OK: true}, nil
}

func normalizeRelPaths(relPaths []string) ([]string, error) {
	if len(relPaths) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}
	if len(relPaths) > MaxFileList {
		return nil, fmt.Errorf("too many paths (max %d)", MaxFileList)
	}
	out := make([]string, 0, len(relPaths))
	seen := map[string]bool{}
	for _, raw := range relPaths {
		rel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(raw)))
		if rel == "" || rel == "." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("invalid path: %q", raw)
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one path is required")
	}
	return out, nil
}

func toGitPath(rel, relPrefix string) string {
	if relPrefix == "" {
		return rel
	}
	return filepath.ToSlash(filepath.Join(relPrefix, rel))
}

func errorsIsNotRepo(err error) bool {
	return errors.Is(err, errNotARepo) ||
		(err != nil && strings.Contains(strings.ToLower(err.Error()), "not a git repository"))
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
