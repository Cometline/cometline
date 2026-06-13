package tools

import (
	"context"
	"encoding/json"
	"os"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// ReadFile reads UTF-8 text within the workspace.
type ReadFile struct{ Root string }

func (ReadFile) Name() string { return "read_file" }

func (ReadFile) Description() string {
	return "Read the contents of a text file relative to the workspace root."
}

func (ReadFile) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative path from workspace root"}},"required":["path"]}`)
}

func (r ReadFile) Execute(ctx context.Context, workspaceRoot string, input json.RawMessage) (Result, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	p, err := sandbox.ResolveWorkspacePath(r.Root, in.Path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: string(b)}, nil
}
