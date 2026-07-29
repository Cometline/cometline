// Package filelist lists relative filesystem entries for browse and search UIs.
package filelist

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultLimit = 50
	MaxLimit     = 10000
)

// Options controls a relative filesystem entry listing.
type Options struct {
	Query              string
	Limit              int
	SkipDirectoryNames map[string]bool
}

// Result is the outcome of an entry listing.
type Result struct {
	Files []string
	// Truncated is true when more matching entries exist than the limit allowed,
	// so callers know the list is incomplete and should narrow with a query.
	Truncated bool
}

// List returns root-relative file and directory paths matching the query, sorted.
// Directories have a trailing slash so clients can distinguish them from files.
func List(ctx context.Context, root string, opts Options) (Result, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("path is not a directory: %s", root)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	query := strings.ToLower(strings.TrimSpace(opts.Query))
	var results []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}

		if d.IsDir() && opts.SkipDirectoryNames[d.Name()] {
			return fs.SkipDir
		}

		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		}
		if query != "" && !strings.Contains(strings.ToLower(rel), query) {
			return nil
		}

		results = append(results, rel)
		// Collect one extra so callers can distinguish an exact limit from truncation.
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
