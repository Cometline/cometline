package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{
		"index.md",
		"entities/foo.md",
		"raw/2026-01-01-note.md",
		"notes.txt",
		".hidden.md",
	} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ListMarkdownFiles(context.Background(), dir, ListOptions{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 3 {
		t.Fatalf("files = %v want 3 markdown paths", result.Files)
	}
}

func TestListMarkdownFilesMissingDir(t *testing.T) {
	result, err := ListMarkdownFiles(context.Background(), filepath.Join(t.TempDir(), "missing"), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected empty list, got %v", result.Files)
	}
}

func TestIsWriteProtected(t *testing.T) {
	cases := map[string]bool{
		"index.md":        false,
		"entities/foo.md": false,
		"raw/note.md":     true,
		"Raw/note.md":     true,
		"raw":             true,
		"Raw":             true,
		"WIKI.md":         true,
		"wiki.md":         true,
		"lint-report.md":  false,
	}
	for path, want := range cases {
		if got := IsWriteProtected(path); got != want {
			t.Fatalf("IsWriteProtected(%q) = %v want %v", path, got, want)
		}
	}
}
