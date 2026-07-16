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

func TestRuntimeWikiMountIsReadableAndWritable(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	root := t.TempDir()

	writer := WriteFile{Workspace: Workspace{Root: root}}
	res, err := writer.Execute(context.Background(), json.RawMessage(`{
		"path":"@runtime/wiki/entities/test.md",
		"content":"# hello wiki"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "@runtime/wiki/entities/test.md") {
		t.Fatalf("write result = %+v", res)
	}

	reader := ReadFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err = reader.Execute(context.Background(), json.RawMessage(`{"path":"@runtime/wiki/entities/test.md"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || !strings.Contains(res.Output, "# hello wiki") {
		t.Fatalf("read result = %+v", res)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "wiki", "entities", "test.md")); err != nil {
		t.Fatalf("wiki file missing: %v", err)
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
