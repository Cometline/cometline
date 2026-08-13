package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
)

func TestNewSubagentRegistryExcludesWriteAndDelegateTools(t *testing.T) {
	r := NewSubagentRegistry(t.TempDir(), nil, SubagentModeResearch)
	excluded := []string{
		"edit_file", "write_file", "run_command", "write_skill", "write_skill_draft",
		"delegate_coding_task", "spawn_general_agent", "wait_subagents",
		"list_settings", "get_settings", "patch_settings",
	}
	for _, name := range excluded {
		if r.Has(name) {
			t.Errorf("research subagent registry should not include %q", name)
		}
	}
	included := []string{"read_file", "list_dir", "glob", "grep", "web_fetch", "web_search"}
	for _, name := range included {
		if !r.Has(name) {
			t.Errorf("subagent registry missing %q", name)
		}
	}
}

func TestNewSubagentRegistryCodingIncludesEditTools(t *testing.T) {
	r := NewSubagentRegistry(t.TempDir(), nil, SubagentModeCoding)
	for _, name := range []string{"read_file", "edit_file", "write_file", "run_command", "grep"} {
		if !r.Has(name) {
			t.Errorf("coding subagent registry missing %q", name)
		}
	}
	for _, name := range []string{"delegate_coding_task", "spawn_general_agent", "list_settings", "patch_settings"} {
		if r.Has(name) {
			t.Errorf("coding subagent registry should not include %q", name)
		}
	}
}

func TestNewRegistryCapturesWorkspaceAndExposesSpecs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	r := NewRegistry(root)
	if got := r.workspace.Root; got != root {
		t.Errorf("workspace.Root = %q, want %q", got, root)
	}

	specs := r.CometSDK()
	if len(specs) != 16 {
		t.Fatalf("CometSDK() returned %d specs, want 16", len(specs))
	}
	wantNames := []string{
		"read_file", "edit_file", "write_file", "list_dir", "glob", "grep", "run_command",
		"web_fetch", "web_search", "present_image", "present_image_url", "capture_screenshot", "list_capture_targets",
		"list_settings", "get_settings", "patch_settings",
	}
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
	if res.Output != "1: world" {
		t.Errorf("read_file output = %q, want %q", res.Output, "1: world")
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

func TestNewRegistryGatesDelegateOnHarnessAvailability(t *testing.T) {
	for _, harness := range []acp.Harness{
		acp.HarnessOpenCode,
		acp.HarnessClaude,
		acp.HarnessCodex,
	} {
		t.Run(string(harness), func(t *testing.T) {
			cfg := acp.DefaultHarnessConfig(harness)
			cfg.Enabled = true
			r := NewRegistry(t.TempDir(), RegistryOptions{
				Sessions: &session.Service{},
				ACP:      cfg,
			})
			if got, want := r.Has("delegate_coding_task"), cfg.CommandAvailable(); got != want {
				t.Fatalf("delegate_coding_task registered = %v, want %v", got, want)
			}
		})
	}
}

func TestNewRegistryOmitsDelegateWhenDisabled(t *testing.T) {
	cfg := acp.DefaultHarnessConfig(acp.HarnessOpenCode)
	cfg.Enabled = false
	r := NewRegistry(t.TempDir(), RegistryOptions{
		Sessions: &session.Service{},
		ACP:      cfg,
	})
	if r.Has("delegate_coding_task") {
		t.Fatal("delegate_coding_task should not register when ACP.Enabled is false")
	}
}
