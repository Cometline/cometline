package files

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/filelist"
)

const (
	DefaultLimit = filelist.DefaultLimit
	MaxLimit     = filelist.MaxLimit
)

type ListOptions = filelist.Options

type Result = filelist.Result

// ListMarkdownFiles returns wiki-root-relative file and directory paths, sorted.
// Directories have a trailing slash so clients can distinguish them from files.
func ListMarkdownFiles(ctx context.Context, root string, opts ListOptions) (Result, error) {
	result, err := filelist.List(ctx, root, opts)
	if errors.Is(err, os.ErrNotExist) {
		return Result{Files: []string{}}, nil
	}
	return result, err
}

// ListDirectory returns direct children of a wiki-root-relative directory.
func ListDirectory(ctx context.Context, root, directory string, opts ListOptions) (Result, error) {
	result, err := filelist.ListDirectory(ctx, root, directory, opts)
	if errors.Is(err, os.ErrNotExist) {
		return Result{Files: []string{}}, nil
	}
	return result, err
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
