package fsops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// ReadFile reads UTF-8 text within the workspace.
type ReadFile struct{ Workspace Workspace }

const (
	readFileDefaultLimit   = 2000
	readFileMaxLineRunes   = 2000
	readFileMaxOutputRunes = 50000
	readFileMaxInputBytes  = 10 << 20
)

func (ReadFile) Spec() ToolSpec {
	return ToolSpec{
		Name: "read_file",
		Description: "Read a text file. Relative paths resolve against the workspace root; " +
			"absolute paths read anywhere the CometMind process may access. You may also read " +
			"@runtime/tool-output/..., @runtime/tmp/..., and @runtime/wiki/...; other ~/.cometmind files are private. " +
			"Each line is prefixed with its 1-based line number as \"N: content\" " +
			"(the \"N: \" prefix is not part of the file). " +
			"Use offset/limit to window large files.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"Relative path from workspace root or an absolute host path"},
				"offset":{"type":"integer","description":"1-based line number to start reading from (default 1)"},
				"limit":{"type":"integer","description":"Maximum number of lines to return (default 2000)"}
			},
			"required":["path"]
		}`),
	}
}

func (r ReadFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path   *string `json:"path"`
		Offset *int    `json:"offset"`
		Limit  *int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	path, bad, ok := toolkit.RequiredTrimmedString(in.Path, "path")
	if !ok {
		return bad, nil
	}
	p, err := r.Workspace.ResolveReadable(path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	info, err := os.Stat(p)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if !info.Mode().IsRegular() {
		return Result{OK: false, Output: "path is not a regular file"}, nil
	}
	if info.Size() > readFileMaxInputBytes {
		return Result{OK: false, Output: fmt.Sprintf(
			"file is too large to read in full (%d bytes > %d byte limit); use a path-based alternative",
			info.Size(), readFileMaxInputBytes,
		)}, nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if !utf8.Valid(b) {
		return Result{OK: false, Output: "file is not valid UTF-8 text"}, nil
	}

	return formatReadFile(string(b), in.Offset, in.Limit), nil
}

func formatReadFile(content string, offset, limit *int) Result {
	// Preserve empty file as empty output (no line prefix).
	if content == "" {
		return Result{OK: true, Output: ""}
	}

	// Split keeping a trailing empty line only when the file ends with \n
	// would add an extra blank; use strings.Split which matches editor line counts.
	lines := strings.Split(content, "\n")
	// If file ends with newline, Split yields a trailing empty element that
	// represents the empty line after the final \n. Keep it so line counts
	// match common editor behavior for "N lines ending with newline".

	start := 1
	if offset != nil {
		start = *offset
	}
	if start < 1 {
		return Result{OK: false, Output: "offset must be >= 1"}
	}

	lineLimit := readFileDefaultLimit
	if limit != nil {
		lineLimit = *limit
	}
	if lineLimit < 1 {
		return Result{OK: false, Output: "limit must be >= 1"}
	}

	if start > len(lines) {
		return Result{OK: false, Output: fmt.Sprintf(
			"offset %d is past end of file (%d lines)", start, len(lines),
		)}
	}

	end := start + lineLimit - 1
	if end > len(lines) {
		end = len(lines)
	}

	var bld strings.Builder
	for i := start; i <= end; i++ {
		line := lines[i-1]
		if runes := []rune(line); len(runes) > readFileMaxLineRunes {
			line = string(runes[:readFileMaxLineRunes]) + fmt.Sprintf("... (line truncated to %d chars)", readFileMaxLineRunes)
		}
		if i > start {
			bld.WriteByte('\n')
		}
		fmt.Fprintf(&bld, "%d: %s", i, line)
	}

	out := bld.String()
	truncated := false
	if len([]rune(out)) > readFileMaxOutputRunes {
		out, truncated = truncateOutput(out, readFileMaxOutputRunes)
	}

	var footer strings.Builder
	if end < len(lines) {
		fmt.Fprintf(&footer, "\n\n(showing lines %d-%d of %d; use offset/limit to read more)", start, end, len(lines))
	} else if start > 1 {
		fmt.Fprintf(&footer, "\n\n(showing lines %d-%d of %d)", start, end, len(lines))
	}
	if truncated {
		footer.WriteString("\n\n(file truncated for tool output limit)")
	}
	return Result{OK: true, Output: out + footer.String()}
}
