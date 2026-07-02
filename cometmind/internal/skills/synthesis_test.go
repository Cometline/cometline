package skills

import (
	"context"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

type synthesisProvider struct {
	text string
}

func (p synthesisProvider) ID() string { return "static" }

func (p synthesisProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, 3)
	ch <- cometsdk.TextDeltaEvent{Text: p.text}
	ch <- cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop}
	ch <- cometsdk.DoneEvent{}
	close(ch)
	return ch, nil
}

func TestProposeSkillFromJobWritesDraftOutsideDiscoveryRoots(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	json := `{"should_propose":true,"name":"retry-policy","content":"---\nname: retry-policy\ndescription: Handle retry policy work\n---\n\n## Trigger\nUse for retry policy work.","reason":"reusable workflow"}`
	job := SynthesisJob{
		ID:               "job-1",
		Description:      "Implement a retry policy",
		DefinitionOfDone: "Retries are bounded",
		Progress:         "Added blocked state",
	}
	if err := ProposeSkillFromJob(context.Background(), synthesisProvider{text: json}, "test-model", job, nil); err != nil {
		t.Fatalf("ProposeSkillFromJob() error = %v", err)
	}
	_, content, err := DraftMarkdown("retry-policy")
	if err != nil {
		t.Fatalf("DraftMarkdown() error = %v", err)
	}
	if !strings.Contains(content, "Handle retry policy work") {
		t.Fatalf("draft content missing expected text: %s", content)
	}
	reg := Discover("", Config{Enabled: true})
	if _, ok := reg.Find("retry-policy"); ok {
		t.Fatalf("draft should not be discoverable before promotion")
	}
}

func TestProposeSkillFromJobSkipsWhenModelDeclines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	json := `{"should_propose":false,"name":"","content":"","reason":"one-off"}`
	job := SynthesisJob{ID: "job-1", Description: "One-off cleanup"}
	if err := ProposeSkillFromJob(context.Background(), synthesisProvider{text: json}, "test-model", job, nil); err != nil {
		t.Fatalf("ProposeSkillFromJob() error = %v", err)
	}
	drafts, err := ListDrafts()
	if err != nil {
		t.Fatalf("ListDrafts() error = %v", err)
	}
	if len(drafts) != 0 {
		t.Fatalf("unexpected drafts: %+v", drafts)
	}
}
