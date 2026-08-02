package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Glob finds files by name pattern under the workspace.
type Glob struct{ Workspace Workspace }

type globInput struct {
	Pattern *string `json:"pattern"`
	Path    *string `json:"path,omitempty"`
}

type globMatch struct {
	rel   string
	mtime int64
}

func (Glob) Spec() ToolSpec {
	return ToolSpec{
		Name: "glob",
		Description: "Find files by name pattern. Supports standard glob syntax " +
			"including ** for recursive matching. Results sorted by modification time " +
			"(newest first), capped at 100 files. Skips gitignored paths. " +
			"The path may be @runtime/wiki or an absolute host directory.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {"type": "string", "description": "Glob pattern, e.g. **/*.ts, src/**/*.go, *.{json,yaml}"},
				"path": {"type": "string", "description": "Optional subdirectory to scope the search: workspace-relative or an absolute host directory (default workspace root)"}
			},
			"required": ["pattern"]
		}`),
	}
}

func (g Glob) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	pattern, bad, ok := requiredTrimmedString(in.Pattern, "pattern")
	if !ok {
		return bad, nil
	}

	searchRel := "."
	if in.Path != nil {
		trimmed := strings.TrimSpace(*in.Path)
		if trimmed != "" {
			searchRel = trimmed
		}
	}

	searchRoot, displayPrefix, err := g.Workspace.ResolveSearchRoot(searchRel)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	workspaceRoot, err := g.Workspace.Resolve(".")
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if displayPrefix != "" {
		workspaceRoot = searchRoot
	}

	walkCtx, cancel := withSearchDeadline(ctx)
	defer cancel()

	var matches []globMatch
	err = walkSearchableFiles(walkCtx, workspaceRoot, searchRoot, func(workspaceRel, absPath string) error {
		matched, err := doublestar.PathMatch(pattern, workspaceRel)
		if err != nil || !matched {
			return nil
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return nil
		}
		matches = append(matches, globMatch{rel: searchDisplayPath(displayPrefix, workspaceRel), mtime: info.ModTime().UnixNano()})
		if len(matches) > globMaxFiles {
			return fs.SkipAll
		}
		return nil
	})
	budgetExhausted := errors.Is(err, errSearchBudget) || errors.Is(err, context.DeadlineExceeded)
	if err != nil && !errors.Is(err, fs.SkipAll) && !budgetExhausted {
		return Result{OK: false, Output: err.Error()}, nil
	}

	truncated := len(matches) > globMaxFiles || errors.Is(err, fs.SkipAll) || budgetExhausted
	if truncated && len(matches) > globMaxFiles {
		matches = matches[:globMaxFiles]
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].mtime > matches[j].mtime
	})

	paths := make([]string, len(matches))
	for i, m := range matches {
		paths[i] = m.rel
	}

	out := strings.Join(paths, "\n") + formatSearchFooter(len(paths), truncated, "files")
	return Result{OK: true, Output: out}, nil
}

func searchDisplayPath(prefix, rel string) string {
	if prefix == "" {
		return rel
	}
	return strings.TrimSuffix(prefix, "/") + "/" + rel
}
