package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewRegistryCapturesWorkspaceAndExposesSpecs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	r := NewRegistry(root)
	if got := r.Workspace().Root; got != root {
		t.Errorf("Workspace().Root = %q, want %q", got, root)
	}

	specs := r.CometSDK()
	if len(specs) != 5 {
		t.Fatalf("CometSDK() returned %d specs, want 5", len(specs))
	}
	wantNames := []string{"read_file", "write_file", "list_dir", "run_command", "web_fetch"}
	for i, name := range wantNames {
		if specs[i].Name != name {
			t.Errorf("spec[%d].Name = %q, want %q", i, specs[i].Name, name)
		}
	}

	res, err := r.Execute(context.Background(), "read_file", []byte(`{"path":"hello.txt"}`))
	if err != nil {
		t.Fatalf("Execute(read_file) error = %v", err)
	}
	if !res.OK {
		t.Fatalf("Execute(read_file) not OK: %s", res.Output)
	}
	if res.Output != "world" {
		t.Errorf("read_file output = %q, want %q", res.Output, "world")
	}
}
