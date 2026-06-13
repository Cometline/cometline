package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// ListDir lists non-hidden entries one level under a path relative to the workspace.
type ListDir struct{ Root string }

func (ListDir) Name() string { return "list_dir" }

func (ListDir) Description() string {
	return "List files and directories at a path relative to the workspace root (non-recursive)."
}

func (ListDir) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative directory; use . for workspace root"}},"required":["path"]}`)
}

func (l ListDir) Execute(ctx context.Context, workspaceRoot string, input json.RawMessage) (Result, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	p, err := sandbox.ResolveWorkspacePath(l.Root, in.Path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	var b strings.Builder
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			fmt.Fprintf(&b, "%s/\n", name)
		} else {
			fmt.Fprintf(&b, "%s\n", name)
		}
	}
	return Result{OK: true, Output: b.String()}, nil
}
