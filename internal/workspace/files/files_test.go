package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "main.go"), "package main")
	mustWrite(t, filepath.Join(root, "README.md"), "# readme")
	mustWrite(t, filepath.Join(root, "vendor", "lib.go"), "package lib")
	mustWrite(t, filepath.Join(root, "internal", "helper.go"), "package internal")
	mustWrite(t, filepath.Join(root, ".hidden", "secret.go"), "package hidden")
	mustWrite(t, filepath.Join(root, "dist", "bundle.js"), "bundle")
	mustWrite(t, filepath.Join(root, "node_modules", "x", "index.js"), "x")
	mustWrite(t, filepath.Join(root, "src", "app.svelte"), "<div/>")

	ctx := context.Background()

	t.Run("lists all source files recursively", func(t *testing.T) {
		got, err := ListFiles(ctx, root, ListOptions{})
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		want := []string{"README.md", "internal/helper.go", "main.go", "src/app.svelte"}
		assertSlice(t, got.Files, want)
		if got.Truncated {
			t.Fatalf("did not expect truncation for %d files", len(want))
		}
	})

	t.Run("filters by query", func(t *testing.T) {
		got, err := ListFiles(ctx, root, ListOptions{Query: "go"})
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		want := []string{"internal/helper.go", "main.go"}
		assertSlice(t, got.Files, want)
	})

	t.Run("respects limit and flags truncation", func(t *testing.T) {
		got, err := ListFiles(ctx, root, ListOptions{Limit: 2})
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		if len(got.Files) != 2 {
			t.Fatalf("want 2 results, got %d: %v", len(got.Files), got.Files)
		}
		if !got.Truncated {
			t.Fatalf("expected truncated=true when more files exist than the limit")
		}
	})

	t.Run("exact limit is not truncated", func(t *testing.T) {
		dir := t.TempDir()
		mustWrite(t, filepath.Join(dir, "a.go"), "x")
		mustWrite(t, filepath.Join(dir, "b.go"), "x")
		got, err := ListFiles(ctx, dir, ListOptions{Limit: 2})
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		if len(got.Files) != 2 || got.Truncated {
			t.Fatalf("exactly-limit should not be truncated, got files=%d truncated=%v", len(got.Files), got.Truncated)
		}
	})

	t.Run("caps limit at max", func(t *testing.T) {
		// Create many files to ensure cap works.
		dir := t.TempDir()
		for i := 0; i < MaxLimit+10; i++ {
			mustWrite(t, filepath.Join(dir, filepath.FromSlash(formatFile(i))), "x")
		}
		got, err := ListFiles(ctx, dir, ListOptions{Limit: MaxLimit + 100})
		if err != nil {
			t.Fatalf("ListFiles error: %v", err)
		}
		if len(got.Files) != MaxLimit {
			t.Fatalf("want %d results, got %d", MaxLimit, len(got.Files))
		}
		if !got.Truncated {
			t.Fatalf("expected truncated=true when capped at MaxLimit")
		}
	})
}

func TestListFilesGitignore(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "keep.go"), "package main")
	mustWrite(t, filepath.Join(root, "ignore.log"), "log")
	mustWrite(t, filepath.Join(root, "build", "out.js"), "out")
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\nbuild/\n")

	ctx := context.Background()
	got, err := ListFiles(ctx, root, ListOptions{})
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	want := []string{"keep.go"}
	assertSlice(t, got.Files, want)
}

func TestListFilesNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := ListFiles(ctx, filepath.Join(t.TempDir(), "missing"), ListOptions{})
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func assertSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func formatFile(i int) string {
	return filepath.Join("dir", string(rune('a'+i%26))+"_"+string(rune('0'+i/26))+".txt")
}
