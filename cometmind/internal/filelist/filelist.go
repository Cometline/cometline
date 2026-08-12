// Package filelist lists relative filesystem entries for browse and search UIs.
package filelist

import (
	"context"
	"fmt"
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
	IncludeFile        func(relativePath string) bool
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
	// Enumerate level by level so a large first directory cannot hide later root entries.
	directories := []string{root}
	for len(directories) > 0 {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}

		dir := directories[0]
		directories = directories[1:]
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}

			isDir := entry.IsDir()
			if isDir && opts.SkipDirectoryNames[entry.Name()] {
				continue
			}

			rel, err := filepath.Rel(root, filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if isDir {
				directories = append(directories, filepath.Join(dir, entry.Name()))
				rel += "/"
			} else if opts.IncludeFile != nil && !opts.IncludeFile(rel) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(rel), query) {
				continue
			}

			results = append(results, rel)
			// Collect one extra so callers can distinguish an exact limit from truncation.
			if len(results) > limit {
				truncated := results[:limit]
				sort.Strings(truncated)
				return Result{Files: truncated, Truncated: true}, nil
			}
		}
	}

	sort.Strings(results)
	return Result{Files: results}, nil
}

// ListDirectory returns direct children of a root-relative directory, sorted.
// Directory paths retain a trailing slash and are relative to root.
func ListDirectory(ctx context.Context, root, directory string, opts Options) (Result, error) {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("path is not a directory: %s", root)
	}

	requested := strings.TrimSpace(strings.ReplaceAll(directory, "\\", "/"))
	if requested == "." {
		requested = ""
	}
	relativeDirectory := filepath.Clean(filepath.FromSlash(requested))
	if requested == "" {
		relativeDirectory = "."
	}
	if filepath.IsAbs(relativeDirectory) || relativeDirectory == ".." || strings.HasPrefix(relativeDirectory, ".."+string(filepath.Separator)) {
		return Result{}, fmt.Errorf("directory must be relative to root")
	}

	target := filepath.Join(root, relativeDirectory)
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve root: %w", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return Result{}, fmt.Errorf("directory not found")
	}
	resolvedRelative, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || resolvedRelative == ".." || strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) {
		return Result{}, fmt.Errorf("directory must be within root")
	}
	info, err = os.Stat(resolvedTarget)
	if err != nil {
		return Result{}, fmt.Errorf("directory not found")
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("not a directory")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	query := strings.ToLower(strings.TrimSpace(opts.Query))
	entries, err := os.ReadDir(resolvedTarget)
	if err != nil {
		return Result{}, fmt.Errorf("read directory: %w", err)
	}

	results := make([]string, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		isDir := entry.IsDir()
		if isDir && opts.SkipDirectoryNames[entry.Name()] {
			continue
		}

		path := filepath.Join(relativeDirectory, entry.Name())
		path = filepath.ToSlash(path)
		if relativeDirectory == "." {
			path = entry.Name()
		}
		if isDir {
			path += "/"
		} else if opts.IncludeFile != nil && !opts.IncludeFile(path) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(path), query) {
			continue
		}

		results = append(results, path)
		if len(results) > limit {
			results = results[:limit]
			sort.Strings(results)
			return Result{Files: results, Truncated: true}, nil
		}
	}

	sort.Strings(results)
	return Result{Files: results}, nil
}
