package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/cometline/cometmind/internal/filelist"
)

const (
	DefaultLimit  = filelist.DefaultLimit
	MaxLimit      = filelist.MaxLimit
	IndexMaxLimit = filelist.IndexMaxLimit
)

var defaultSkippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
}

var indexSkippedDirs = map[string]bool{
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
	".svelte-kit":  true,
	"target":       true,
	".turbo":       true,
	".cache":       true,
}

type ListOptions = filelist.Options

type Result = filelist.Result

// ListFiles returns workspace-relative file and directory paths matching the query, sorted.
// Directories have a trailing slash. Git metadata and dependency directories are skipped.
// When opts.Index is true, common build directories and root .gitignore rules are also skipped.
func ListFiles(ctx context.Context, root string, opts ListOptions) (Result, error) {
	if opts.Index {
		return listIndex(ctx, root, opts)
	}
	opts.SkipDirectoryNames = defaultSkippedDirs
	return filelist.List(ctx, root, opts)
}

// ListDirectory returns direct children of a workspace-relative directory.
func ListDirectory(ctx context.Context, root, directory string, opts ListOptions) (Result, error) {
	opts.SkipDirectoryNames = defaultSkippedDirs
	return filelist.ListDirectory(ctx, root, directory, opts)
}

func listIndex(ctx context.Context, root string, opts ListOptions) (Result, error) {
	opts.SkipDirectoryNames = indexSkippedDirs
	ignorer := loadGitignore(root)
	opts.SkipPath = func(relativePath string, isDir bool) bool {
		name := filepath.Base(strings.TrimSuffix(relativePath, "/"))
		if isDir && strings.HasPrefix(name, ".") {
			return true
		}
		if ignorer == nil {
			return false
		}
		if isDir {
			return ignorer.MatchesPath(strings.TrimSuffix(relativePath, "/") + "/")
		}
		return ignorer.MatchesPath(relativePath)
	}
	return filelist.List(ctx, root, opts)
}

func loadGitignore(root string) *gitignore.GitIgnore {
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	return gitignore.CompileIgnoreLines(strings.Split(string(data), "\n")...)
}
