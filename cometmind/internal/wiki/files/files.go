package files

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
	MaxLimit     = 500
)

// ListOptions controls wiki markdown file listing.
type ListOptions struct {
	Query string
	Limit int
}

// Result is the outcome of a wiki file listing.
type Result struct {
	Files     []string
	Truncated bool
}

func isListableWikiExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".html", ".htm":
		return true
	default:
		return false
	}
}

// ListMarkdownFiles returns wiki-root-relative wiki page paths (.md and .html), sorted.
// HTML is included so raw ingest sources linked as [[….html]] can resolve in the UI.
func ListMarkdownFiles(ctx context.Context, root string, opts ListOptions) (Result, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Files: []string{}}, nil
		}
		return Result{}, fmt.Errorf("stat wiki: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("wiki path is not a directory: %s", root)
	}

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
			return nil
		}

		if !isListableWikiExt(filepath.Ext(name)) {
			return nil
		}

		rel = filepath.ToSlash(rel)
		if query != "" {
			if !strings.Contains(strings.ToLower(rel), queryLower) {
				return nil
			}
		}

		results = append(results, rel)
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

// IsWriteProtected reports paths that must not be edited from the UI.
func IsWriteProtected(rel string) bool {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	if rel == "" {
		return true
	}
	if strings.EqualFold(rel, "WIKI.md") {
		return true
	}
	return strings.EqualFold(rel, "raw") || strings.HasPrefix(strings.ToLower(rel), "raw/")
}
