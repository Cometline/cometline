package skills

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/skills"
)

func TestLoadSkillRestoresArchivedSelfImprovement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".cometmind", "skills", ".archive", "ship-checklist")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: ship-checklist\ndescription: Ship a reviewed change\nmetadata:\n  cometline:\n    origin: self-improvement\n---\n\n# Workflow\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var used string
	tool := LoadSkill{
		Skills: &skills.Registry{},
		Used: func(name string) {
			used = name
		},
	}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"name":"ship-checklist"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatal(res.Output)
	}
	if used != "ship-checklist" {
		t.Fatalf("used = %q", used)
	}
	live := filepath.Join(home, ".cometmind", "skills", "ship-checklist", "SKILL.md")
	if _, err := os.Stat(live); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("archive dir = %v", err)
	}
}
