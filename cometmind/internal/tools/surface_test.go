package tools

import "testing"

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
