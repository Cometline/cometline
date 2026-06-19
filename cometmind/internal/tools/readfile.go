package tools

import (
	"context"
	"encoding/json"
	"os"
)

// ReadFile reads UTF-8 text within the workspace.
type ReadFile struct{ Workspace Workspace }

func (ReadFile) Spec() ToolSpec {
	return ToolSpec{
		Name:        "read_file",
		Description: "Read the contents of a text file relative to the workspace root.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Relative path from workspace root"}},"required":["path"]}`),
	}
}

func (r ReadFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
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
	p, err := r.Workspace.Resolve(path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: string(b)}, nil
}
