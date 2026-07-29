package files

import (
	"context"

	"github.com/cometline/cometmind/internal/filelist"
)

const (
	DefaultLimit = filelist.DefaultLimit
	MaxLimit     = filelist.MaxLimit
)

var defaultSkippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
}

type ListOptions = filelist.Options

type Result = filelist.Result

// ListFiles returns workspace-relative file and directory paths matching the query, sorted.
// Directories have a trailing slash. Git metadata and dependency directories are skipped.
func ListFiles(ctx context.Context, root string, opts ListOptions) (Result, error) {
	opts.SkipDirectoryNames = defaultSkippedDirs
	return filelist.List(ctx, root, opts)
}
