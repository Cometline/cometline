package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/skills"
)

// ListSkillDrafts lists pending Agent Skill drafts waiting for review.
type ListSkillDrafts struct{}

func (ListSkillDrafts) Spec() ToolSpec {
	return ToolSpec{
		Name:        "list_skill_drafts",
		Description: "List pending Agent Skill drafts that can be reviewed, edited, rejected, or promoted.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (ListSkillDrafts) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	drafts, err := skills.ListDrafts()
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if len(drafts) == 0 {
		return Result{OK: true, Output: "No pending skill drafts."}, nil
	}
	var b strings.Builder
	b.WriteString("Pending skill drafts:\n")
	for _, draft := range drafts {
		fmt.Fprintf(&b, "- %s: %s\n", draft.Name, draft.Description)
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}

// ReadSkillDraft reads one pending Agent Skill draft's SKILL.md.
type ReadSkillDraft struct{}

func (ReadSkillDraft) Spec() ToolSpec {
	return ToolSpec{
		Name:        "read_skill_draft",
		Description: "Read one pending Agent Skill draft by name, including its full SKILL.md contents.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Draft name, e.g. commit-conventions"}},"required":["name"]}`),
	}
}

func (ReadSkillDraft) Execute(_ context.Context, input json.RawMessage) (Result, error) {
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
	draft, content, err := skills.DraftMarkdown(name)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: fmt.Sprintf("name: %s\ndescription: %s\npath: %s\n\n%s", draft.Name, draft.Description, draft.Path, content)}, nil
}

// PromoteSkillDraft promotes a pending Agent Skill draft into the managed skills root.
type PromoteSkillDraft struct{}

func (PromoteSkillDraft) Spec() ToolSpec {
	return ToolSpec{
		Name:        "promote_skill_draft",
		Description: "Promote a pending Agent Skill draft into ~/.cometmind/skills so it becomes available as a normal skill.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Draft name, e.g. commit-conventions"}},"required":["name"]}`),
	}
}

func (PromoteSkillDraft) Execute(_ context.Context, input json.RawMessage) (Result, error) {
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
	if err := skills.PromoteDraft(name); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: "skill draft " + name + " promoted to ~/.cometmind/skills/" + name + "/SKILL.md"}, nil
}
