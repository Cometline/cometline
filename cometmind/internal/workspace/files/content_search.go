package files

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/Cometline/cometline/cometmind/internal/process"
)

const (
	defaultContentSearchLimit = 100
	maxContentSearchLimit     = 200
	maxContentPreviewRunes    = 240
)

// ContentMatch is one literal text hit in a workspace file.
type ContentMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview"`
}

// ContentSearch is a capped workspace content search.
type ContentSearch struct {
	Matches   []ContentMatch `json:"matches"`
	Truncated bool           `json:"truncated"`
}

// SearchContent finds literal text under a workspace root.
func SearchContent(ctx context.Context, workspaceRoot, query string, limit int) (ContentSearch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ContentSearch{}, errors.New("q is required")
	}
	if limit <= 0 {
		limit = defaultContentSearchLimit
	}
	if limit > maxContentSearchLimit {
		limit = maxContentSearchLimit
	}
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return ContentSearch{}, err
	}

	if rg, _ := process.ResolveCommand("rg"); rg != "" {
		return searchContentRipgrep(ctx, rg, root, query, limit, loadGitignore(root))
	}
	return searchContentNative(ctx, root, query, limit)
}

func searchContentRipgrep(ctx context.Context, rg, root, query string, limit int, ignorer *gitignore.GitIgnore) (ContentSearch, error) {
	cmd := process.CommandContext(ctx, rg,
		"--fixed-strings",
		"--line-number",
		"--column",
		"--no-heading",
		"--color=never",
		"--glob", "!.git/**",
		"--glob", "!node_modules/**",
		"--glob", "!.gitignore",
		"--max-count", strconv.Itoa(limit),
		"--",
		query,
		".",
	)
	cmd.Dir = root
	cmd.Env = process.Env()
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return ContentSearch{Matches: []ContentMatch{}}, nil
		}
		if len(out) == 0 {
			return ContentSearch{}, err
		}
	}
	result := parseContentMatches(string(out), limit)
	if ignorer == nil {
		return result, nil
	}
	kept := make([]ContentMatch, 0, len(result.Matches))
	for _, match := range result.Matches {
		if ignorer.MatchesPath(match.Path) {
			continue
		}
		kept = append(kept, match)
	}
	result.Matches = kept
	return result, nil
}

func searchContentNative(ctx context.Context, root, query string, limit int) (ContentSearch, error) {
	ignorer := loadGitignore(root)
	matches := make([]ContentMatch, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if ctx.Err() != nil || len(matches) >= limit {
			return io.EOF
		}
		if walkErr != nil || path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		name := entry.Name()
		if entry.IsDir() {
			if indexSkippedDirs[name] || strings.HasPrefix(name, ".") || (ignorer != nil && ignorer.MatchesPath(rel+"/")) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignorer != nil && ignorer.MatchesPath(rel) {
			return nil
		}
		fileMatches, err := scanContentFile(path, rel, query, limit-len(matches))
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) && ctx.Err() == nil {
		return ContentSearch{}, err
	}
	return ContentSearch{Matches: matches, Truncated: len(matches) >= limit}, nil
}

func scanContentFile(absPath, rel, query string, limit int) ([]ContentMatch, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	matches := make([]ContentMatch, 0)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		column := strings.Index(line, query)
		if column < 0 || !utf8.ValidString(line) {
			continue
		}
		matches = append(matches, ContentMatch{
			Path:    rel,
			Line:    lineNum,
			Column:  column + 1,
			Preview: trimContentPreview(line),
		})
		if len(matches) >= limit {
			break
		}
	}
	return matches, scanner.Err()
}

func parseContentMatches(text string, limit int) ContentSearch {
	matches := make([]ContentMatch, 0)
	for _, line := range strings.Split(text, "\n") {
		match, ok := parseContentMatch(line)
		if !ok {
			continue
		}
		matches = append(matches, match)
		if len(matches) >= limit {
			return ContentSearch{Matches: matches, Truncated: true}
		}
	}
	return ContentSearch{Matches: matches}
}

func parseContentMatch(line string) (ContentMatch, bool) {
	path, rest, ok := strings.Cut(line, ":")
	if !ok || path == "" {
		return ContentMatch{}, false
	}
	lineText, rest, ok := strings.Cut(rest, ":")
	if !ok {
		return ContentMatch{}, false
	}
	columnText, preview, ok := strings.Cut(rest, ":")
	if !ok {
		return ContentMatch{}, false
	}
	lineNum, err := strconv.Atoi(lineText)
	if err != nil || lineNum < 1 {
		return ContentMatch{}, false
	}
	column, err := strconv.Atoi(columnText)
	if err != nil || column < 1 {
		return ContentMatch{}, false
	}
	return ContentMatch{
		Path:    strings.TrimPrefix(filepath.ToSlash(path), "./"),
		Line:    lineNum,
		Column:  column,
		Preview: trimContentPreview(preview),
	}, true
}

func trimContentPreview(line string) string {
	line = strings.TrimSpace(line)
	runes := []rune(line)
	if len(runes) <= maxContentPreviewRunes {
		return line
	}
	return string(runes[:maxContentPreviewRunes])
}
