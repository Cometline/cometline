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

	if len(reg.Skills) != 2 {
		t.Fatalf("len(Skills) = %d, want 2; errors=%v", len(reg.Skills), reg.Errors)
	}
	alpha, ok := reg.Find("alpha")
	if !ok {
		t.Fatal("alpha not found")
	}
	if alpha.Description != "first" {
		t.Fatalf("alpha description = %q, want first", alpha.Description)
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
	if len(reg.Skills) != 0 {
		t.Fatalf("len(Skills) = %d, want 0", len(reg.Skills))
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
	if len(created) != 1 || created[0] != "alpha" || len(skipped) != 0 {
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

func TestExpandCreateSkillCommandIncludesRequest(t *testing.T) {
	out := ExpandCreateSkillCommand("commit message helper")
	if !strings.Contains(out, "write_skill") || !strings.Contains(out, "commit message helper") {
		t.Fatalf("unexpected expansion: %s", out)
	}
}
