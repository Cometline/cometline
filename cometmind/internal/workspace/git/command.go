package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/process"
)

var errNotARepo = errors.New("not a git repository")

// resolveGitRoot returns the path prefix from the git toplevel to workspace.
// The prefix is empty when workspace is the git root; otherwise paths from git
// are filtered/rewritten to be relative to workspace.
func resolveGitRoot(ctx context.Context, workspace string) (relPrefix string, err error) {
	out, err := runGit(ctx, workspace, "rev-parse", "--show-toplevel")
	if err != nil {
		// rev-parse fails outside a work tree
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not a git repository") || strings.Contains(msg, "exit status") {
			return "", errNotARepo
		}
		return "", err
	}
	toplevel := filepath.Clean(strings.TrimSpace(string(out)))
	if toplevel == "" {
		return "", errNotARepo
	}

	ws, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		ws = workspace
	}
	top, err := filepath.EvalSymlinks(toplevel)
	if err != nil {
		top = toplevel
	}
	ws = filepath.Clean(ws)
	top = filepath.Clean(top)

	if ws == top {
		return "", nil
	}
	rel, err := filepath.Rel(top, ws)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		// Workspace is not inside the git root (unusual); treat as not a repo for safety.
		return "", errNotARepo
	}
	return filepath.ToSlash(rel), nil
}

func branchInfo(ctx context.Context, workspace string) (branch, upstream string) {
	out, err := runGit(ctx, workspace, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		branch = strings.TrimSpace(string(out))
		if branch == "HEAD" {
			// Detached HEAD — show short SHA.
			if sha, err := runGit(ctx, workspace, "rev-parse", "--short", "HEAD"); err == nil {
				branch = "detached@" + strings.TrimSpace(string(sha))
			}
		}
	}
	if out, err := runGit(ctx, workspace, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		upstream = strings.TrimSpace(string(out))
	}
	return branch, upstream
}

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	cmd := process.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// Avoid interactive prompts and locale-dependent noise.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("git %s: timeout", strings.Join(args, " "))
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// runGitAllowExit1 is like runGit but treats exit status 1 as success (git diff --no-index).
func runGitAllowExit1(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	cmd := process.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("git %s: timeout", strings.Join(args, " "))
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return stdout.Bytes(), nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", path)
	}
	return nil
}
