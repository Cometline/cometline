package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile creates or overwrites a file relative to the workspace.
type WriteFile struct{ Workspace Workspace }

func (WriteFile) Spec() ToolSpec {
	return ToolSpec{
		Name: "write_file",
		Description: "Create a new file or intentionally overwrite an entire file. " +
			"@runtime/tmp/... is a shared cross-session temporary directory. " +
			"@runtime/wiki/... is the persistent LLM Wiki (not age-purged). " +
			"Prefer edit_file for modifying existing files. " +
			"Creates parent directories if needed. " +
			"For long documents (markdown, reports, job descriptions), write one short complete chunk per step, then set append=true for later sections. " +
			"Never put an entire long document in one call — oversized JSON arguments get truncated.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string"},` +
			`"content":{"type":"string"},` +
			`"append":{"type":"boolean","description":"Append content to an existing file instead of overwriting (default false). Creates the file if it does not exist."}` +
			`},"required":["path","content"]}`),
	}
}

func (w WriteFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path    *string `json:"path"`
		Content *string `json:"content"`
		Append  *bool   `json:"append"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	path, bad, ok := requiredTrimmedString(in.Path, "path")
	if !ok {
		return bad, nil
	}
	content, bad, ok := requiredString(in.Content, "content")
	if !ok {
		return bad, nil
	}
	p, err := w.Workspace.ResolveWritable(path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	// Per-file lock so concurrent sessions can write different files.
	release := w.Workspace.LockFile(p)
	defer release()

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	appendMode := in.Append != nil && *in.Append
	if appendMode {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return Result{OK: false, Output: err.Error()}, nil
		}
		n, err := f.WriteString(content)
		closeErr := f.Close()
		if err != nil {
			return Result{OK: false, Output: err.Error()}, nil
		}
		if closeErr != nil {
			return Result{OK: false, Output: closeErr.Error()}, nil
		}
		displayPath := w.Workspace.DisplayPath(p, path)
		return Result{OK: true, Output: fmt.Sprintf("appended %d bytes to %s", n, displayPath)}, nil
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	displayPath := w.Workspace.DisplayPath(p, path)
	return Result{OK: true, Output: fmt.Sprintf("wrote %d bytes to %s", len(content), displayPath)}, nil
}
