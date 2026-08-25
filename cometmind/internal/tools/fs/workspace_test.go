package fs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveReadableRejectsTerminalEnv(t *testing.T) {
	root := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", root)
	envFile := filepath.Join(root, "terminal-env", "sess1", "environ")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("SECRET=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := Workspace{Root: t.TempDir()}
	if _, err := ws.ResolveReadable(envFile); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("ResolveReadable err = %v, want private", err)
	}
	if _, _, err := ws.ResolveSearchRoot(envFile); err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("ResolveSearchRoot err = %v, want private", err)
	}
}
