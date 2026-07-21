package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/skills"
)

func TestWriteSkillDraftBlocksOverlapUntilForce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	live := "---\nname: pr-review\ndescription: Review pull requests for this repository\n---\n\n# Live\n"
	if err := skills.WriteSkill("pr-review", live, false); err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}

	content := "---\nname: go-pr-review\ndescription: Specialized Go pull request checklist\n---\n\n# Draft\n"
	payload, err := json.Marshal(map[string]any{"name": "go-pr-review", "content": content})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	blocked, err := (WriteSkillDraft{}).Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if blocked.OK {
		t.Fatalf("expected overlap block, got %+v", blocked)
	}
	if !strings.Contains(blocked.Output, "pr-review") || !strings.Contains(blocked.Output, "force=true") {
		t.Fatalf("blocked output missing guidance: %s", blocked.Output)
	}
	if _, _, err := skills.DraftMarkdown("go-pr-review"); err == nil {
		t.Fatal("draft should not exist after blocked write")
	}

	forcePayload, err := json.Marshal(map[string]any{"name": "go-pr-review", "content": content, "force": true})
	if err != nil {
		t.Fatalf("marshal force: %v", err)
	}
	forced, err := (WriteSkillDraft{}).Execute(context.Background(), forcePayload)
	if err != nil {
		t.Fatalf("force Execute error = %v", err)
	}
	if !forced.OK {
		t.Fatalf("force write failed: %+v", forced)
	}
	if _, _, err := skills.DraftMarkdown("go-pr-review"); err != nil {
		t.Fatalf("draft missing after force: %v", err)
	}
}

func TestWriteSkillDraftOverwriteSameNameWithoutForce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	content := "---\nname: retry-policy\ndescription: Handle retry policy work\n---\n\n# v1\n"
	if err := skills.WriteDraft("retry-policy", content, false); err != nil {
		t.Fatalf("WriteDraft: %v", err)
	}
	updated := "---\nname: retry-policy\ndescription: Handle retry policy work\n---\n\n# v2\n"
	payload, err := json.Marshal(map[string]any{"name": "retry-policy", "content": updated, "overwrite": true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	res, err := (WriteSkillDraft{}).Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if !res.OK {
		t.Fatalf("overwrite same-name draft should succeed without force: %+v", res)
	}
	_, body, err := skills.DraftMarkdown("retry-policy")
	if err != nil {
		t.Fatalf("DraftMarkdown: %v", err)
	}
	if !strings.Contains(body, "# v2") {
		t.Fatalf("draft not updated: %s", body)
	}
}
