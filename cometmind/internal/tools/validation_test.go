package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileRejectsMissingContent(t *testing.T) {
	tool := WriteFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "content is required") {
		t.Fatalf("result = %+v, want content validation error", res)
	}
}

func TestWriteFileAppendsWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	tool := WriteFile{Workspace: Workspace{Root: root}}
	first, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"note.md","content":"# Title\n"}`))
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if !first.OK {
		t.Fatalf("first write = %+v", first)
	}
	second, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"note.md","content":"body\n","append":true}`))
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if !second.OK || !strings.Contains(second.Output, "appended") {
		t.Fatalf("append result = %+v, want appended", second)
	}
	got, err := os.ReadFile(filepath.Join(root, "note.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "# Title\nbody\n" {
		t.Fatalf("file = %q, want title plus body", got)
	}
}

func TestWriteFileAppendCreatesMissingFile(t *testing.T) {
	root := t.TempDir()
	tool := WriteFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"fresh.md","content":"hello","append":true}`))
	if err != nil {
		t.Fatalf("append create: %v", err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	got, err := os.ReadFile(filepath.Join(root, "fresh.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("file = %q, want hello", got)
	}
}

func TestWriteFileAllowsExplicitEmptyContent(t *testing.T) {
	root := t.TempDir()
	tool := WriteFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"note.txt","content":""}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result = %+v, want success", res)
	}
	read := ReadFile{Workspace: Workspace{Root: root}}
	readRes, err := read.Execute(context.Background(), json.RawMessage(`{"path":"note.txt"}`))
	if err != nil {
		t.Fatalf("Read Execute error: %v", err)
	}
	if !readRes.OK || readRes.Output != "" {
		t.Fatalf("read result = %+v, want empty file", readRes)
	}
}

func TestReadFileRejectsMissingPath(t *testing.T) {
	tool := ReadFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "path is required") {
		t.Fatalf("result = %+v, want path validation error", res)
	}
}

func TestListDirRejectsMissingPath(t *testing.T) {
	tool := ListDir{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "path is required") {
		t.Fatalf("result = %+v, want path validation error", res)
	}
}

func TestRunCommandRejectsMissingCommand(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "command is required") {
		t.Fatalf("result = %+v, want command validation error", res)
	}
}

func TestRunCommandRejectsCurlWordWithoutTrailingSpace(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"curl --help"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK {
		t.Fatalf("result = %+v, want curl to be allowed", res)
	}
}

func TestRunCommandAllowsHTTPURLs(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"printf https://example.com"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK || res.Output != "https://example.com" {
		t.Fatalf("result = %+v, want printed URL", res)
	}
}

func TestRunCommandAllowsRedirectToDevNull(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"printf ok 2>/dev/null"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !res.OK || res.Output != "ok" {
		t.Fatalf("result = %+v, want printed output", res)
	}
}

func TestRunCommandRejectsRedirectToOtherDevPaths(t *testing.T) {
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"command":"printf ok >/dev/random"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "matched > /dev/") {
		t.Fatalf("result = %+v, want /dev guardrail error", res)
	}
}

func TestWriteFileRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	tool := WriteFile{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"escape/secret.txt","content":"x"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if res.OK || !strings.Contains(res.Output, "path escapes workspace") {
		t.Fatalf("result = %+v, want workspace escape error", res)
	}
}
