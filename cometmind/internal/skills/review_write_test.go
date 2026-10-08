package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReviewSkillCreateInjectsOrigin(t *testing.T) {
	root := reviewHome(t)
	got, err := WriteReviewSkill("ship-checklist", reviewMarkdown("ship-checklist", "Ship a reviewed change", ""), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "created" {
		t.Fatalf("action = %q", got.Action)
	}
	skill, err := ReadSkill(filepath.Join(root, "ship-checklist"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsSelfImprovement(skill) {
		t.Fatalf("origin = %q", skill.Origin)
	}
}

func TestWriteReviewSkillRejectsOtherOrigin(t *testing.T) {
	reviewHome(t)
	_, err := WriteReviewSkill("ship-checklist", reviewMarkdown("ship-checklist", "Ship a reviewed change", "user"), false)
	if err == nil || !strings.Contains(err.Error(), "cannot set origin") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteReviewSkillOverwriteKeepsSelfImprovement(t *testing.T) {
	root := reviewHome(t)
	if _, err := WriteReviewSkill("ship-checklist", reviewMarkdown("ship-checklist", "Ship a reviewed change", ""), false); err != nil {
		t.Fatal(err)
	}
	got, err := WriteReviewSkill("ship-checklist", reviewMarkdown("ship-checklist", "Ship a reviewed change with tests", ""), true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "updated" {
		t.Fatalf("action = %q", got.Action)
	}
	raw, err := os.ReadFile(filepath.Join(root, "ship-checklist", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "origin: self-improvement") {
		t.Fatalf("frontmatter lost origin:\n%s", raw)
	}
	if !strings.Contains(string(raw), "with tests") {
		t.Fatalf("description was not updated:\n%s", raw)
	}
}

func TestWriteReviewSkillRejectsUserSkill(t *testing.T) {
	root := reviewHome(t)
	content := reviewMarkdown("user-workflow", "A workflow the user wrote", "")
	if err := WriteSkill("user-workflow", content, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "user-workflow", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = WriteReviewSkill("user-workflow", reviewMarkdown("user-workflow", "A workflow the user wrote", ""), true)
	if err == nil || !strings.Contains(err.Error(), "not self-improvement") {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "user-workflow", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("user skill changed")
	}
}

func TestWriteReviewSkillOverlapRequiresOverwrite(t *testing.T) {
	reviewHome(t)
	if _, err := WriteReviewSkill("ship-checklist", reviewMarkdown("ship-checklist", "Ship a reviewed change", ""), false); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReviewSkill("ship-checklist-extra", reviewMarkdown("ship-checklist-extra", "Ship a reviewed change", ""), false)
	if err == nil {
		t.Fatal("expected overlap rejection")
	}
	if !strings.Contains(err.Error(), "overwrite") {
		t.Fatalf("error = %v", err)
	}
	mirror, err := MirrorRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(mirror, "ship-checklist-extra")); !os.IsNotExist(statErr) {
		t.Fatal("overlap created a second skill")
	}
}

func TestWriteReviewSkillUserOverlapWritesNothing(t *testing.T) {
	reviewHome(t)
	if err := WriteSkill("user-workflow", reviewMarkdown("user-workflow", "Prepare a release branch", ""), false); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReviewSkill("user-workflow-copy", reviewMarkdown("user-workflow-copy", "Prepare a release branch", ""), false)
	if err == nil || !strings.Contains(err.Error(), "stop without writing") {
		t.Fatalf("error = %v", err)
	}
	mirror, err := MirrorRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(mirror, "user-workflow-copy")); !os.IsNotExist(statErr) {
		t.Fatal("user overlap still wrote a skill")
	}
}

func TestWriteSkillPreservesSelfImprovementOrigin(t *testing.T) {
	root := reviewHome(t)
	if _, err := WriteReviewSkill("ship-checklist", reviewMarkdown("ship-checklist", "Ship a reviewed change", ""), false); err != nil {
		t.Fatal(err)
	}
	edited := reviewMarkdown("ship-checklist", "Ship a reviewed change carefully", "")
	if err := WriteSkill("ship-checklist", edited, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "ship-checklist", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "origin: self-improvement") {
		t.Fatalf("parent edit stripped origin:\n%s", raw)
	}
	if !strings.Contains(string(raw), "carefully") {
		t.Fatalf("parent edit did not apply:\n%s", raw)
	}
}

func TestWriteSkillStillEditsUserSkills(t *testing.T) {
	root := reviewHome(t)
	if err := WriteSkill("user-workflow", reviewMarkdown("user-workflow", "A workflow the user wrote", ""), false); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkill("user-workflow", reviewMarkdown("user-workflow", "A workflow the user revised", ""), true); err != nil {
		t.Fatal(err)
	}
	skill, err := ReadSkill(filepath.Join(root, "user-workflow"))
	if err != nil {
		t.Fatal(err)
	}
	if skill.Origin != "" {
		t.Fatalf("origin = %q", skill.Origin)
	}
	if skill.Description != "A workflow the user revised" {
		t.Fatalf("description = %q", skill.Description)
	}
}

func TestWriteReviewSkillRejectsSymlink(t *testing.T) {
	root := reviewHome(t)
	real := filepath.Join(root, "real-skill")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "SKILL.md"), []byte(reviewMarkdown("real-skill", "Linked skill", OriginSelfImprovement)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, "linked-skill")); err != nil {
		t.Fatal(err)
	}
	_, err := WriteReviewSkill("linked-skill", reviewMarkdown("linked-skill", "Linked skill", ""), true)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v", err)
	}
}

func reviewHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".cometmind", "skills")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func reviewMarkdown(name, description, origin string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: " + description + "\n")
	if origin != "" {
		b.WriteString("metadata:\n  cometline:\n    origin: " + origin + "\n")
	}
	b.WriteString("---\n\n# Workflow\n\nDo the steps.\n")
	return b.String()
}
