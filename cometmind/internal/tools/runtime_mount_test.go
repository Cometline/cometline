package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeMountsShareFilesAcrossToolInstances(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	root := t.TempDir()

	writer := WriteFile{Workspace: Workspace{Root: root}}
	res, err := writer.Execute(context.Background(), json.RawMessage(`{
		"path":"@runtime/tmp/shared.txt",
		"content":"shared across sessions"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "@runtime/tmp/shared.txt") {
		t.Fatalf("write result = %+v", res)
	}

	reader := ReadFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err = reader.Execute(context.Background(), json.RawMessage(`{"path":"@runtime/tmp/shared.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "shared across sessions") {
		t.Fatalf("read result = %+v", res)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "agent-tmp", "shared.txt")); err != nil {
		t.Fatalf("shared file missing: %v", err)
	}
}

func TestRuntimeToolOutputIsReadableButNotWritable(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	root := t.TempDir()
	outputDir := filepath.Join(dataDir, "tool-output")
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "result.txt"), []byte("complete output"), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := ReadFile{Workspace: Workspace{Root: root}}
	res, err := reader.Execute(context.Background(), json.RawMessage(`{"path":"@runtime/tool-output/result.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "complete output") {
		t.Fatalf("read result = %+v", res)
	}

	writer := WriteFile{Workspace: Workspace{Root: root}}
	res, err = writer.Execute(context.Background(), json.RawMessage(`{
		"path":"@runtime/tool-output/forbidden.txt",
		"content":"must not write"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "read-only") {
		t.Fatalf("write result = %+v", res)
	}
}

func TestRuntimeWikiSupportsReadWriteAndSearch(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	root := t.TempDir()

	writer := WriteFile{Workspace: Workspace{Root: root}}
	res, err := writer.Execute(context.Background(), json.RawMessage(`{
		"path":"@runtime/wiki/concepts/runtime-mounts.md",
		"content":"# Runtime mounts\n\nA shared knowledge wiki.\n"
	}`))
	if err != nil || !res.OK {
		t.Fatalf("write wiki = %+v, %v", res, err)
	}

	reader := ReadFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err = reader.Execute(context.Background(), json.RawMessage(`{"path":"@runtime/wiki/concepts/runtime-mounts.md"}`))
	if err != nil || !res.OK || !strings.Contains(res.Output, "shared knowledge wiki") {
		t.Fatalf("read wiki = %+v, %v", res, err)
	}

	editor := EditFile{Workspace: Workspace{Root: root}}
	res, err = editor.Execute(context.Background(), json.RawMessage(`{
		"path":"@runtime/wiki/concepts/runtime-mounts.md",
		"old_string":"shared knowledge wiki",
		"new_string":"persistent knowledge wiki"
	}`))
	if err != nil || !res.OK {
		t.Fatalf("edit wiki = %+v, %v", res, err)
	}

	glob := Glob{Workspace: Workspace{Root: root}}
	res, err = glob.Execute(context.Background(), json.RawMessage(`{"pattern":"**/*.md","path":"@runtime/wiki"}`))
	if err != nil || !res.OK || !strings.Contains(res.Output, "@runtime/wiki/concepts/runtime-mounts.md") {
		t.Fatalf("glob wiki = %+v, %v", res, err)
	}

	grep := Grep{Workspace: Workspace{Root: root}}
	res, err = grep.Execute(context.Background(), json.RawMessage(`{"pattern":"persistent knowledge","path":"@runtime/wiki","literal_text":true}`))
	if err != nil || !res.OK || !strings.Contains(res.Output, "@runtime/wiki/concepts/runtime-mounts.md:3:") {
		t.Fatalf("grep wiki = %+v, %v", res, err)
	}
}

func TestRuntimeMountDoesNotExposePrivateDataFiles(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "cometline-settings.json"), []byte(`{"apiKey":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	reader := ReadFile{Workspace: Workspace{Root: root}}
	for _, path := range []string{"@runtime/cometline-settings.json", "@runtime/../cometline-settings.json"} {
		res, err := reader.Execute(context.Background(), mustRuntimeJSON(t, path))
		if err != nil {
			t.Fatal(err)
		}
		if res.OK {
			t.Fatalf("private path %q was readable: %+v", path, res)
		}
	}
}

func TestListRuntimeMounts(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	tool := ListDir{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"@runtime"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Output != "tool-output/\ntmp/\nwiki/\n" {
		t.Fatalf("result = %+v", res)
	}
}

func mustRuntimeJSON(t *testing.T, path string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	return b
}
