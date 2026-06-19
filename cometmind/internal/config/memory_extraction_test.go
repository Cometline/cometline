package config

import "testing"

func TestExtractionLLMForSessionUsesConfiguredProviderAndModel(t *testing.T) {
	cfg := &Config{
		Memory: MemoryConfig{
			ExtractionProvider: "opencode-go",
			ExtractionModel:    "deepseek-v4-flash",
		},
	}
	providerID, model := cfg.ExtractionLLMForSession("codex", "gpt-5.4")
	if providerID != "opencode-go" {
		t.Fatalf("provider = %q, want opencode-go", providerID)
	}
	if model != "deepseek-v4-flash" {
		t.Fatalf("model = %q, want deepseek-v4-flash", model)
	}
}

func TestExtractionLLMForSessionFallsBackToSession(t *testing.T) {
	cfg := &Config{Memory: MemoryConfig{}}
	providerID, model := cfg.ExtractionLLMForSession("codex", "gpt-5.4")
	if providerID != "codex" || model != "gpt-5.4" {
		t.Fatalf("got provider=%q model=%q, want codex/gpt-5.4", providerID, model)
	}
}

func TestAdaptCometlineSettingsMapsExtractionProvider(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{{
			ID: "codex", Name: "Codex", Method: "codex", Enabled: true, EnabledModels: []string{"gpt-5.4"},
		}},
		ActiveProviderID: "codex",
		Cometmind: cometlineCometmindJSON{
			Memory: struct {
				ExtractionProviderID string                       `json:"extractionProviderId"`
				ExtractionModel      string                       `json:"extractionModel"`
				Embedding            cometlineMemoryEmbeddingJSON `json:"embedding"`
			}{
				ExtractionProviderID: "opencode-go",
				ExtractionModel:      "deepseek-v4-flash",
			},
		},
	})
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v", err)
	}
	if cfg.Memory.ExtractionProvider != "opencode-go" {
		t.Fatalf("ExtractionProvider = %q", cfg.Memory.ExtractionProvider)
	}
	if cfg.Memory.ExtractionModel != "deepseek-v4-flash" {
		t.Fatalf("ExtractionModel = %q", cfg.Memory.ExtractionModel)
	}
}
