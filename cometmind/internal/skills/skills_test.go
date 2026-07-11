package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedSkillsHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
}

func TestDiscoverFindsSkillsAndDeduplicatesByRootOrder(t *testing.T) {
	isolatedSkillsHome(t)
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeSkill(t, rootA, "alpha", "first")
	writeSkill(t, rootB, "alpha", "second")
	writeSkill(t, rootB, "beta", "second beta")

	reg := Discover("", Config{Enabled: true, Roots: []string{rootA, rootB}})

	alpha, ok := reg.Find("alpha")
	if !ok {
		t.Fatal("alpha not found")
	}
	if alpha.Description != "first" {
		t.Fatalf("alpha description = %q, want first", alpha.Description)
	}
}

func TestDiscoverIncludesBundledSetupSkill(t *testing.T) {
	isolatedSkillsHome(t)

	reg := Discover("", Config{Enabled: true})

	skill, ok := reg.Find("setup-cometline")
	if !ok {
		t.Fatalf("setup-cometline not found; errors=%v", reg.Errors)
	}
	if skill.Internal {
		t.Fatal("setup-cometline should be visible to users")
	}
	if !strings.Contains(reg.PromptIndex(), "setup-cometline") {
		t.Fatal("prompt index should include setup-cometline")
	}
}

func TestDiscoverIncludesGlobalAgentSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	globalRoot := filepath.Join(home, ".agents", "skills")
	writeSkill(t, globalRoot, "global-finance", "global finance skill")

	reg := Discover("", Config{Enabled: true, IncludeOpenCode: false, IncludeClaude: false})
	skill, ok := reg.Find("global-finance")
	if !ok {
		t.Fatalf("global Agent Skill not found; errors=%v", reg.Errors)
	}
	wantPath, err := filepath.EvalSymlinks(filepath.Join(globalRoot, "global-finance"))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Path != wantPath {
		t.Fatalf("skill path = %q, want %q", skill.Path, wantPath)
	}
}

func TestDiscoverLetsUserSkillOverrideBundledSkill(t *testing.T) {
	isolatedSkillsHome(t)
	root := t.TempDir()
	writeSkill(t, root, "setup-cometline", "custom setup guidance")

	reg := Discover("", Config{Enabled: true, Roots: []string{root}})

	skill, ok := reg.Find("setup-cometline")
	if !ok {
		t.Fatal("setup-cometline not found")
	}
	if skill.Description != "custom setup guidance" {
		t.Fatalf("description = %q, want custom setup guidance", skill.Description)
	}
}

func TestDiscoverSkipsMalformedSkills(t *testing.T) {
	isolatedSkillsHome(t)
	root := t.TempDir()
	dir := filepath.Join(root, "bad")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Missing frontmatter"), 0o600); err != nil {
		t.Fatal(err)
	}

	reg := Discover("", Config{Enabled: true, Roots: []string{root}})
	if _, ok := reg.Find("bad"); ok {
		t.Fatal("malformed skill should not be discovered")
	}
	if len(reg.Errors) == 0 {
		t.Fatal("expected parse error")
	}
}

func TestSyncMirrorCreatesSymlinks(t *testing.T) {
	isolatedSkillsHome(t)
	root := t.TempDir()
	mirror := t.TempDir()
	writeSkill(t, root, "alpha", "first")
	reg := Discover("", Config{Enabled: true, Roots: []string{root}})

	created, skipped, err := reg.SyncMirror(mirror)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(created, "alpha") || len(skipped) != 0 {
		t.Fatalf("created=%v skipped=%v", created, skipped)
	}
	info, err := os.Lstat(filepath.Join(mirror, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("mirror entry is not a symlink")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func writeSkill(t *testing.T, root, name, desc string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReadSkillFileRejectsPathTraversal(t *testing.T) {
	isolatedSkillsHome(t)
	root := t.TempDir()
	writeSkill(t, root, "alpha", "first")
	reg := Discover("", Config{Enabled: true, Roots: []string{root}})

	if _, _, err := reg.ReadSkillFile("alpha", "../secret.txt"); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestCapabilitiesManagedDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mirror, err := MirrorRoot()
	if err != nil {
		t.Fatal(err)
	}
	writeSkill(t, mirror, "alpha", "managed skill")
	reg := Discover("", Config{Enabled: true})
	skill, ok := reg.Find("alpha")
	if !ok {
		t.Fatal("alpha not found")
	}
	caps, err := SkillCapabilities(skill)
	if err != nil {
		t.Fatal(err)
	}
	if !caps.CanExport || !caps.CanDelete || caps.IsSymlink {
		t.Fatalf("caps = %+v, want export+delete without symlink", caps)
	}
}

func TestCapabilitiesSymlinkMirrorNotDeletable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	mirror, err := MirrorRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mirror, 0o700); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	writeSkill(t, external, "alpha", "external skill")
	if err := os.Symlink(filepath.Join(external, "alpha"), filepath.Join(mirror, "alpha")); err != nil {
		t.Fatal(err)
	}
	reg := Discover("", Config{Enabled: true})
	skill, ok := reg.Find("alpha")
	if !ok {
		t.Fatal("alpha not found")
	}
	caps, err := SkillCapabilities(skill)
	if err != nil {
		t.Fatal(err)
	}
	if !caps.CanExport || caps.CanDelete || !caps.IsSymlink {
		t.Fatalf("caps = %+v, want export only with symlink", caps)
	}
}

func TestWriteSkillAndDeleteManagedSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := "---\nname: new-skill\ndescription: test skill\n---\n\n# New\n"
	if err := WriteSkill("new-skill", content, false); err != nil {
		t.Fatal(err)
	}
	reg := Discover("", Config{Enabled: true})
	skill, ok := reg.Find("new-skill")
	if !ok {
		t.Fatal("new-skill not found after write")
	}
	data, err := ExportSkill(skill)
	if err != nil || len(data) == 0 {
		t.Fatalf("ExportSkill() = %d bytes, err=%v", len(data), err)
	}
	if err := DeleteManagedSkill(skill); err != nil {
		t.Fatal(err)
	}
	mirror, err := MirrorRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(mirror, "new-skill")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("skill dir still exists: %v", err)
	}
}

func TestDraftPromoteRejectRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := "---\nname: draft-skill\ndescription: Draft skill\n---\n\nUse this draft."
	if err := WriteDraft("draft-skill", content, false); err != nil {
		t.Fatalf("WriteDraft() error = %v", err)
	}
	reg := Discover("", Config{Enabled: true})
	if _, ok := reg.Find("draft-skill"); ok {
		t.Fatalf("draft should not be discoverable before promotion")
	}
	drafts, err := ListDrafts()
	if err != nil {
		t.Fatalf("ListDrafts() error = %v", err)
	}
	if len(drafts) != 1 || drafts[0].Name != "draft-skill" {
		t.Fatalf("ListDrafts() = %+v", drafts)
	}
	if err := PromoteDraft("draft-skill"); err != nil {
		t.Fatalf("PromoteDraft() error = %v", err)
	}
	reg = Discover("", Config{Enabled: true})
	if _, ok := reg.Find("draft-skill"); !ok {
		t.Fatalf("promoted skill should be discoverable")
	}
	drafts, err = ListDrafts()
	if err != nil {
		t.Fatalf("ListDrafts() after promote error = %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("draft should be removed after promotion: %+v", drafts)
	}

	rejected := strings.ReplaceAll(content, "draft-skill", "rejected-skill")
	if err := WriteDraft("rejected-skill", rejected, false); err != nil {
		t.Fatalf("WriteDraft(rejected) error = %v", err)
	}
	if err := RejectDraft("rejected-skill"); err != nil {
		t.Fatalf("RejectDraft() error = %v", err)
	}
	drafts, err = ListDrafts()
	if err != nil {
		t.Fatalf("ListDrafts() after reject error = %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("draft should be removed after rejection: %+v", drafts)
	}
}

func TestExpandCreateSkillCommandIncludesRequest(t *testing.T) {
	out := ExpandCreateSkillCommand("commit message helper")
	if !strings.Contains(out, "write_skill_draft") || !strings.Contains(out, "commit message helper") {
		t.Fatalf("unexpected expansion: %s", out)
	}
}
