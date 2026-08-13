package config

import "testing"

func TestExtractionLLMUsesConfiguredProviderAndModel(t *testing.T) {
	cfg := &Config{
		DefaultProviderID: "codex",
		DefaultModelID:    "gpt-5.4",
		Memory: MemoryConfig{
			ExtractionProvider: "opencode-go",
			ExtractionModel:    "deepseek-v4-flash",
		},
	}
	providerID, model := cfg.ExtractionLLM()
	if providerID != "opencode-go" {
		t.Fatalf("provider = %q, want opencode-go", providerID)
	}
	if model != "deepseek-v4-flash" {
		t.Fatalf("model = %q, want deepseek-v4-flash", model)
	}
}

func TestExtractionLLMFallsBackToDefault(t *testing.T) {
	cfg := &Config{
		Provider:          "codex",
		Model:             "gpt-5.4",
		DefaultProviderID: "opencode-go",
		DefaultModelID:    "deepseek-v4-flash",
		Memory:            MemoryConfig{},
	}
	providerID, model := cfg.ExtractionLLM()
	if providerID != "opencode-go" || model != "deepseek-v4-flash" {
		t.Fatalf("got provider=%q model=%q, want default opencode-go/deepseek-v4-flash", providerID, model)
	}
}

func TestExtractionLLMProviderPrefersExtractionProvider(t *testing.T) {
	cfg := &Config{
		Provider:          "codex",
		DefaultProviderID: "codex",
		DefaultModelID:    "gpt-5.4",
		Memory:            MemoryConfig{ExtractionProvider: "opencode-go", ExtractionModel: "qwen3.7-plus"},
	}
	if got, _ := cfg.ExtractionLLM(); got != "opencode-go" {
		t.Fatalf("ExtractionLLM() provider = %q, want opencode-go", got)
	}
}

func TestExtractionLLMProviderFallsBackToDefaultProvider(t *testing.T) {
	cfg := &Config{
		Provider:          "codex",
		DefaultProviderID: "opencode-go",
		DefaultModelID:    "deepseek-v4-flash",
	}
	if got, _ := cfg.ExtractionLLM(); got != "opencode-go" {
		t.Fatalf("ExtractionLLM() provider = %q, want opencode-go", got)
	}
}

func TestAdaptCometlineSettingsMapsExtractionProvider(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{{
			ID: "codex", Name: "Codex", Method: "codex", Enabled: true, EnabledModels: []string{"gpt-5.4"},
		}},
		ActiveProviderID: "codex",
		Cometmind: cometlineCometmindJSON{
			Memory: cometlineMemoryJSON{
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
