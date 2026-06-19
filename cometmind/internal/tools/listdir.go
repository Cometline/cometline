package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ListDir lists non-hidden entries one level under a path relative to the workspace.
type ListDir struct{ Workspace Workspace }

func (ListDir) Spec() ToolSpec {
	return ToolSpec{
		Name:        "list_dir",
		Description: "List files and directories at a path relative to the workspace root (non-recursive).",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative directory; use . for workspace root"}},"required":["path"]}`),
	}
}

func (l ListDir) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path *string `json:"path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	path, bad, ok := requiredTrimmedString(in.Path, "path")
	if !ok {
		return bad, nil
	}
	p, err := l.Workspace.Resolve(path)
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
