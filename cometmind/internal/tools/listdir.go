package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ListDir lists non-hidden entries one level under a path. Relative paths
// resolve against the workspace; absolute paths list any readable directory.
type ListDir struct{ Workspace Workspace }

const listDirMaxEntries = 2000

func (ListDir) Spec() ToolSpec {
	return ToolSpec{
		Name: "list_dir",
		Description: "List files and directories at a path (non-recursive). " +
			"Relative paths resolve against the workspace root; absolute paths list any readable directory. " +
			"Hidden entries are skipped. Use @runtime to list managed shared mounts.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative directory (use . for workspace root) or an absolute host directory"}},"required":["path"]}`),
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
	if path == RuntimePrefix || path == RuntimePrefix+"/" {
		return Result{OK: true, Output: "tool-output/\ntmp/\nwiki/\n"}, nil
	}
	p, err := l.Workspace.ResolveReadable(path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	var b strings.Builder
	listed := 0
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
		listed++
		if listed >= listDirMaxEntries {
			fmt.Fprintf(&b, "(listing truncated at %d entries)\n", listDirMaxEntries)
			break
		}
	}
	return Result{OK: true, Output: b.String()}, nil
}
