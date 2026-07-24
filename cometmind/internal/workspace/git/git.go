// Package git provides read-only workspace-scoped git status and diff helpers.
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
	"time"

	"github.com/cometline/cometmind/internal/tools/sandbox"
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

	_, relPrefix, err := resolveGitRoot(ctx, workspace)
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

// Diff returns the unified diff for a single workspace-relative path.
func Diff(ctx context.Context, workspace, relPath string, scope Scope) (DiffResult, error) {
	workspace = filepath.Clean(workspace)
	if err := ensureDir(workspace); err != nil {
		return DiffResult{}, err
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "../") || filepath.IsAbs(relPath) {
		return DiffResult{}, fmt.Errorf("invalid path")
	}

	if _, err := sandbox.ResolveWorkspacePath(workspace, relPath); err != nil {
		return DiffResult{}, err
	}

	if _, err := exec.LookPath("git"); err != nil {
		return DiffResult{Path: relPath, Message: "git is not installed or not on PATH"}, nil
	}

	_, relPrefix, err := resolveGitRoot(ctx, workspace)
	if err != nil {
		if errors.Is(err, errNotARepo) {
			return DiffResult{Path: relPath, Message: "This workspace is not a git repository."}, nil
		}
		return DiffResult{}, err
	}

	// Path as git sees it (relative to toplevel).
	gitPath := relPath
	if relPrefix != "" {
		gitPath = filepath.ToSlash(filepath.Join(relPrefix, relPath))
	}

	// Detect untracked for this path via status.
	statusOut, err := runGit(ctx, workspace, "status", "--porcelain=v1", "-uall", "--", gitPath)
	if err != nil {
		return DiffResult{}, err
	}
	statusLine := strings.TrimSpace(string(statusOut))
	untracked := strings.HasPrefix(statusLine, "??")

	if untracked {
		if scope == ScopeStaged {
			return DiffResult{Path: relPath, Empty: true, Message: "Untracked file is not staged."}, nil
		}
		return untrackedDiff(ctx, workspace, relPath, gitPath)
	}

	var parts []string
	switch scope {
	case ScopeStaged:
		parts = append(parts, diffCached(ctx, workspace, gitPath)...)
	case ScopeWorking:
		parts = append(parts, diffUnstaged(ctx, workspace, gitPath)...)
	case ScopeAll:
		parts = append(parts, diffCached(ctx, workspace, gitPath)...)
		parts = append(parts, diffUnstaged(ctx, workspace, gitPath)...)
	}

	body := strings.TrimSpace(strings.Join(parts, "\n"))
	if body == "" {
		return DiffResult{Path: relPath, Empty: true, Message: "No diff for this path in the selected scope."}, nil
	}

	if looksBinary(body) {
		return DiffResult{Path: relPath, Binary: true, Message: "Binary file; diff not shown."}, nil
	}

	truncated := false
	if len(body) > MaxDiffBytes {
		body = body[:MaxDiffBytes]
		truncated = true
	}

	return DiffResult{Path: relPath, Diff: body, Truncated: truncated}, nil
}

func diffCached(ctx context.Context, workspace, gitPath string) []string {
	out, err := runGit(ctx, workspace, "diff", "--cached", "--", gitPath)
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return nil
	}
	return []string{string(out)}
}

func diffUnstaged(ctx context.Context, workspace, gitPath string) []string {
	out, err := runGit(ctx, workspace, "diff", "--", gitPath)
	if err != nil || len(bytes.TrimSpace(out)) == 0 {
		return nil
	}
	return []string{string(out)}
}

func untrackedDiff(ctx context.Context, workspace, relPath, gitPath string) (DiffResult, error) {
	abs, err := sandbox.ResolveWorkspacePath(workspace, relPath)
	if err != nil {
		return DiffResult{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return DiffResult{}, err
	}
	if info.IsDir() {
		return DiffResult{Path: relPath, Message: "Untracked directory; open a file to see a diff."}, nil
	}
	if info.Size() > MaxUntrackedBytes {
		return DiffResult{
			Path:      relPath,
			Truncated: true,
			Message:   fmt.Sprintf("Untracked file is larger than %d bytes; diff not shown.", MaxUntrackedBytes),
		}, nil
	}

	// git diff --no-index exits 1 when files differ; treat that as success.
	out, err := runGitAllowExit1(ctx, workspace, "diff", "--no-index", "--", "/dev/null", gitPath)
	if err != nil {
		// Fallback: synthesize a minimal new-file unified diff.
		data, readErr := os.ReadFile(abs)
		if readErr != nil {
			return DiffResult{}, readErr
		}
		if isBinaryBytes(data) {
			return DiffResult{Path: relPath, Binary: true, Message: "Binary file; diff not shown."}, nil
		}
		body := synthesizeNewFileDiff(relPath, string(data))
		truncated := false
		if len(body) > MaxDiffBytes {
			body = body[:MaxDiffBytes]
			truncated = true
		}
		return DiffResult{Path: relPath, Diff: body, Truncated: truncated}, nil
	}
	body := string(out)
	if looksBinary(body) {
		return DiffResult{Path: relPath, Binary: true, Message: "Binary file; diff not shown."}, nil
	}
	truncated := false
	if len(body) > MaxDiffBytes {
		body = body[:MaxDiffBytes]
		truncated = true
	}
	return DiffResult{Path: relPath, Diff: body, Truncated: truncated}, nil
}

func synthesizeNewFileDiff(path, content string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
	fmt.Fprintf(&b, "new file mode 100644\n")
	fmt.Fprintf(&b, "--- /dev/null\n")
	fmt.Fprintf(&b, "+++ b/%s\n", path)
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	// Drop trailing empty from final newline split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		b.WriteByte('+')
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

var errNotARepo = errors.New("not a git repository")

// resolveGitRoot returns (toplevel, pathPrefixFromToplevelToWorkspace, nil).
// pathPrefix is empty when workspace is the git root; otherwise paths from git
// are filtered/rewritten to be relative to workspace.
func resolveGitRoot(ctx context.Context, workspace string) (toplevel, relPrefix string, err error) {
	out, err := runGit(ctx, workspace, "rev-parse", "--show-toplevel")
	if err != nil {
		// rev-parse fails outside a work tree
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not a git repository") || strings.Contains(msg, "exit status") {
			return "", "", errNotARepo
		}
		return "", "", err
	}
	toplevel = filepath.Clean(strings.TrimSpace(string(out)))
	if toplevel == "" {
		return "", "", errNotARepo
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
		return toplevel, "", nil
	}
	rel, err := filepath.Rel(top, ws)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		// Workspace is not inside the git root (unusual); treat as not a repo for safety.
		return "", "", errNotARepo
	}
	return toplevel, filepath.ToSlash(rel), nil
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

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
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
	cmd := exec.CommandContext(ctx, "git", args...)
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

func looksBinary(diff string) bool {
	return strings.Contains(diff, "Binary files ") || strings.Contains(diff, "GIT binary patch")
}

func isBinaryBytes(data []byte) bool {
	// NUL in first 8KB → treat as binary.
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}
