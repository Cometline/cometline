package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// WriteFile creates or overwrites a file relative to the workspace.
type WriteFile struct{ Root string }

func (WriteFile) Name() string { return "write_file" }

func (WriteFile) Description() string {
	return "Write text to a file relative to the workspace root, creating parent directories if needed."
}

func (WriteFile) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}

func (w WriteFile) Execute(ctx context.Context, workspaceRoot string, input json.RawMessage) (Result, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	p, err := sandbox.ResolveWorkspacePath(w.Root, in.Path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if err := os.WriteFile(p, []byte(in.Content), 0o644); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("wrote %d bytes to %s", len(in.Content), strings.TrimPrefix(strings.TrimPrefix(p, w.Root), string(filepath.Separator)))}, nil
}
