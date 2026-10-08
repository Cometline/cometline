package fsops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/paths"
	wikifiles "github.com/Cometline/cometline/cometmind/internal/wiki/files"
)

// WikiWriteFile is write_file limited to the LLM wiki root.
type WikiWriteFile struct{ Workspace Workspace }

func (WikiWriteFile) Spec() ToolSpec {
	spec := WriteFile{}.Spec()
	spec.Description = "Create or overwrite a file under @runtime/wiki/ only. Existing files in raw/ cannot be edited. Paths outside the wiki are rejected."
	return spec
}

func (w WikiWriteFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if err := rejectWikiWrite(input); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return WriteFile{Workspace: w.Workspace}.Execute(ctx, input)
}

// WikiEditFile is edit_file limited to the LLM wiki root.
type WikiEditFile struct{ Workspace Workspace }

func (WikiEditFile) Spec() ToolSpec {
	spec := EditFile{}.Spec()
	spec.Description = "Edit a file under @runtime/wiki/ only. Existing files in raw/ cannot be edited. Paths outside the wiki are rejected."
	return spec
}

func (w WikiEditFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if err := rejectWikiWrite(input); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return EditFile{Workspace: w.Workspace}.Execute(ctx, input)
}

func rejectWikiWrite(input json.RawMessage) error {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return err
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	abs, err := (Workspace{}).ResolveWritable(path)
	if err != nil {
		return err
	}
	root, err := paths.WikiDir()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path is outside the wiki")
	}
	rel = filepath.ToSlash(rel)
	if !wikifiles.IsWriteProtected(rel) {
		return nil
	}
	if strings.EqualFold(rel, "raw") || strings.HasPrefix(strings.ToLower(rel), "raw/") {
		_, statErr := os.Stat(abs)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		return fmt.Errorf("cannot edit an existing raw wiki file")
	}
	return fmt.Errorf("cannot edit %s", rel)
}

func canonicalPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	var suffix []string
	current := path
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		suffix = append([]string{filepath.Base(current)}, suffix...)
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Join(parts...)
		}
		current = parent
	}
}
