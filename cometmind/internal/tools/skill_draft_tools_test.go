package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/skills"
)

func TestListReadAndPromoteSkillDraftTools(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	content := "---\nname: review-helper\ndescription: Review helper draft\n---\n\n# Draft\n"
	if err := skills.WriteDraft("review-helper", content, false); err != nil {
		t.Fatalf("WriteDraft() error = %v", err)
	}

	listRes, err := (ListSkillDrafts{}).Execute(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("ListSkillDrafts.Execute() error = %v", err)
	}
	if !listRes.OK || !strings.Contains(listRes.Output, "review-helper") {
		t.Fatalf("unexpected list result: %+v", listRes)
	}

	readRes, err := (ReadSkillDraft{}).Execute(context.Background(), []byte(`{"name":"review-helper"}`))
	if err != nil {
		t.Fatalf("ReadSkillDraft.Execute() error = %v", err)
	}
	if !readRes.OK || !strings.Contains(readRes.Output, "Review helper draft") {
		t.Fatalf("unexpected read result: %+v", readRes)
	}

	promoteRes, err := (PromoteSkillDraft{}).Execute(context.Background(), []byte(`{"name":"review-helper"}`))
	if err != nil {
		t.Fatalf("PromoteSkillDraft.Execute() error = %v", err)
	}
	if !promoteRes.OK || !strings.Contains(promoteRes.Output, "promoted") {
		t.Fatalf("unexpected promote result: %+v", promoteRes)
	}
	if _, _, err := skills.DraftMarkdown("review-helper"); err == nil {
		t.Fatalf("draft still exists after promotion")
	}
	reg := skills.Discover("", skills.Config{Enabled: true})
	if _, ok := reg.Find("review-helper"); !ok {
		t.Fatalf("promoted draft not discoverable as a skill")
	}
}
