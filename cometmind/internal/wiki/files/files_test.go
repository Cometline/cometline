package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListDocumentFiles(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{
		"index.md",
		"entities/foo.md",
		"raw/2026-01-01-note.md",
		"raw/2026-01-01-paper.html",
		"paper.pdf",
		"draft.markdown",
		"notes.txt",
		".hidden.md",
		".private/draft.txt",
	} {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ListDocumentFiles(context.Background(), dir, ListOptions{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		".hidden.md",
		".private/",
		"draft.markdown",
		"entities/",
		"entities/foo.md",
		"index.md",
		"paper.pdf",
		"raw/",
		"raw/2026-01-01-note.md",
		"raw/2026-01-01-paper.html",
	}
	if len(result.Files) != len(want) {
		t.Fatalf("files = %v want %v", result.Files, want)
	}
	for i, path := range want {
		if result.Files[i] != path {
			t.Fatalf("files = %v want %v", result.Files, want)
		}
	}
}

func TestListDirectoryFiltersWikiDocuments(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"guide.md", "paper.PDF", "page.html", "notes.txt", "assets/image.png"} {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := ListDirectory(context.Background(), dir, "", ListOptions{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"assets/", "guide.md", "page.html", "paper.PDF"}
	if len(result.Files) != len(want) {
		t.Fatalf("files = %v want %v", result.Files, want)
	}
	for i := range want {
		if result.Files[i] != want[i] {
			t.Fatalf("files = %v want %v", result.Files, want)
		}
	}
}

func TestListDocumentFilesMissingDir(t *testing.T) {
	result, err := ListDocumentFiles(context.Background(), filepath.Join(t.TempDir(), "missing"), ListOptions{})
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
