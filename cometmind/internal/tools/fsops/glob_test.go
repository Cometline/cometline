package fsops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestGlob_Basic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "main.go"), "package main")
	writeTestFile(t, filepath.Join(root, "README.md"), "# readme")
	writeTestFile(t, filepath.Join(root, "helper.go"), "package main")

	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "main.go") || !strings.Contains(res.Output, "helper.go") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	if strings.Contains(res.Output, "README.md") {
		t.Fatalf("should not match README.md: %q", res.Output)
	}
}

func TestGlob_Recursive(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "src", "app.ts"), "export {}")
	writeTestFile(t, filepath.Join(root, "src", "lib", "util.ts"), "export {}")
	writeTestFile(t, filepath.Join(root, "src", "app.go"), "package main")

	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"**/*.ts"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "src/app.ts") || !strings.Contains(res.Output, "src/lib/util.ts") {
		t.Fatalf("unexpected output: %q", res.Output)
	}
	if strings.Contains(res.Output, "app.go") {
		t.Fatalf("should not match app.go: %q", res.Output)
	}
}

func TestGlob_CappedAt100(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 105; i++ {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("file%d.go", i)), "x")
	}

	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not OK: %s", res.Output)
	}
	if !strings.Contains(res.Output, "truncated") {
		t.Fatalf("expected truncated footer, got: %q", res.Output)
	}
	lines := strings.Split(strings.Split(res.Output, "\n\n(")[0], "\n")
	if len(lines) != 100 {
		t.Fatalf("want 100 file lines, got %d", len(lines))
	}
}

func TestGlob_SortedByMtime(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old.go")
	newPath := filepath.Join(root, "new.go")
	writeTestFile(t, oldPath, "x")
	writeTestFile(t, newPath, "x")

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes old: %v", err)
	}
	if err := os.Chtimes(newPath, newTime, newTime); err != nil {
		t.Fatalf("Chtimes new: %v", err)
	}

	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	body := strings.Split(res.Output, "\n\n(")[0]
	lines := strings.Split(body, "\n")
	if len(lines) < 2 || lines[0] != "new.go" {
		t.Fatalf("newest file should be first, got: %v", lines)
	}
}

func TestGlob_RespectsGitignore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "keep.go"), "x")
	writeTestFile(t, filepath.Join(root, "ignore.log"), "x")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "*.log\n")

	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if strings.Contains(res.Output, "ignore.log") {
		t.Fatalf("gitignored file should be skipped: %q", res.Output)
	}
	if !strings.Contains(res.Output, "keep.go") {
		t.Fatalf("expected keep.go in output: %q", res.Output)
	}
}

func TestGlob_RejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*","path":"escape"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "path escapes workspace") {
		t.Fatalf("result = %+v, want workspace escape error", res)
	}
}

func TestGlob_RejectsMissingPattern(t *testing.T) {
	tool := Glob{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "pattern is required") {
		t.Fatalf("result = %+v, want pattern validation error", res)
	}
}
