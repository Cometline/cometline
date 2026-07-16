package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// EditFile performs surgical search/replace edits inside the workspace.
type EditFile struct{ Workspace Workspace }

func (EditFile) Spec() ToolSpec {
	return ToolSpec{
		Name: "edit_file",
		Description: "Replace text in an existing file (search/replace). " +
			"Prefer this over write_file for modifying existing files. " +
			"old_string must uniquely identify the target (or set replace_all). " +
			"Fuzzy matching tolerates minor whitespace/indentation drift. " +
			"Returns a unified diff on success.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"Relative path from workspace root"},
				"old_string":{"type":"string","description":"Text to find and replace"},
				"new_string":{"type":"string","description":"Replacement text (must differ from old_string)"},
				"replace_all":{"type":"boolean","description":"Replace every occurrence (default false)"}
			},
			"required":["path","old_string","new_string"]
		}`),
	}
}

func (e EditFile) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path       *string `json:"path"`
		OldString  *string `json:"old_string"`
		NewString  *string `json:"new_string"`
		ReplaceAll *bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	path, bad, ok := requiredTrimmedString(in.Path, "path")
	if !ok {
		return bad, nil
	}
	if in.OldString == nil {
		return Result{OK: false, Output: "old_string is required"}, nil
	}
	if in.NewString == nil {
		return Result{OK: false, Output: "new_string is required"}, nil
	}
	oldString := *in.OldString
	newString := *in.NewString
	replaceAll := in.ReplaceAll != nil && *in.ReplaceAll

	abs, err := e.Workspace.ResolveWritable(path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	release := acquireFileLock(abs)
	defer release()

	raw, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{OK: false, Output: "file does not exist; use write_file to create new files"}, nil
		}
		return Result{OK: false, Output: err.Error()}, nil
	}
	if !utf8.Valid(raw) {
		return Result{OK: false, Output: "file is not valid UTF-8 text"}, nil
	}
	content := string(raw)

	// Count occurrences of the eventual match before write for replace_all reporting.
	next, matched, err := applySearchReplace(content, oldString, newString, replaceAll)
	if err != nil {
		msg := err.Error()
		if ctxSnippet := nearbyContext(content, oldString, 2); ctxSnippet != "" {
			msg = msg + "\n\n" + ctxSnippet
		}
		return Result{OK: false, Output: msg}, nil
	}
	if next == content {
		return Result{OK: false, Output: "no changes produced"}, nil
	}

	replaceCount := 1
	if replaceAll {
		replaceCount = strings.Count(normalizeLineEndings(content), normalizeLineEndings(matched))
		if replaceCount < 1 {
			replaceCount = 1
		}
	}

	if err := os.WriteFile(abs, []byte(next), 0o644); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(abs, e.Workspace.Root), string(filepath.Separator))
	rel = filepath.ToSlash(rel)
	if IsRuntimePath(path) {
		rel = filepath.ToSlash(path)
	}
	diff := unifiedDiff(rel, content, next)
	add, del := countDiffLines(diff)

	var out strings.Builder
	fmt.Fprintf(&out, "edited %s (+%d -%d)", rel, add, del)
	if replaceAll {
		fmt.Fprintf(&out, " replace_all=%d", replaceCount)
	}
	out.WriteString("\n\n*** Begin Diff\n")
	out.WriteString(diff)
	if !strings.HasSuffix(diff, "\n") {
		out.WriteByte('\n')
	}
	out.WriteString("*** End Diff")
	return Result{OK: true, Output: out.String()}, nil
}

// unifiedDiff builds a simple line-based unified diff without external deps.
func unifiedDiff(rel, before, after string) string {
	a := strings.Split(normalizeLineEndings(before), "\n")
	b := strings.Split(normalizeLineEndings(after), "\n")
	// Drop trailing empty from Split when file ends with newline? Keep parity
	// with SplitLines: both sides use the same rule.
	ops := myersDiff(a, b)
	var out strings.Builder
	fmt.Fprintf(&out, "--- a/%s\n+++ b/%s\n", rel, rel)

	// Emit one hunk covering the whole file with 3 lines of context style:
	// walk ops and print with @@ header based on first change.
	type lineOp struct {
		kind byte // ' ', '+', '-'
		text string
	}
	var lines []lineOp
	for _, op := range ops {
		lines = append(lines, lineOp{kind: op.kind, text: op.text})
	}
	if len(lines) == 0 {
		return out.String()
	}

	// Find change bounds
	first, last := -1, -1
	for i, l := range lines {
		if l.kind != ' ' {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return out.String()
	}
	const ctx = 3
	start := first - ctx
	if start < 0 {
		start = 0
	}
	end := last + ctx
	if end >= len(lines) {
		end = len(lines) - 1
	}

	// Compute a/b line numbers at start
	aLine, bLine := 1, 1
	for i := 0; i < start; i++ {
		switch lines[i].kind {
		case ' ', '-':
			aLine++
		}
		switch lines[i].kind {
		case ' ', '+':
			bLine++
		}
	}
	aCount, bCount := 0, 0
	for i := start; i <= end; i++ {
		switch lines[i].kind {
		case ' ':
			aCount++
			bCount++
		case '-':
			aCount++
		case '+':
			bCount++
		}
	}
	fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n", aLine, aCount, bLine, bCount)
	for i := start; i <= end; i++ {
		fmt.Fprintf(&out, "%c%s\n", lines[i].kind, lines[i].text)
	}
	return out.String()
}

type diffOp struct {
	kind byte
	text string
}

// myersDiff is a simple LCS-based line diff (fine for tool-sized files).
func myersDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	// DP LCS lengths
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			ops = append(ops, diffOp{kind: ' ', text: a[i]})
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, diffOp{kind: '-', text: a[i]})
			i++
		} else {
			ops = append(ops, diffOp{kind: '+', text: b[j]})
			j++
		}
	}
	for i < n {
		ops = append(ops, diffOp{kind: '-', text: a[i]})
		i++
	}
	for j < m {
		ops = append(ops, diffOp{kind: '+', text: b[j]})
		j++
	}
	return ops
}

func countDiffLines(diff string) (add, del int) {
	for _, line := range strings.Split(diff, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "@@") {
			continue
		}
		switch line[0] {
		case '+':
			add++
		case '-':
			del++
		}
	}
	return add, del
}
