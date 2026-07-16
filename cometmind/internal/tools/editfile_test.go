package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileExactReplace(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "main.go",
		"old_string": "func Hello() {}",
		"new_string": "func Hello() string { return \"hi\" }",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(res.Output, "*** Begin Diff") || !strings.Contains(res.Output, "*** End Diff") {
		t.Fatalf("missing diff markers: %s", res.Output)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `return "hi"`) {
		t.Fatalf("file content = %q", got)
	}
}

func TestEditFileReplaceAll(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("foo bar foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":        "a.txt",
		"old_string":  "foo",
		"new_string":  "baz",
		"replace_all": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "baz bar baz\n" {
		t.Fatalf("content = %q", got)
	}
	if !strings.Contains(res.Output, "replace_all=2") {
		t.Fatalf("output = %s", res.Output)
	}
}

func TestEditFileMultiMatchFails(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("foo\nbar\nfoo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "a.txt",
		"old_string": "foo",
		"new_string": "baz",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "multiple matches") {
		t.Fatalf("result = %+v", res)
	}
}

func TestEditFileFuzzyLineTrimmed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	// File uses tabs; model provides spaces in old_string (line-trimmed match).
	content := "package main\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "a.go",
		"old_string": "func main() {\n  fmt.Println(\"hi\")\n}",
		"new_string": "func main() {\n\tfmt.Println(\"hello\")\n}",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `fmt.Println("hello")`) {
		t.Fatalf("content = %q", got)
	}
}

func TestEditFileWhitespaceNormalized(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("hello   world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "a.txt",
		"old_string": "hello world",
		"new_string": "hi earth",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hi earth\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestEditFileEmptyOldStringRejected(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := EditFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "a.txt",
		"old_string": "",
		"new_string": "y",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "old_string cannot be empty") {
		t.Fatalf("result = %+v", res)
	}
}

func TestEditFileMissingFile(t *testing.T) {
	tool := EditFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "nope.txt",
		"old_string": "a",
		"new_string": "b",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "does not exist") {
		t.Fatalf("result = %+v", res)
	}
}

func TestEditFilePathEscape(t *testing.T) {
	tool := EditFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":       "../outside.txt",
		"old_string": "a",
		"new_string": "b",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatalf("expected path escape rejection, got %+v", res)
	}
}

func TestReadFileLineNumbersAndWindow(t *testing.T) {
	root := t.TempDir()
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ReadFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{
		"path":   "f.txt",
		"offset": 3,
		"limit":  2,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(res.Output, "3: line3") || !strings.Contains(res.Output, "4: line4") {
		t.Fatalf("output = %q", res.Output)
	}
	if strings.Contains(res.Output, "5: line5") {
		t.Fatalf("limit not applied: %q", res.Output)
	}
	if !strings.Contains(res.Output, "showing lines 3-4 of") {
		t.Fatalf("missing footer: %q", res.Output)
	}
}

func TestReadFileDefaultPrefixesLines(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ReadFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), editTestJSON(t, map[string]any{"path": "hello.txt"}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Output != "1: world" {
		t.Fatalf("output = %q, want %q", res.Output, "1: world")
	}
}

func TestApplySearchReplaceIdenticalRejected(t *testing.T) {
	_, _, err := applySearchReplace("abc", "a", "a", false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func editTestJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
