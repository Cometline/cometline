package sandbox_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

func TestResolveWorkspacePath_AllowsChild(t *testing.T) {
	root := t.TempDir()
	p, err := sandbox.ResolveWorkspacePath(root, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "a") {
		t.Fatalf("expected path to contain a: %q", p)
	}
}

func TestResolveWorkspacePath_RejectsEscape(t *testing.T) {
	root := t.TempDir()
	_, err := sandbox.ResolveWorkspacePath(root, "../outside")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
}

func TestResolveWorkspacePath_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, err := sandbox.ResolveWorkspacePath(root, "escape/secret.txt")
	if err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestResolveWorkspacePath_AllowsSymlinkInsideWorkspace(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(root, "alias")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := sandbox.ResolveWorkspacePath(root, "alias/note.txt")
	if err != nil {
		t.Fatalf("ResolveWorkspacePath error: %v", err)
	}
	resolvedRealDir, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(realDir) error: %v", err)
	}
	if got, want := p, filepath.Join(resolvedRealDir, "note.txt"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}
