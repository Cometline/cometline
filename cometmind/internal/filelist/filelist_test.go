package filelist

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestListIncludesRootEntriesBeforeDeepDescendants(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"alpha/one.txt",
		"alpha/two.txt",
		"alpha/three.txt",
		"zebra/only.txt",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := List(context.Background(), root, Options{Limit: 3})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"alpha/", "alpha/one.txt", "zebra/"}
	if !reflect.DeepEqual(result.Files, want) {
		t.Fatalf("files = %v, want %v", result.Files, want)
	}
	if !result.Truncated {
		t.Fatal("Truncated = false, want true")
	}
}

func TestListDirectoryReturnsOnlyDirectChildren(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"README.md",
		"src/app.ts",
		"src/lib/tree.ts",
		"node_modules/ignored.js",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	rootEntries, err := ListDirectory(context.Background(), root, "", Options{
		SkipDirectoryNames: map[string]bool{"node_modules": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"README.md", "src/"}; !reflect.DeepEqual(rootEntries.Files, want) {
		t.Fatalf("root files = %v, want %v", rootEntries.Files, want)
	}

	srcEntries, err := ListDirectory(context.Background(), root, "src", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"src/app.ts", "src/lib/"}; !reflect.DeepEqual(srcEntries.Files, want) {
		t.Fatalf("src files = %v, want %v", srcEntries.Files, want)
	}
}

func TestListDirectoryRejectsPathsOutsideRoot(t *testing.T) {
	_, err := ListDirectory(context.Background(), t.TempDir(), "../outside", Options{})
	if err == nil {
		t.Fatal("expected path outside root to be rejected")
	}
}
