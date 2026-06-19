package config

import "testing"

func TestMemorySettingsOpenAICompatibleUsesBaseURLAndKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")

	cfg := &Config{
		BaseURL: "http://localhost:8080/v1",
		Memory: MemoryConfig{
			Enabled: true,
			Embedding: MemoryEmbeddingConfig{
				Provider: ProviderOpenAICompat,
				Model:    "text-embedding-3-small",
			},
		},
	}
	s := cfg.MemorySettings()
	if s.Embedding.BaseURL != "http://localhost:8080/v1" {
		t.Fatalf("base URL = %q", s.Embedding.BaseURL)
	}
	if s.Embedding.APIKey != "openai-key" {
		t.Fatalf("api key = %q", s.Embedding.APIKey)
	}
}

func TestMemorySettingsFromProviderID(t *testing.T) {
	cfg := &Config{
		BaseURL: "http://localhost:8080/v1",
		Providers: []ProviderEntry{
			{
				ID:     ProviderOpenAI,
				Method: ProviderOpenAI,
				APIKey: "provider-key",
				Model:  "gpt-4o",
			},
		},
		Memory: MemoryConfig{
			Enabled: true,
			Embedding: MemoryEmbeddingConfig{
				ProviderID: ProviderOpenAI,
				Model:      "text-embedding-3-small",
			},
		},
	}
	s := cfg.MemorySettings()
	if s.Embedding.Provider != ProviderOpenAI {
		t.Fatalf("provider = %q", s.Embedding.Provider)
	}
	if s.Embedding.Model != "text-embedding-3-small" {
		t.Fatalf("model = %q", s.Embedding.Model)
	}
	if s.Embedding.APIKey != "provider-key" {
		t.Fatalf("api key = %q", s.Embedding.APIKey)
	}
}

func TestMemorySettingsEmptyWithoutEmbeddingModel(t *testing.T) {
	cfg := &Config{
		Memory: MemoryConfig{Enabled: true},
	}
	s := cfg.MemorySettings()
	if s.Embedding.Model != "" {
		t.Fatalf("expected empty embedding, got model %q", s.Embedding.Model)
	}
}

func TestMemorySettingsOpenAICompatibleExplicitOverrides(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")

	cfg := &Config{
		BaseURL: "http://ignored/v1",
		Memory: MemoryConfig{
			Enabled: true,
			Embedding: MemoryEmbeddingConfig{
				Provider: ProviderOpenAICompat,
				Model:    "custom-embed",
				BaseURL:  "http://custom/v1",
				APIKey:   "explicit-key",
			},
		},
	}
	s := cfg.MemorySettings()
	if s.Embedding.BaseURL != "http://custom/v1" {
		t.Fatalf("base URL = %q", s.Embedding.BaseURL)
	}
	if s.Embedding.APIKey != "explicit-key" {
		t.Fatalf("api key = %q", s.Embedding.APIKey)
	}
}
