package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/skills"
)

// LoadSkill loads a discovered Agent Skill's SKILL.md instructions.
type LoadSkill struct{ Skills *skills.Registry }

func (LoadSkill) Spec() ToolSpec {
	return ToolSpec{
		Name:        "load_skill",
		Description: "Load the full SKILL.md instructions for an available skill by name. Use this before following a skill from the Available Skills index.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Skill name, e.g. systems-analyst"}},"required":["name"]}`),
	}
}

func (l LoadSkill) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	if l.Skills == nil {
		return Result{OK: false, Output: "skills are not configured"}, nil
	}
	var in struct {
		Name *string `json:"name"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	name, bad, ok := requiredTrimmedString(in.Name, "name")
	if !ok {
		return bad, nil
	}
	skill, markdown, err := l.Skills.SkillMarkdown(name)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	files := skillFiles(skill.Path)
	out := fmt.Sprintf("name: %s\ndescription: %s\nsource: %s\nfiles: %s\n\n%s",
		skill.Name, skill.Description, skill.Source, strings.Join(files, ", "), markdown)
	return Result{OK: true, Output: out}, nil
}

// ReadSkillFile reads auxiliary files within a discovered skill directory.
type ReadSkillFile struct{ Skills *skills.Registry }

func (ReadSkillFile) Spec() ToolSpec {
	return ToolSpec{
		Name:        "read_skill_file",
		Description: "Read an auxiliary file inside a loaded skill directory, such as references/foo.md. Paths cannot escape the skill.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Skill name"},"path":{"type":"string","description":"Path inside the skill directory"}},"required":["name","path"]}`),
	}
}

func (r ReadSkillFile) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	if r.Skills == nil {
		return Result{OK: false, Output: "skills are not configured"}, nil
	}
	var in struct {
		Name *string `json:"name"`
		Path *string `json:"path"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	name, bad, ok := requiredTrimmedString(in.Name, "name")
	if !ok {
		return bad, nil
	}
	path, bad, ok := requiredTrimmedString(in.Path, "path")
	if !ok {
		return bad, nil
	}
	skill, content, err := r.Skills.ReadSkillFile(name, path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("name: %s\npath: %s\n\n%s", skill.Name, path, content)}, nil
}

func skillFiles(root string) []string {
	files := []string{"SKILL.md"}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path == filepath.Join(root, "SKILL.md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err == nil {
			files = append(files, rel)
		}
		return nil
	})
	return files
}
