package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrep_Basic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\nfunc Hello() {}\n")

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"Hello","literal_text":true}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "main.go:2:") || !strings.Contains(res.Output, "func Hello()") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
}

func TestGrep_Regex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "foo123bar\n")

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"foo\\d+bar"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "main.go:1:") || !strings.Contains(res.Output, "foo123bar") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
}

func TestGrep_IncludeFilter(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "app.ts"), "needle here\n")
	writeTestFile(t, filepath.Join(root, "app.go"), "needle here\n")

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"needle","literal_text":true,"include":"**/*.ts"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "app.ts:") {
		t.Fatalf("expected app.ts match: %q", res.Output)
	}
	if strings.Contains(strings.Split(res.Output, "\n\n(")[0], "app.go:") {
		t.Fatalf("should not match app.go: %q", res.Output)
	}
}

func TestGrep_RespectsGitignore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "keep.go"), "secret-token\n")
	writeTestFile(t, filepath.Join(root, "ignore.log"), "secret-token\n")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "*.log\n")

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"secret-token","literal_text":true}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if strings.Contains(res.Output, "ignore.log:") {
		t.Fatalf("gitignored file should be skipped: %q", res.Output)
	}
	if !strings.Contains(res.Output, "keep.go:") {
		t.Fatalf("expected keep.go match: %q", res.Output)
	}
}

func TestGrep_TruncatedOutput(t *testing.T) {
	saved := rgPath
	rgPath = ""
	t.Cleanup(func() { rgPath = saved })

	root := t.TempDir()
	var b strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "match line %d with padding\n", i)
	}
	writeTestFile(t, filepath.Join(root, "big.go"), b.String())

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"match line","literal_text":true}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "truncated") {
		t.Fatalf("expected truncated footer, got: %q", res.Output)
	}
	if len([]rune(strings.Split(res.Output, "\n\n(")[0])) > grepMaxOutputChars {
		t.Fatalf("output body exceeds max chars")
	}
}

func TestGrep_FallbackWithoutRg(t *testing.T) {
	saved := rgPath
	rgPath = ""
	t.Cleanup(func() { rgPath = saved })

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "fallback needle\n")

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"fallback needle","literal_text":true}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "main.go:1: fallback needle") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
}

func TestGrep_NoMatches(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main\n")

	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"not-present-xyz","literal_text":true}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "(0 matches)") {
		t.Fatalf("expected zero matches footer, got: %q", res.Output)
	}
}

func TestGrep_RejectsMissingPattern(t *testing.T) {
	tool := Grep{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "pattern is required") {
		t.Fatalf("result = %+v, want pattern validation error", res)
	}
}
