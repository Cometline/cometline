// Package git provides read-only workspace-scoped git status and diff helpers.
package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultTimeout = 10 * time.Second
	MaxFileList    = 500
	// MaxDiffBytes caps a single-file unified diff response body.
	MaxDiffBytes = 400 * 1024
	// MaxUntrackedBytes caps synthesized diffs for untracked files.
	MaxUntrackedBytes = 256 * 1024
)

// Scope selects which change set to report.
type Scope string

const (
	ScopeWorking Scope = "working" // unstaged + untracked (default)
	ScopeStaged  Scope = "staged"
	ScopeAll     Scope = "all" // staged + unstaged + untracked
)

// FileStatus is a workspace-relative changed path.
type FileStatus struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // modified | added | deleted | renamed | untracked | conflict | typechange | unknown
	Staged    bool   `json:"staged"`
	Untracked bool   `json:"untracked"`
	// XY is the raw porcelain v1 XY code (e.g. " M", "M ", "??").
	XY string `json:"xy,omitempty"`
}

// StatusResult is the outcome of Status.
type StatusResult struct {
	IsRepo    bool         `json:"is_repo"`
	Branch    string       `json:"branch,omitempty"`
	Upstream  string       `json:"upstream,omitempty"`
	Files     []FileStatus `json:"files"`
	Truncated bool         `json:"truncated"`
	// Message is a human-readable note when is_repo is false or git is missing.
	Message string `json:"message,omitempty"`
}

// DiffResult is the outcome of Diff.
type DiffResult struct {
	Path      string `json:"path"`
	Binary    bool   `json:"binary"`
	Diff      string `json:"diff,omitempty"`
	Truncated bool   `json:"truncated"`
	// Empty is true when the path has no diff for the requested scope
	// (e.g. staged-only request for a purely unstaged change).
	Empty   bool   `json:"empty,omitempty"`
	Message string `json:"message,omitempty"`
}

// ParseScope maps a query string to Scope; empty becomes working.
func ParseScope(raw string) (Scope, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "working", "worktree", "unstaged":
		return ScopeWorking, nil
	case "staged", "cached", "index":
		return ScopeStaged, nil
	case "all":
		return ScopeAll, nil
	default:
		return "", fmt.Errorf("scope must be working, staged, or all")
	}
}

// Status returns changed files under workspace for the given scope.
// Paths are workspace-relative (filtered when git toplevel is above workspace).
func Status(ctx context.Context, workspace string, scope Scope) (StatusResult, error) {
	workspace = filepath.Clean(workspace)
	if err := ensureDir(workspace); err != nil {
		return StatusResult{}, err
	}

	if _, err := exec.LookPath("git"); err != nil {
		return StatusResult{
			IsRepo:  false,
			Files:   []FileStatus{},
			Message: "git is not installed or not on PATH",
		}, nil
	}

	relPrefix, err := resolveGitRoot(ctx, workspace)
	if err != nil {
		if errors.Is(err, errNotARepo) {
			return StatusResult{
				IsRepo:  false,
				Files:   []FileStatus{},
				Message: "This workspace is not a git repository.",
			}, nil
		}
		return StatusResult{}, err
	}

	branch, upstream := branchInfo(ctx, workspace)

	out, err := runGit(ctx, workspace, "status", "--porcelain=v1", "-uall", "--no-renames")
	if err != nil {
		return StatusResult{}, err
	}

	files := parsePorcelain(string(out), scope, relPrefix)
	truncated := false
	if len(files) > MaxFileList {
		files = files[:MaxFileList]
		truncated = true
	}

	return StatusResult{
		IsRepo:    true,
		Branch:    branch,
		Upstream:  upstream,
		Files:     files,
		Truncated: truncated,
	}, nil
}

func parsePorcelain(output string, scope Scope, relPrefix string) []FileStatus {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	var files []FileStatus
	for _, line := range lines {
		if line == "" {
			continue
		}
		// porcelain v1: XY PATH or XY ORIG -> PATH (renames; we pass --no-renames)
		if len(line) < 3 {
			continue
		}
		xy := line[:2]
		pathPart := strings.TrimSpace(line[2:])
		if pathPart == "" {
			continue
		}
		// Rename form "old -> new" if renames slip through.
		if i := strings.Index(pathPart, " -> "); i >= 0 {
			pathPart = pathPart[i+4:]
		}
		pathPart = filepath.ToSlash(pathPart)

		// Filter to workspace subtree when git root is above workspace.
		wsPath, ok := toWorkspacePath(pathPart, relPrefix)
		if !ok {
			continue
		}

		x := xy[0]
		y := xy[1]
		untracked := xy == "??"
		// Staged when index side (X) is not space or ?
		staged := !untracked && x != ' ' && x != '?'
		// Unstaged when worktree side (Y) is not space or ?
		unstaged := !untracked && y != ' ' && y != '?'

		switch scope {
		case ScopeWorking:
			if !untracked && !unstaged {
				continue
			}
		case ScopeStaged:
			if !staged {
				continue
			}
		case ScopeAll:
			// keep all
		}

		status := classifyStatus(xy, untracked)
		files = append(files, FileStatus{
			Path:      wsPath,
			Status:    status,
			Staged:    staged,
			Untracked: untracked,
			XY:        xy,
		})
	}
	return files
}

func toWorkspacePath(gitPath, relPrefix string) (string, bool) {
	gitPath = strings.TrimPrefix(filepath.ToSlash(gitPath), "./")
	if relPrefix == "" {
		return gitPath, true
	}
	prefix := strings.TrimSuffix(relPrefix, "/") + "/"
	if gitPath == strings.TrimSuffix(relPrefix, "/") {
		// Change is the workspace directory itself; skip.
		return "", false
	}
	if !strings.HasPrefix(gitPath, prefix) {
		return "", false
	}
	return strings.TrimPrefix(gitPath, prefix), true
}

func classifyStatus(xy string, untracked bool) string {
	if untracked {
		return "untracked"
	}
	if xy == "UU" || xy == "AA" || xy == "DD" || xy[0] == 'U' || xy[1] == 'U' {
		return "conflict"
	}
	// Prefer worktree letter, then index.
	letter := xy[1]
	if letter == ' ' {
		letter = xy[0]
	}
	switch letter {
	case 'M':
		return "modified"
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "typechange"
	default:
		return "unknown"
	}
}
