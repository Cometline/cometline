package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cometline/cometmind/internal/skills"
)

// WriteSkillDraft creates or updates an Agent Skill draft under ~/.cometmind/skill-drafts.
type WriteSkillDraft struct{}

func (WriteSkillDraft) Spec() ToolSpec {
	return ToolSpec{
		Name: "write_skill_draft",
		Description: "Create or update a pending Agent Skill draft in ~/.cometmind/skill-drafts. " +
			"Provide the full SKILL.md contents with YAML frontmatter (name, description) and markdown body. " +
			"Before writing, the runtime compares name/description against managed skills (~/.cometmind/skills) and pending drafts. " +
			"If a likely overlap is found, the write is blocked unless force=true. " +
			"When blocked: tell the user about the overlaps and ask whether to proceed; only re-call with force=true after they agree. " +
			"To update an existing draft with the same name, set overwrite=true (force not required for that same-name draft). " +
			"Prefer updating or extending an overlapping live skill over creating a parallel near-duplicate.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "Skill draft directory name, e.g. commit-conventions"},
				"content": {"type": "string", "description": "Full SKILL.md file contents"},
				"overwrite": {"type": "boolean", "description": "Replace an existing draft with the same name (default false)"},
				"force": {"type": "boolean", "description": "Create anyway after the user agreed despite overlap warnings (default false)"}
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
		Force     bool    `json:"force"`
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

	description := skills.SkillMarkdownDescription(content)
	related, err := skills.FindRelatedManagedSkills(name, description)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	related = skills.FilterSelfDraftOverwrite(related, name, in.Overwrite)
	if !in.Force && skills.ShouldBlockOverlap(related) {
		msg := skills.FormatOverlapBlock(related) +
			"\n\nWrite blocked. Tell the user about these overlaps and ask whether to create a new draft anyway, " +
			"update an existing same-name draft (overwrite=true), or extend the existing live skill instead. " +
			"Only re-call write_skill_draft with force=true after the user explicitly agrees to create a near-duplicate."
		return Result{OK: false, Output: msg}, nil
	}

	if err := skills.WriteDraft(name, content, in.Overwrite); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	out := fmt.Sprintf("skill draft %s written to ~/.cometmind/skill-drafts/%s/SKILL.md", name, name)
	if in.Force && len(related) > 0 {
		out += "\n(force=true; overlaps were acknowledged)\n" + skills.FormatOverlapBlock(related)
	}
	return Result{OK: true, Output: out}, nil
}
