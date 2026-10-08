package skills

import (
	"context"
	"encoding/json"

	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// ReviewWriteSkill is the review fork's write_skill. It cannot promote drafts
// and cannot edit a skill that is not origin self-improvement.
type ReviewWriteSkill struct{}

func (ReviewWriteSkill) Spec() ToolSpec {
	return ToolSpec{
		Name:        "write_skill",
		Description: "Create or update a self-improvement Agent Skill in ~/.cometmind/skills. On create the runtime sets metadata.cometline.origin to self-improvement. Overwrite only a skill that already has that origin. If an existing self-improvement skill overlaps, call again with overwrite=true for that skill. If the overlap is any other skill, stop without writing. There is no force flag.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Skill directory name"},
				"content": {"type": "string", "description": "Full SKILL.md file contents"},
				"overwrite": {"type": "boolean", "description": "Replace an existing self-improvement skill"}
			},
			"required": ["name", "content"]
		}`),
	}
}

func (ReviewWriteSkill) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Name      *string `json:"name"`
		Content   *string `json:"content"`
		Overwrite bool    `json:"overwrite"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	name, bad, ok := toolkit.RequiredTrimmedString(in.Name, "name")
	if !ok {
		return bad, nil
	}
	content, bad, ok := toolkit.RequiredString(in.Content, "content")
	if !ok {
		return bad, nil
	}
	written, err := skills.WriteReviewSkill(name, content, in.Overwrite)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: "skill " + written.Name + " " + written.Action}, nil
}
