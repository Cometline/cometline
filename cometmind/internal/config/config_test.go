package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultCometlineSettingsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COMETMIND_PROVIDER", "")
	t.Setenv("COMETMIND_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != ProviderAnthropic {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, ProviderAnthropic)
	}
	if cfg.BaseURL != "" {
		t.Fatalf("BaseURL = %q, want empty", cfg.BaseURL)
	}
	if cfg.MaxTokens != 2048 {
		t.Fatalf("MaxTokens = %d, want 2048", cfg.MaxTokens)
	}

	path := filepath.Join(home, ".cometmind", "cometline-settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected settings file at %s: %v", path, err)
	}
}

func TestLoadReadsBaseURLEnvironmentOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("COMETMIND_BASE_URL", "http://localhost:11434/v1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:11434/v1")
	}
}

func TestLoadUsesConfiguredDataDir(t *testing.T) {
	home := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "cometmind-data")
	t.Setenv("HOME", home)
	t.Setenv("COMETMIND_DATA_DIR", dataDir)

	_, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	path := filepath.Join(dataDir, "cometline-settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected settings file at %s: %v", path, err)
	}
}

func TestLoadReadsSystemPromptPathEnvironmentOverride(t *testing.T) {
	home := t.TempDir()
	promptPath := filepath.Join(home, "SOUL.md")
	t.Setenv("HOME", home)
	t.Setenv("COMETMIND_SYSTEM_PROMPT_PATH", promptPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SystemPromptPath != promptPath {
		t.Fatalf("SystemPromptPath = %q, want %q", cfg.SystemPromptPath, promptPath)
	}
}

func TestLoadReadsCometlineSettingsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".cometmind")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "cometline-settings.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "cometline-settings.json"), fixture, 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(cfg.Providers))
	}
	if cfg.FindProvider("local-llm") == nil {
		t.Fatal("expected to find provider 'local-llm'")
	}
	anthropic := cfg.FindProvider("anthropic")
	if anthropic == nil {
		t.Fatal("expected to find provider 'anthropic'")
	}
	if anthropic.APIKey != "sk-ant-123" {
		t.Fatalf("anthropic APIKey = %q, want %q", anthropic.APIKey, "sk-ant-123")
	}
	if cfg.Provider != "local-llm" {
		t.Fatalf("Provider = %q, want local-llm", cfg.Provider)
	}
	if cfg.SystemPromptPath != "/tmp/SOUL.md" {
		t.Fatalf("SystemPromptPath = %q, want /tmp/SOUL.md", cfg.SystemPromptPath)
	}
	if cfg.MaxTokens != 2048 {
		t.Fatalf("MaxTokens = %d, want 2048", cfg.MaxTokens)
	}
	if cfg.Storage.RetentionDays != 90 {
		t.Fatalf("Storage.RetentionDays = %d, want 90", cfg.Storage.RetentionDays)
	}
	if cfg.Storage.ArchivedMemoryPurgeDays != 90 {
		t.Fatalf("Storage.ArchivedMemoryPurgeDays = %d, want 90", cfg.Storage.ArchivedMemoryPurgeDays)
	}
	if !cfg.Storage.VacuumAfterPurge {
		t.Fatal("expected Storage.VacuumAfterPurge true")
	}
	if cfg.ACP.DefaultHarness != "opencode" {
		t.Fatalf("ACP.DefaultHarness = %q, want opencode", cfg.ACP.DefaultHarness)
	}
}

func TestLoadReadsLegacyProvidersToml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".cometmind")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `provider = "local-llm"
model = "qwen2.5"
base_url = "http://localhost:11434/v1"
max_tokens = 4096
max_steps = 25

[[providers]]
id = "local-llm"
name = "Local LLM"
method = "openai-compatible"
base_url = "http://localhost:11434/v1"
api_key = "ignored"
model = "qwen2.5"

[[providers]]
id = "anthropic"
name = "Anthropic"
method = "anthropic"
base_url = "https://api.anthropic.com"
api_key = "sk-ant-123"
model = "claude-sonnet-4-5"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(cfg.Providers))
	}
	if cfg.FindProvider("local-llm") == nil {
		t.Fatal("expected to find provider 'local-llm'")
	}
	anthropic := cfg.FindProvider("anthropic")
	if anthropic == nil {
		t.Fatal("expected to find provider 'anthropic'")
	}
	if anthropic.APIKey != "sk-ant-123" {
		t.Fatalf("anthropic APIKey = %q, want %q", anthropic.APIKey, "sk-ant-123")
	}
}

func TestAdaptCometlineSettingsMatchesRuntimeSlice(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "cometline-settings.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(fixture, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg, err := adaptCometlineSettings(raw)
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v", err)
	}
	if cfg.Gateway.Discord.BotTokenEnv != "DISCORD_BOT_TOKEN" {
		t.Fatalf("BotTokenEnv = %q", cfg.Gateway.Discord.BotTokenEnv)
	}
	if !cfg.Skills.IncludeOpenCode {
		t.Fatal("expected skills.include_opencode true")
	}
	if cfg.Storage.RetentionDays != 90 {
		t.Fatalf("Storage.RetentionDays = %d, want 90", cfg.Storage.RetentionDays)
	}
	if cfg.Storage.MaxSessionsPerWorkspace != 0 {
		t.Fatalf("Storage.MaxSessionsPerWorkspace = %d, want 0", cfg.Storage.MaxSessionsPerWorkspace)
	}
}

func TestAdaptCometlineSettingsContextWindowLimit(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{{
			ID:            "anthropic",
			Name:          "Anthropic",
			Method:        ProviderAnthropic,
			Enabled:       true,
			BaseURL:       "https://api.anthropic.com",
			EnabledModels: []string{"claude-sonnet-4-20250514"},
		}},
		ActiveProviderID: "anthropic",
		Cometmind: cometlineCometmindJSON{
			MaxTokens:          2048,
			ContextWindowLimit: 256_000,
		},
	})
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v", err)
	}
	if cfg.ContextWindowLimit != 256_000 {
		t.Fatalf("ContextWindowLimit = %d, want 256000", cfg.ContextWindowLimit)
	}
}

func TestAdaptCometlineSettingsAutonomyModelOverride(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{
			{
				ID:            "anthropic",
				Name:          "Anthropic",
				Method:        ProviderAnthropic,
				Enabled:       true,
				BaseURL:       "https://api.anthropic.com",
				EnabledModels: []string{"claude-sonnet-4-20250514"},
			},
			{
				ID:            "codex",
				Name:          "Codex",
				Method:        ProviderCodex,
				Enabled:       true,
				EnabledModels: []string{"gpt-5.1-codex"},
			},
		},
		ActiveProviderID: "anthropic",
		Cometmind: cometlineCometmindJSON{
			Autonomy: cometlineAutonomyJSON{
				ProviderID: " codex ",
				ModelID:    " gpt-5.1-codex ",
			},
		},
	})
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Fatalf("Provider = %q, want default-from-active anthropic", cfg.Provider)
	}
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("Model = %q, want default-from-active model claude-sonnet-4-20250514", cfg.Model)
	}
	if cfg.DefaultProviderID != "anthropic" {
		t.Fatalf("DefaultProviderID = %q, want anthropic", cfg.DefaultProviderID)
	}
	if cfg.Autonomy.ProviderID != "codex" {
		t.Fatalf("Autonomy.ProviderID = %q, want codex", cfg.Autonomy.ProviderID)
	}
	if cfg.Autonomy.ModelID != "gpt-5.1-codex" {
		t.Fatalf("Autonomy.ModelID = %q, want gpt-5.1-codex", cfg.Autonomy.ModelID)
	}
}

func TestAdaptCometlineSettingsMemoryBehavior(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{{
			ID: "codex", Name: "Codex", Method: "codex", Enabled: true, EnabledModels: []string{"gpt-5.4"},
		}},
		ActiveProviderID: "codex",
		Cometmind: cometlineCometmindJSON{
			Memory: cometlineMemoryJSON{
				Enabled:             true,
				AutoExtract:         false,
				AutoRetrieve:        false,
				MaxRetrieved:        9,
				TaskOutcomeLimit:    4,
				SimilarityThreshold: 0.72,
				Lifecycle: cometlineMemoryLifecycleJSON{
					DecayHalfLifeDays:     45,
					ForgetThreshold:       0.22,
					UsageBoostFactor:      0.33,
					MaxUsageBoost:         3.5,
					MaxMemories:           777,
					CompactionTargetRatio: 0.66,
					CompactionOnExtract:   false,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v", err)
	}
	if !cfg.Memory.Enabled {
		t.Fatal("Memory.Enabled = false, want true")
	}
	if cfg.Memory.AutoExtract {
		t.Fatal("Memory.AutoExtract = true, want false")
	}
	if cfg.Memory.AutoRetrieve {
		t.Fatal("Memory.AutoRetrieve = true, want false")
	}
	if cfg.Memory.MaxRetrieved != 9 {
		t.Fatalf("Memory.MaxRetrieved = %d, want 9", cfg.Memory.MaxRetrieved)
	}
	if cfg.Memory.TaskOutcomeLimit != 4 {
		t.Fatalf("Memory.TaskOutcomeLimit = %d, want 4", cfg.Memory.TaskOutcomeLimit)
	}
	if cfg.Memory.SimilarityThreshold != 0.72 {
		t.Fatalf("Memory.SimilarityThreshold = %v, want 0.72", cfg.Memory.SimilarityThreshold)
	}
	if cfg.Memory.Lifecycle.DecayHalfLifeDays != 45 {
		t.Fatalf("Memory.Lifecycle.DecayHalfLifeDays = %v, want 45", cfg.Memory.Lifecycle.DecayHalfLifeDays)
	}
	if cfg.Memory.Lifecycle.ForgetThreshold != 0.22 {
		t.Fatalf("Memory.Lifecycle.ForgetThreshold = %v, want 0.22", cfg.Memory.Lifecycle.ForgetThreshold)
	}
	if cfg.Memory.Lifecycle.UsageBoostFactor != 0.33 {
		t.Fatalf("Memory.Lifecycle.UsageBoostFactor = %v, want 0.33", cfg.Memory.Lifecycle.UsageBoostFactor)
	}
	if cfg.Memory.Lifecycle.MaxUsageBoost != 3.5 {
		t.Fatalf("Memory.Lifecycle.MaxUsageBoost = %v, want 3.5", cfg.Memory.Lifecycle.MaxUsageBoost)
	}
	if cfg.Memory.Lifecycle.MaxMemories != 777 {
		t.Fatalf("Memory.Lifecycle.MaxMemories = %d, want 777", cfg.Memory.Lifecycle.MaxMemories)
	}
	if cfg.Memory.Lifecycle.CompactionTargetRatio != 0.66 {
		t.Fatalf("Memory.Lifecycle.CompactionTargetRatio = %v, want 0.66", cfg.Memory.Lifecycle.CompactionTargetRatio)
	}
	if cfg.Memory.Lifecycle.CompactionOnExtract {
		t.Fatal("Memory.Lifecycle.CompactionOnExtract = true, want false")
	}
}

// TestLoadBootsWithNoEnabledProviders simulates a fresh Electron install:
// the renderer writes a settings JSON where every provider is disabled with
// no enabled models. The sidecar must still boot (Load returns no error) with
// an empty provider configuration so the UI stays usable and can guide the
// user to Settings. Sending a message surfaces a clear error at request time
// instead of a TCP connection refused.
func TestLoadBootsWithNoEnabledProviders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".cometmind")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	empty := `{"providers":[
		{"id":"anthropic","name":"Anthropic","method":"anthropic","enabled":false,"baseURL":"https://api.anthropic.com","apiKey":"","selectedModel":"","models":[],"enabledModels":[]},
		{"id":"openai","name":"OpenAI","method":"openai","enabled":false,"baseURL":"https://api.openai.com/v1","apiKey":"","selectedModel":"","models":[],"enabledModels":[]}
	],"defaultProviderId":"","defaultModelId":""}`
	if err := os.WriteFile(filepath.Join(configDir, "cometline-settings.json"), []byte(empty), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil (sidecar must boot with no providers)", err)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("len(Providers) = %d, want 0", len(cfg.Providers))
	}
	if cfg.Provider != "" {
		t.Fatalf("Provider = %q, want empty (no provider configured)", cfg.Provider)
	}
	if cfg.Model != "" {
		t.Fatalf("Model = %q, want empty (no provider configured)", cfg.Model)
	}
	// Non-provider defaults should still be applied so the sidecar is usable.
	if cfg.MaxTokens != 2048 {
		t.Fatalf("MaxTokens = %d, want 2048", cfg.MaxTokens)
	}
	if cfg.MaxSteps != 50 {
		t.Fatalf("MaxSteps = %d, want 50", cfg.MaxSteps)
	}
}

// TestAdaptCometlineSettingsEmptyProviders verifies the adapter directly:
// an all-disabled provider list must not error and must yield an empty
// provider config rather than the legacy hard fail.
func TestAdaptCometlineSettingsEmptyProviders(t *testing.T) {
	cfg, err := adaptCometlineSettings(cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{{
			ID:      "anthropic",
			Name:    "Anthropic",
			Method:  ProviderAnthropic,
			Enabled: false,
		}},
	})
	if err != nil {
		t.Fatalf("adaptCometlineSettings() error = %v, want nil", err)
	}
	if len(cfg.Providers) != 0 {
		t.Fatalf("len(Providers) = %d, want 0", len(cfg.Providers))
	}
	if cfg.Provider != "" {
		t.Fatalf("Provider = %q, want empty", cfg.Provider)
	}
}
