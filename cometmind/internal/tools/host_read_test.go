package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func quoteJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// hostReadFixture creates a workspace plus an external fixture tree outside it.
func hostReadFixture(t *testing.T) (workspaceRoot, externalDir string) {
	t.Helper()
	workspaceRoot = t.TempDir()
	externalDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(externalDir, "note.txt"), []byte("external content\nline two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(externalDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(externalDir, "nested", "deep.txt"), []byte("nested external"), 0o644); err != nil {
		t.Fatal(err)
	}
	return workspaceRoot, externalDir
}

func TestReadFileAbsoluteExternalPath(t *testing.T) {
	root, external := hostReadFixture(t)
	reader := ReadFile{Workspace: Workspace{Root: root}}

	res, err := reader.Execute(context.Background(), json.RawMessage(`{"path":`+quoteJSON(t, filepath.Join(external, "note.txt"))+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "external content") {
		t.Fatalf("read result = %+v", res)
	}
}

func TestReadFileRejectsNonRegularFiles(t *testing.T) {
	root, external := hostReadFixture(t)
	reader := ReadFile{Workspace: Workspace{Root: root}}

	res, err := reader.Execute(context.Background(), json.RawMessage(`{"path":`+quoteJSON(t, external)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "not a regular file") {
		t.Fatalf("directory read result = %+v, want regular-file rejection", res)
	}

	fifo := filepath.Join(external, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	res, err = reader.Execute(context.Background(), json.RawMessage(`{"path":`+quoteJSON(t, fifo)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "not a regular file") {
		t.Fatalf("fifo read result = %+v, want regular-file rejection", res)
	}
}

func TestReadFileRejectsOversizedInput(t *testing.T) {
	root, external := hostReadFixture(t)
	big := filepath.Join(external, "big.bin")
	if err := os.WriteFile(big, make([]byte, readFileMaxInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := ReadFile{Workspace: Workspace{Root: root}}
	res, err := reader.Execute(context.Background(), json.RawMessage(`{"path":`+quoteJSON(t, big)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "too large") {
		t.Fatalf("oversized read result = %+v, want size rejection", res)
	}
}

func TestReadFileWorkspaceSymlinkToExternal(t *testing.T) {
	root, external := hostReadFixture(t)
	if err := os.Symlink(filepath.Join(external, "note.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	reader := ReadFile{Workspace: Workspace{Root: root}}
	res, err := reader.Execute(context.Background(), json.RawMessage(`{"path":"link.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "external content") {
		t.Fatalf("symlink read result = %+v", res)
	}
}

func TestListDirHidesTerminalEnv(t *testing.T) {
	data := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", data)
	if err := os.MkdirAll(filepath.Join(data, "terminal-env", "sess"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "visible.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := ListDir{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":`+quoteJSON(t, data)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "visible.txt") {
		t.Fatalf("list result = %+v", res)
	}
	if strings.Contains(res.Output, "terminal-env") {
		t.Fatalf("terminal-env leaked: %s", res.Output)
	}
}

func TestGlobSkipsTerminalEnv(t *testing.T) {
	data := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", data)
	envFile := filepath.Join(data, "terminal-env", "sess", "environ")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "ok.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := Glob{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"**/*","path":`+quoteJSON(t, data)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "ok.txt") {
		t.Fatalf("glob result = %+v", res)
	}
	if strings.Contains(res.Output, "terminal-env") || strings.Contains(res.Output, "environ") {
		t.Fatalf("terminal-env leaked: %s", res.Output)
	}
}

func TestListDirAbsoluteExternalPath(t *testing.T) {
	root, external := hostReadFixture(t)
	tool := ListDir{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":`+quoteJSON(t, external)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "note.txt") || !strings.Contains(res.Output, "nested/") {
		t.Fatalf("list result = %+v", res)
	}
}

func TestGlobAbsoluteExternalPath(t *testing.T) {
	root, external := hostReadFixture(t)
	tool := Glob{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"**/*.txt","path":`+quoteJSON(t, external)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("glob result = %+v", res)
	}
	for _, want := range []string{"deep.txt", "note.txt"} {
		if !strings.Contains(res.Output, want) {
			t.Fatalf("glob output missing %q: %s", want, res.Output)
		}
	}
	// External results must retain their absolute display path.
	if !strings.Contains(res.Output, external) {
		t.Fatalf("glob output missing absolute root %q: %s", external, res.Output)
	}
}

func TestGrepAbsoluteExternalPath(t *testing.T) {
	root, external := hostReadFixture(t)
	tool := Grep{Workspace: Workspace{Root: root}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"external content","path":`+quoteJSON(t, external)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "note.txt") || !strings.Contains(res.Output, "external content") {
		t.Fatalf("grep result = %+v", res)
	}
	if !strings.Contains(res.Output, external) {
		t.Fatalf("grep output missing absolute root %q: %s", external, res.Output)
	}
}

func TestRelativePathsStillResolveFromWorkspace(t *testing.T) {
	root, _ := hostReadFixture(t)
	if err := os.WriteFile(filepath.Join(root, "local.txt"), []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := ReadFile{Workspace: Workspace{Root: root}}
	res, err := reader.Execute(context.Background(), json.RawMessage(`{"path":"local.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "local") {
		t.Fatalf("relative read result = %+v", res)
	}
}

func TestWritableBoundaryStillEnforced(t *testing.T) {
	root, external := hostReadFixture(t)
	writer := WriteFile{Workspace: Workspace{Root: root}}
	res, err := writer.Execute(context.Background(), json.RawMessage(`{
		"path":`+quoteJSON(t, filepath.Join(external, "escape.txt"))+`,
		"content":"nope"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "escapes workspace") {
		t.Fatalf("external write result = %+v, want workspace escape rejection", res)
	}
	if _, err := os.Stat(filepath.Join(external, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("external file should not exist after rejected write")
	}
}

func TestDisplayPathPreservesExternalAbsolute(t *testing.T) {
	root, external := hostReadFixture(t)
	ws := Workspace{Root: root}
	abs := filepath.Join(external, "note.txt")
	if got := ws.DisplayPath(abs, abs); got != filepath.ToSlash(filepath.Clean(abs)) {
		t.Fatalf("DisplayPath(external) = %q, want %q", got, filepath.ToSlash(filepath.Clean(abs)))
	}
	local := filepath.Join(root, "local.txt")
	if got := ws.DisplayPath(local, "local.txt"); got != "local.txt" {
		t.Fatalf("DisplayPath(workspace) = %q, want local.txt", got)
	}
	if got := ws.DisplayPath("", "@runtime/wiki/x.md"); got != "@runtime/wiki/x.md" {
		t.Fatalf("DisplayPath(runtime) = %q, want @runtime/wiki/x.md", got)
	}
}
