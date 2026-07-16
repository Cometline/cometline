package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cometline/cometmind/internal/process"
)

// Grep searches file contents under the workspace.
type Grep struct{ Workspace Workspace }

type grepInput struct {
	Pattern     *string `json:"pattern"`
	Path        *string `json:"path,omitempty"`
	Include     *string `json:"include,omitempty"`
	LiteralText *bool   `json:"literal_text,omitempty"`
}

// rgPath is set at init; tests may override to force the native fallback.
var rgPath, _ = process.ResolveCommand("rg")

func (Grep) Spec() ToolSpec {
	return ToolSpec{
		Name: "grep",
		Description: "Search file contents for a pattern. Uses ripgrep syntax when available " +
			"(not POSIX grep). Respects .gitignore. Results include line numbers. The path may be @runtime/wiki.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {"type": "string", "description": "Search pattern (ripgrep regex syntax)"},
				"path": {"type": "string", "description": "Optional relative subdirectory to scope the search (default workspace root)"},
				"include": {"type": "string", "description": "Optional file glob filter, e.g. **/*.tsx"},
				"literal_text": {"type": "boolean", "description": "Treat pattern as literal text, not regex"}
			},
			"required": ["pattern"]
		}`),
	}
}

func (g Grep) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	pattern, bad, ok := requiredTrimmedString(in.Pattern, "pattern")
	if !ok {
		return bad, nil
	}

	searchRel := "."
	if in.Path != nil {
		trimmed := strings.TrimSpace(*in.Path)
		if trimmed != "" {
			searchRel = trimmed
		}
	}

	searchRoot, displayPrefix, err := g.Workspace.ResolveSearchRoot(searchRel)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	workspaceRoot, err := g.Workspace.Resolve(".")
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if displayPrefix != "" {
		workspaceRoot = searchRoot
		searchRel = "."
	}

	include := ""
	if in.Include != nil {
		include = strings.TrimSpace(*in.Include)
	}
	literal := in.LiteralText != nil && *in.LiteralText

	if rgPath != "" {
		return grepRipgrep(ctx, workspaceRoot, searchRel, pattern, include, literal, displayPrefix)
	}
	return grepNative(ctx, workspaceRoot, searchRoot, pattern, include, literal, displayPrefix)
}

func grepRipgrep(ctx context.Context, workspaceRoot, searchRel, pattern, include string, literal bool, displayPrefix string) (Result, error) {
	args := []string{"-n", "--no-heading", "--color=never"}
	if literal {
		args = append(args, "-F")
	}
	if include != "" {
		args = append(args, "--glob", include)
	}
	args = append(args, pattern)

	rgTarget := "."
	if searchRel != "." {
		rgTarget = filepath.ToSlash(searchRel)
	}
	args = append(args, rgTarget)

	cmdCtx, cancel := context.WithTimeout(ctx, grepTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, rgPath, args...) //nolint:gosec
	cmd.Dir = workspaceRoot
	cmd.Env = process.Env()

	out, err := cmd.CombinedOutput()
	text := strings.TrimRight(string(out), "\n")
	text = normalizeRipgrepOutput(text)
	text = filterGrepOutput(workspaceRoot, text)
	text = prefixGrepOutput(text, displayPrefix)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return Result{OK: true, Output: formatSearchFooter(0, false, "matches")}, nil
		}
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return Result{OK: false, Output: "grep timed out after " + grepTimeout.String()}, nil
		}
		if text == "" {
			return Result{OK: false, Output: err.Error()}, nil
		}
	}

	matchCount := countGrepLines(text)
	truncated := false
	if len([]rune(text)) > grepMaxOutputChars {
		text, truncated = truncateOutput(text, grepMaxOutputChars)
		matchCount = countGrepLines(text)
	}

	outStr := text + formatSearchFooter(matchCount, truncated, "matches")
	return Result{OK: true, Output: outStr}, nil
}

func grepNative(ctx context.Context, workspaceRoot, searchRoot, pattern, include string, literal bool, displayPrefix string) (Result, error) {
	var matcher func(line string) bool
	if literal {
		matcher = func(line string) bool { return strings.Contains(line, pattern) }
	} else {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return Result{OK: false, Output: "invalid regex pattern: " + err.Error()}, nil
		}
		matcher = re.MatchString
	}

	var lines []string
	err := walkSearchableFiles(ctx, workspaceRoot, searchRoot, func(workspaceRel, absPath string) error {
		if include != "" {
			matched, err := doublestar.PathMatch(include, workspaceRel)
			if err != nil || !matched {
				return nil
			}
		}

		f, err := os.Open(absPath)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lineNum++
			line := scanner.Text()
			if !matcher(line) {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s:%d: %s", searchDisplayPath(displayPrefix, workspaceRel), lineNum, line))
			combined := strings.Join(lines, "\n")
			if len([]rune(combined)) > grepMaxOutputChars {
				return fs.SkipAll
			}
		}
		return scanner.Err()
	})
	if err != nil && !errors.Is(err, fs.SkipAll) && ctx.Err() == nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	text := strings.Join(lines, "\n")
	truncated := false
	if len([]rune(text)) > grepMaxOutputChars {
		text, truncated = truncateOutput(text, grepMaxOutputChars)
	} else if errors.Is(err, fs.SkipAll) {
		truncated = true
	}

	matchCount := countGrepLines(text)
	out := text + formatSearchFooter(matchCount, truncated, "matches")
	return Result{OK: true, Output: out}, nil
}

func prefixGrepOutput(text, prefix string) string {
	if text == "" || prefix == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if _, ok := grepLinePath(line); ok {
			lines[i] = searchDisplayPath(prefix, line)
		}
	}
	return strings.Join(lines, "\n")
}

func countGrepLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func normalizeRipgrepOutput(text string) string {
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "./") {
			lines[i] = line[2:]
		}
	}
	return strings.Join(lines, "\n")
}

func filterGrepOutput(workspaceRoot, text string) string {
	if text == "" {
		return text
	}
	ignorer := loadGitignore(workspaceRoot)
	lines := strings.Split(text, "\n")
	filtered := lines[:0]
	for _, line := range lines {
		path, ok := grepLinePath(line)
		if !ok {
			filtered = append(filtered, line)
			continue
		}
		if ignorer != nil && ignorer.MatchesPath(path) {
			continue
		}
		if grepPathHasSkippedDir(path) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func grepLinePath(line string) (string, bool) {
	first := strings.IndexByte(line, ':')
	if first < 0 {
		return "", false
	}
	rest := line[first+1:]
	second := strings.IndexByte(rest, ':')
	if second < 0 {
		return "", false
	}
	return line[:first], true
}

func grepPathHasSkippedDir(workspaceRel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(workspaceRel), "/") {
		if defaultSkippedDirs[part] {
			return true
		}
	}
	return false
}
