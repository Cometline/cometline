package files

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

const (
	DefaultLimit = 50
	MaxLimit     = 500
)

var defaultSkippedDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"out":          true,
	"coverage":     true,
	"__pycache__":  true,
	".git":         true,
	".svn":         true,
	".hg":          true,
}

// ListOptions controls the workspace file listing.
type ListOptions struct {
	Query string
	Limit int
}

// Result is the outcome of a workspace file listing.
type Result struct {
	Files []string
	// Truncated is true when more matching files exist than the limit allowed,
	// so callers know the list is incomplete and should narrow with a query.
	Truncated bool
}

// ListFiles returns workspace-relative file paths matching the query, sorted.
// It skips hidden files and directories, common build/output directories, and
// entries ignored by .gitignore at the workspace root.
func ListFiles(ctx context.Context, root string, opts ListOptions) (Result, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("workspace path is not a directory: %s", root)
	}

	ignorer := loadGitignore(root)

	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	query := strings.TrimSpace(opts.Query)
	queryLower := strings.ToLower(query)

	var results []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if defaultSkippedDirs[name] {
				return fs.SkipDir
			}
			if ignorer != nil && ignorer.MatchesPath(rel+"/") {
				return fs.SkipDir
			}
			return nil
		}

		if ignorer != nil && ignorer.MatchesPath(rel) {
			return nil
		}

		if query != "" {
			lower := strings.ToLower(rel)
			if !strings.Contains(lower, queryLower) {
				return nil
			}
		}

		results = append(results, filepath.ToSlash(rel))
		// Collect one extra so we can tell "exactly limit" from "more exist".
		if len(results) > limit {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	truncated := len(results) > limit
	if truncated {
		results = results[:limit]
	}
	sort.Strings(results)
	return Result{Files: results, Truncated: truncated}, nil
}

func loadGitignore(root string) *gitignore.GitIgnore {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	return gitignore.CompileIgnoreLines(lines...)
}
