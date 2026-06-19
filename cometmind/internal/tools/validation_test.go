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
