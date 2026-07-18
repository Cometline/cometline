package links

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTargets(t *testing.T) {
	got := ExtractTargets("See [[Foo]] and [[Bar|label]] plus [[Baz#h]].")
	if len(got) != 3 {
		t.Fatalf("got %v", got)
	}
}

func TestResolveTargetPrefersEntities(t *testing.T) {
	files := []string{"entities/dup.md", "concepts/dup.md"}
	if got := ResolveTarget("dup", files); got != "entities/dup.md" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildBacklinkIndex(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("entities/a.md", "links to [[b]] and [[2026-07-16-paper.html]]")
	mustWrite("concepts/b.md", "no outbound")
	mustWrite("raw/2026-07-16-paper.html", "<html></html>")
	mustWrite("raw/skip.md", "[[a]]")

	index, err := BuildBacklinkIndex(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	backlinks := BacklinksFor(index, "concepts/b.md")
	if len(backlinks) != 1 || backlinks[0] != "entities/a.md" {
		t.Fatalf("backlinks=%v", backlinks)
	}
	htmlBacklinks := BacklinksFor(index, "raw/2026-07-16-paper.html")
	if len(htmlBacklinks) != 1 || htmlBacklinks[0] != "entities/a.md" {
		t.Fatalf("html backlinks=%v", htmlBacklinks)
	}
	if got := BacklinksFor(index, "entities/a.md"); len(got) != 0 {
		t.Fatalf("expected no backlinks for a, got %v", got)
	}
}

func TestResolveHTMLTarget(t *testing.T) {
	files := []string{"entities/a.md", "raw/2026-07-16-paper.html"}
	if got := ResolveTarget("2026-07-16-paper.html", files); got != "raw/2026-07-16-paper.html" {
		t.Fatalf("with ext: got %q", got)
	}
	if got := ResolveTarget("2026-07-16-paper", files); got != "raw/2026-07-16-paper.html" {
		t.Fatalf("stem: got %q", got)
	}
	targets := ExtractTargets("links to [[b]] and [[2026-07-16-paper.html]]")
	if len(targets) != 2 || targets[0] != "b" || targets[1] != "2026-07-16-paper" {
		t.Fatalf("targets=%v", targets)
	}
}
