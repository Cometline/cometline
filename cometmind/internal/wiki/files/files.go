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

func includeWikiDocument(relativePath string) bool {
	switch strings.ToLower(filepath.Ext(relativePath)) {
	case ".md", ".markdown", ".html", ".htm", ".pdf":
		return true
	default:
		return false
	}
}

// ListDocumentFiles returns wiki document files and directories, sorted.
// Directories have a trailing slash so clients can distinguish them from files.
func ListDocumentFiles(ctx context.Context, root string, opts ListOptions) (Result, error) {
	opts.IncludeFile = includeWikiDocument
	result, err := filelist.List(ctx, root, opts)
	if errors.Is(err, os.ErrNotExist) {
		return Result{Files: []string{}}, nil
	}
	return result, err
}

// ListDirectory returns direct children of a wiki-root-relative directory.
func ListDirectory(ctx context.Context, root, directory string, opts ListOptions) (Result, error) {
	opts.IncludeFile = includeWikiDocument
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
