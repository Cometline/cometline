package tools

import (
	"context"
	"encoding/json"

	"github.com/cometline/cometmind/internal/skills"
)

// WriteSkillDraft creates or updates an Agent Skill draft under ~/.cometmind/skill-drafts.
type WriteSkillDraft struct{}

func (WriteSkillDraft) Spec() ToolSpec {
	return ToolSpec{
		Name:        "write_skill_draft",
		Description: "Create or update a pending Agent Skill draft in ~/.cometmind/skill-drafts. Provide the full SKILL.md contents with YAML frontmatter (name, description) and markdown body.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Skill draft directory name, e.g. commit-conventions"},
				"content": {"type": "string", "description": "Full SKILL.md file contents"},
				"overwrite": {"type": "boolean", "description": "Replace an existing draft (default false)"}
			},
			"required": ["name", "content"]
		}`),
	}
}

func (WriteSkillDraft) Execute(_ context.Context, input json.RawMessage) (Result, error) {
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
	if err := skills.WriteDraft(name, content, in.Overwrite); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: "skill draft " + name + " written to ~/.cometmind/skill-drafts/" + name + "/SKILL.md"}, nil
}
