package tools

import (
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/skills"
)

func TestSurfaceForMode(t *testing.T) {
	r := ResearchSurface()
	if r.Edit || r.Run || r.Spawn || r.Delegate {
		t.Fatalf("research should be read-only: %+v", r)
	}
	c := CodingSurface()
	if !c.Edit || !c.Run || c.Spawn || c.Delegate {
		t.Fatalf("coding should edit/run without spawn/delegate: %+v", c)
	}
	p := ParentSurface(true)
	if !p.Delegate || !p.Spawn || !p.Edit || !p.Settings || !p.Inbox {
		t.Fatalf("parent with delegate: %+v", p)
	}
	pOff := ParentSurface(false)
	if pOff.Delegate {
		t.Fatal("parent without harness should not delegate")
	}
}

func TestSkillReviewSurfaceOmitsMutationTools(t *testing.T) {
	surface := SkillReviewSurface()
	if !surface.Read || !surface.Skills || !surface.SkillMut {
		t.Fatalf("skill review surface = %+v", surface)
	}
	if surface.Edit || surface.Run || surface.Spawn || surface.SkillDraft || surface.Delegate {
		t.Fatalf("skill review surface is too wide: %+v", surface)
	}
	reg := NewSkillReviewRegistry(t.TempDir(), &skills.Registry{})
	got := map[string]bool{}
	for _, tool := range reg.CometSDK() {
		got[tool.Name] = true
	}
	for _, name := range []string{"read_file", "list_dir", "glob", "grep", "load_skill", "read_skill_file", "write_skill"} {
		if !got[name] {
			t.Fatalf("missing %s in %#v", name, got)
		}
	}
	for _, name := range []string{"edit_file", "write_file", "run_command", "write_skill_draft", "promote_skill_draft", "web_search", "web_fetch", "spawn_general_agent"} {
		if got[name] {
			t.Fatalf("unexpected %s", name)
		}
	}
}

func TestSessionKindAndLabels(t *testing.T) {
	if SessionKindForMode(SubagentModeCoding) != SessionKindCoding {
		t.Fatal("coding kind")
	}
	if AgentLabelForMode(SubagentModeResearch) != AgentLabelResearch {
		t.Fatal("research label")
	}
	if AgentLabelForSessionKind(SessionKindCoding) != AgentLabelCoding {
		t.Fatal("label from kind")
	}
}
