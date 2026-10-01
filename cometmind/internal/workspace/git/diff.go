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

	"github.com/Cometline/cometline/cometmind/internal/tools/sandbox"
)

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

	relPrefix, err := resolveGitRoot(ctx, workspace)
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
