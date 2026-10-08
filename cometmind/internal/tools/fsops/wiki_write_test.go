package fsops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWikiWriteRejectsPathsOutsideTheWiki(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	tool := WikiWriteFile{Workspace: Workspace{Root: root}}
	outside := filepath.Join(root, "main.go")
	res, err := tool.Execute(context.Background(), []byte(`{"path":"main.go","content":"package main\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("write outside the wiki succeeded")
	}
	if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
		t.Fatal("outside file was created")
	}

	created, err := tool.Execute(context.Background(), []byte(`{"path":"@runtime/wiki/concepts/go.md","content":"# Go\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !created.OK {
		t.Fatalf("wiki write failed: %s", created.Output)
	}

	raw, err := tool.Execute(context.Background(), []byte(`{"path":"@runtime/wiki/raw/source.md","content":"# Source\n"}`))
	if err != nil || !raw.OK {
		t.Fatalf("new raw file: ok=%v err=%v output=%s", raw.OK, err, raw.Output)
	}
	again, err := tool.Execute(context.Background(), []byte(`{"path":"@runtime/wiki/raw/source.md","content":"# Changed\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if again.OK {
		t.Fatal("existing raw file was edited")
	}
}

func TestWikiEditRejectsWorkspacePath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tool := WikiEditFile{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(context.Background(), []byte(`{"path":"main.go","old_string":"a","new_string":"b"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("edit outside the wiki succeeded")
	}
}
