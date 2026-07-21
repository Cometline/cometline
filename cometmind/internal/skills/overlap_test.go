package skills

import (
	"path/filepath"
	"testing"
)

func TestNameContainmentGoPRReview(t *testing.T) {
	score, ok := nameSimilarity("go-pr-review", "pr-review")
	if !ok || score < overlapBlockThreshold {
		t.Fatalf("nameSimilarity(go-pr-review, pr-review) = %v, %v; want block-level containment", score, ok)
	}
}

func TestFindRelatedManagedSkillsExactAndDescription(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	live := `---
name: terraform-apply-guardrails
description: plan validate apply terraform and handle provider auth failures
---

Body.
`
	if err := WriteSkill("terraform-apply-guardrails", live, false); err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}

	related, err := FindRelatedManagedSkills(
		"tf-provider-auth-recovery",
		"plan validate apply terraform and handle provider auth failures on recovery",
	)
	if err != nil {
		t.Fatalf("FindRelatedManagedSkills: %v", err)
	}
	if !ShouldBlockOverlap(related) {
		t.Fatalf("expected description overlap to block, got %+v", related)
	}
	found := false
	for _, r := range related {
		if r.Name == "terraform-apply-guardrails" && r.Location == OverlapLocationLive {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected live terraform skill in related: %+v", related)
	}
}

func TestFindRelatedManagedSkillsIgnoresWorkspaceOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ws := t.TempDir()
	writeSkill(t, filepath.Join(ws, ".agents", "skills"), "workspace-only", "only in workspace review pull requests")

	related, err := FindRelatedManagedSkills("repo-pr-review", "only in workspace review pull requests carefully")
	if err != nil {
		t.Fatalf("FindRelatedManagedSkills: %v", err)
	}
	for _, r := range related {
		if r.Name == "workspace-only" {
			t.Fatalf("workspace skill must not be in managed overlap: %+v", related)
		}
	}
}

func TestFindRelatedManagedSkillsUnrelated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := `---
name: commit-conventions
description: Write conventional commit messages for this monorepo
---

Body.
`
	if err := WriteSkill("commit-conventions", content, false); err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}
	related, err := FindRelatedManagedSkills("docker-build-cache", "Tune Docker layer caching for CI builds")
	if err != nil {
		t.Fatalf("FindRelatedManagedSkills: %v", err)
	}
	if ShouldBlockOverlap(related) {
		t.Fatalf("unrelated skills should not block: %+v", related)
	}
}

func TestFilterSelfDraftOverwrite(t *testing.T) {
	related := []RelatedSkill{
		{Name: "retry-policy", Location: OverlapLocationDraft, Kind: OverlapKindExactName, Score: 1},
		{Name: "other", Location: OverlapLocationLive, Kind: OverlapKindSimilarDescription, Score: 0.5},
	}
	filtered := FilterSelfDraftOverwrite(related, "retry-policy", true)
	if len(filtered) != 1 || filtered[0].Name != "other" {
		t.Fatalf("FilterSelfDraftOverwrite = %+v", filtered)
	}
	if got := FilterSelfDraftOverwrite(related, "retry-policy", false); len(got) != 2 {
		t.Fatalf("overwrite=false should keep all, got %+v", got)
	}
}

func TestFindRelatedManagedSkillsNameContainmentViaLive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	content := `---
name: pr-review
description: Review pull requests for this repository
---

Body.
`
	if err := WriteSkill("pr-review", content, false); err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}
	related, err := FindRelatedManagedSkills("go-pr-review", "Specialized Go pull request checklist")
	if err != nil {
		t.Fatalf("FindRelatedManagedSkills: %v", err)
	}
	if !ShouldBlockOverlap(related) {
		t.Fatalf("expected name containment to block, got %+v", related)
	}
	if related[0].Kind != OverlapKindSimilarName && related[0].Kind != OverlapKindExactName {
		t.Fatalf("expected similar_name, got %+v", related[0])
	}
}
