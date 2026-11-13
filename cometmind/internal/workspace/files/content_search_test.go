package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchContentFindsLiteralText(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(root, "a.txt"), "alpha\nneedle here\n")
	mustWriteSearchFile(t, filepath.Join(root, "b.txt"), "other\n")

	got, err := SearchContent(context.Background(), root, "needle", 10)
	if err != nil {
		t.Fatalf("SearchContent() error = %v", err)
	}
	if len(got.Matches) != 1 || got.Matches[0].Path != "a.txt" || got.Matches[0].Line != 2 {
		t.Fatalf("matches = %+v", got.Matches)
	}
}

func TestSearchContentSkipsGitignore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(root, ".gitignore"), "skip.txt\n")
	mustWriteSearchFile(t, filepath.Join(root, "skip.txt"), "needle\n")
	mustWriteSearchFile(t, filepath.Join(root, "keep.txt"), "needle\n")

	got, err := SearchContent(context.Background(), root, "needle", 10)
	if err != nil {
		t.Fatalf("SearchContent() error = %v", err)
	}
	if len(got.Matches) != 1 || got.Matches[0].Path != "keep.txt" {
		t.Fatalf("matches = %+v", got.Matches)
	}
}

func TestSearchContentCapsResults(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteSearchFile(t, filepath.Join(root, "many.txt"), "needle\nneedle\nneedle\n")

	got, err := SearchContent(context.Background(), root, "needle", 2)
	if err != nil {
		t.Fatalf("SearchContent() error = %v", err)
	}
	if len(got.Matches) != 2 || !got.Truncated {
		t.Fatalf("result = %+v", got)
	}
}

func mustWriteSearchFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
