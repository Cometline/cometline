package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/modelcatalog"
)

func TestLabelForModel(t *testing.T) {
	t.Parallel()

	if got := LabelForModel("claude-opus-4-5"); got != "CLAUDE-OPUS-4-5" {
		t.Fatalf("LabelForModel() = %q, want CLAUDE-OPUS-4-5", got)
	}
	if got := LabelForModel("gpt_4o"); got != "GPT 4O" {
		t.Fatalf("LabelForModel() = %q, want GPT 4O", got)
	}
}

func TestModelEntriesFromSettings(t *testing.T) {
	t.Parallel()

	raw := cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{
			{
				ID:            "anthropic",
				Enabled:       true,
				EnabledModels: []string{"claude-sonnet-4-5", "text-embedding-3-small"},
			},
			{
				ID:            "openai",
				Enabled:       true,
				EnabledModels: []string{"gpt-4o"},
			},
			{
				ID:            "disabled",
				Enabled:       false,
				EnabledModels: []string{"ignored"},
			},
		},
	}

	got := modelEntriesFromSettings(raw)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ProviderID != "anthropic" || got[0].ModelID != "claude-sonnet-4-5" {
		t.Fatalf("first entry = %+v", got[0])
	}
	if got[0].Context <= 0 || got[0].LimitSource == "" {
		t.Fatalf("first entry missing limits: %+v", got[0])
	}
	if got[1].ProviderID != "openai" || got[1].ModelID != "gpt-4o" {
		t.Fatalf("second entry = %+v", got[1])
	}
}

func TestLookupModelCatalog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "modelcatalog", "testdata", "models-dev-snippet.json"))
	if err != nil {
		t.Fatal(err)
	}
	modelcatalog.ResetCacheForTest()
	if err := modelcatalog.LoadFromJSONForTest(data); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(modelcatalog.ResetCacheForTest)

	got := LookupModelCatalog("openai", "openai", []string{"o3-mini", "o3-mini", "  ", "missing-model"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (dedupe + skip blank)", len(got))
	}
	if got[0].ModelID != "o3-mini" || !got[0].VisionKnown || got[0].Vision {
		t.Fatalf("entry = %+v", got[0])
	}
	if len(got[0].InputModalities) != 1 || got[0].InputModalities[0] != "text" {
		t.Fatalf("modalities = %v, want [text]", got[0].InputModalities)
	}
	if got[1].ModelID != "missing-model" || got[1].VisionKnown {
		t.Fatalf("miss = %+v", got[1])
	}
	if len(got[1].InputModalities) != 0 {
		t.Fatalf("fallback modalities = %v, want empty", got[1].InputModalities)
	}
}

func TestUpdateDefaultModel(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".cometmind")
	if err := os.MkdirAll(settingsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(settingsDir, "cometline-settings.json")
	data, err := os.ReadFile("testdata/cometline-settings.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("HOME", dir)

	if err := UpdateDefaultModel("anthropic", "claude-sonnet-4-5"); err != nil {
		t.Fatalf("UpdateDefaultModel() error = %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(updated) error = %v", err)
	}
	if !containsAll(string(updated), `"defaultProviderId": "anthropic"`, `"defaultModelId": "claude-sonnet-4-5"`) {
		t.Fatalf("updated settings missing defaults: %s", string(updated))
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
