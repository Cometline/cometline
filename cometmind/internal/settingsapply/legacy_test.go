package settingsapply

import "testing"

func TestNormalizeLegacyActiveProviderMigratesAndStrips(t *testing.T) {
	doc := map[string]any{
		"activeProviderId":  "codex",
		"defaultProviderId": "",
	}
	got := NormalizeLegacyActiveProvider(doc)
	if got["defaultProviderId"] != "codex" {
		t.Fatalf("defaultProviderId = %v, want codex", got["defaultProviderId"])
	}
	if _, ok := got["activeProviderId"]; ok {
		t.Fatal("activeProviderId should be stripped")
	}
}

func TestNormalizeLegacyActiveProviderKeepsExplicitDefault(t *testing.T) {
	doc := map[string]any{
		"activeProviderId":  "codex",
		"defaultProviderId": "opencode-go",
	}
	got := NormalizeLegacyActiveProvider(doc)
	if got["defaultProviderId"] != "opencode-go" {
		t.Fatalf("defaultProviderId = %v, want opencode-go", got["defaultProviderId"])
	}
	if _, ok := got["activeProviderId"]; ok {
		t.Fatal("activeProviderId should be stripped")
	}
}

func TestRewriteLegacyActiveProviderPatch(t *testing.T) {
	patch := map[string]any{"activeProviderId": "anthropic"}
	RewriteLegacyActiveProviderPatch(patch)
	if patch["defaultProviderId"] != "anthropic" {
		t.Fatalf("defaultProviderId = %v, want anthropic", patch["defaultProviderId"])
	}
	if _, ok := patch["activeProviderId"]; ok {
		t.Fatal("activeProviderId should be removed from patch")
	}
}

func TestRewriteLegacyActiveProviderPatchKeepsExplicitDefault(t *testing.T) {
	patch := map[string]any{
		"activeProviderId":  "codex",
		"defaultProviderId": "opencode-go",
	}
	RewriteLegacyActiveProviderPatch(patch)
	if patch["defaultProviderId"] != "opencode-go" {
		t.Fatalf("defaultProviderId = %v, want opencode-go", patch["defaultProviderId"])
	}
	if _, ok := patch["activeProviderId"]; ok {
		t.Fatal("activeProviderId should be removed from patch")
	}
}
