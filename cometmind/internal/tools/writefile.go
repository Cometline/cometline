package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile creates or overwrites a file relative to the workspace.
type WriteFile struct{ Workspace Workspace }

func (WriteFile) Spec() ToolSpec {
	return ToolSpec{
		Name:        "write_file",
		Description: "Write text to a file relative to the workspace root, creating parent directories if needed.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
	}
}

func (w WriteFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path    *string `json:"path"`
		Content *string `json:"content"`
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
	p, err := w.Workspace.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	// Acquire a per-workspace mutex to prevent concurrent sessions from
	// interleaving writes to the same workspace root.
	release := acquireWorkspaceLock(w.Workspace.Root)
	defer release()

	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("wrote %d bytes to %s", len(content), strings.TrimPrefix(strings.TrimPrefix(p, w.Workspace.Root), string(filepath.Separator)))}, nil
}
