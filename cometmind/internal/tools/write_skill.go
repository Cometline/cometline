package tools

import (
	"context"
	"encoding/json"

	"github.com/cometline/cometmind/internal/skills"
)

// WriteSkill creates or updates an Agent Skill under ~/.cometmind/skills.
type WriteSkill struct{}

func (WriteSkill) Spec() ToolSpec {
	return ToolSpec{
		Name:        "write_skill",
		Description: "Create or update an Agent Skill in ~/.cometmind/skills. Provide the full SKILL.md contents with YAML frontmatter (name, description) and markdown body.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Skill directory name, e.g. commit-conventions"},
				"content": {"type": "string", "description": "Full SKILL.md file contents"},
				"overwrite": {"type": "boolean", "description": "Replace an existing skill (default false)"}
			},
			"required": ["name", "content"]
		}`),
	}
}

func (WriteSkill) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Name      *string `json:"name"`
		Content   *string `json:"content"`
		Overwrite bool    `json:"overwrite"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	name, bad, ok := requiredTrimmedString(in.Name, "name")
	if !ok {
		return bad, nil
	}
	content, bad, ok := requiredString(in.Content, "content")
	if !ok {
		return bad, nil
	}
	if err := skills.WriteSkill(name, content, in.Overwrite); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: "skill " + name + " written to ~/.cometmind/skills/" + name + "/SKILL.md"}, nil
}
