package tools

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/cometline/cometmind/internal/paths"
)

const (
	globMaxFiles       = 100
	grepMaxOutputChars = 12000
	grepTimeout        = 60 * time.Second
	searchMaxVisited   = 250_000
	searchWalkTimeout  = 90 * time.Second
)

// errSearchBudget stops a recursive search when the visited-entry budget is
// exhausted so host-wide searches cannot traverse unbounded directory trees.
var errSearchBudget = fmt.Errorf("search exceeded traversal budget")

// withSearchDeadline bounds a host-wide search walk. External roots such as
// /usr or /Library are intentionally readable but must not run forever.
func withSearchDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, searchWalkTimeout)
}

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

func formatSearchFooter(count int, truncated bool, unit string) string {
	if truncated {
		return fmt.Sprintf("\n\n(%d %s, truncated)", count, unit)
	}
	return fmt.Sprintf("\n\n(%d %s)", count, unit)
}

func truncateOutput(s string, max int) (string, bool) {
	runes := []rune(s)
	if len(runes) <= max {
		return s, false
	}
	return string(runes[:max]), true
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

func shouldSkipDir(name, workspaceRel string, ignorer *gitignore.GitIgnore) bool {
	if defaultSkippedDirs[name] {
		return true
	}
	if ignorer != nil && ignorer.MatchesPath(workspaceRel+"/") {
		return true
	}
	return false
}

func shouldSkipFile(workspaceRel string, ignorer *gitignore.GitIgnore) bool {
	return ignorer != nil && ignorer.MatchesPath(workspaceRel)
}

// walkSearchableFiles visits regular files under searchRoot, invoking fn with
// workspace-relative paths (forward slashes). Hidden entries, common build
// directories, and root .gitignore rules are skipped. The walk is bounded by
// context cancellation and a visited-entry budget.
func walkSearchableFiles(ctx context.Context, workspaceRoot, searchRoot string, fn func(workspaceRel, absPath string) error) error {
	workspaceRoot = filepath.Clean(workspaceRoot)
	searchRoot = filepath.Clean(searchRoot)

	info, err := os.Stat(searchRoot)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("search path is not a directory: %s", searchRoot)
	}

	ignorer := loadGitignore(workspaceRoot)
	visited := 0

	return filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		visited++
		if visited > searchMaxVisited {
			return errSearchBudget
		}
		if walkErr != nil {
			return nil
		}

		workspaceRel, err := filepath.Rel(workspaceRoot, path)
		if err != nil {
			return nil
		}
		workspaceRel = filepath.ToSlash(workspaceRel)
		if workspaceRel == "." {
			return nil
		}

		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if paths.IsTerminalEnvPath(path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if shouldSkipDir(name, workspaceRel, ignorer) {
				return fs.SkipDir
			}
			return nil
		}

		if shouldSkipFile(workspaceRel, ignorer) {
			return nil
		}

		return fn(workspaceRel, path)
	})
}
