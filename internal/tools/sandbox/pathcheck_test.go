package sandbox_test

import (
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
