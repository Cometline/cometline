package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/skills"
)

func TestNewSubagentRegistryExcludesWriteAndDelegateTools(t *testing.T) {
	r := NewSubagentRegistry(t.TempDir(), nil)
	excluded := []string{
		"write_file", "run_command", "write_skill", "write_skill_draft",
		"delegate_coding_task", "spawn_general_agent", "wait_subagents",
	}
	for _, name := range excluded {
		if r.Has(name) {
			t.Errorf("subagent registry should not include %q", name)
		}
	}
	included := []string{"read_file", "list_dir", "glob", "grep", "web_fetch"}
	for _, name := range included {
		if !r.Has(name) {
			t.Errorf("subagent registry missing %q", name)
		}
	}
}

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
	if len(specs) != 7 {
		t.Fatalf("CometSDK() returned %d specs, want 7", len(specs))
	}
	wantNames := []string{"read_file", "write_file", "list_dir", "glob", "grep", "run_command", "web_fetch"}
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

func TestNewRegistryIncludesSkillDraftToolsWhenSkillsEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg := skills.Discover("", skills.Config{Enabled: true})
	r := NewRegistry(t.TempDir(), RegistryOptions{Skills: &reg})
	for _, name := range []string{"write_skill_draft", "list_skill_drafts", "read_skill_draft", "promote_skill_draft"} {
		if !r.Has(name) {
			t.Fatalf("registry missing %q", name)
		}
	}
}
