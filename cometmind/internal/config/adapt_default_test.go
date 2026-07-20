package config

import "testing"

func TestAdaptCometlineSettingsPrefersDefaultOverActive(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{
			{
				ID: "codex", Name: "Codex", Method: ProviderCodex, Enabled: true,
				EnabledModels: []string{"gpt-5.4-mini", "gpt-5.4"},
			},
			{
				ID: "opencode-go", Name: "OpenCode Go", Method: ProviderOpencodeGo, Enabled: true,
				EnabledModels: []string{"deepseek-v4-flash", "qwen3.7-plus"},
				BaseURL:       "https://opencode.ai/zen/go/v1",
				APIKey:        "k",
			},
		},
		ActiveProviderID:  "codex",
		DefaultProviderID: "opencode-go",
		DefaultModelID:    "deepseek-v4-flash",
	})
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v", err)
	}
	if cfg.Provider != "opencode-go" || cfg.DefaultProviderID != "opencode-go" {
		t.Fatalf("provider = %q/%q, want opencode-go", cfg.Provider, cfg.DefaultProviderID)
	}
	if cfg.Model != "deepseek-v4-flash" || cfg.DefaultModelID != "deepseek-v4-flash" {
		t.Fatalf("model = %q/%q, want deepseek-v4-flash", cfg.Model, cfg.DefaultModelID)
	}
}
