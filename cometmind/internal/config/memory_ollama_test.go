package config

import "testing"

func TestMemorySettingsFromOllamaProvider(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderEntry{{
			ID:      "ollama",
			Method:  ProviderOllama,
			BaseURL: "http://127.0.0.1:11434",
			APIKey:  "",
		}},
		Memory: MemoryConfig{
			Enabled: true,
			Embedding: MemoryEmbeddingConfig{
				ProviderID: "ollama",
				Model:      "qwen3-embedding:0.6b",
			},
		},
	}

	s := cfg.MemorySettings()
	if s.Embedding.Provider != ProviderOllama {
		t.Fatalf("provider = %q, want ollama", s.Embedding.Provider)
	}
	if s.Embedding.Model != "qwen3-embedding:0.6b" {
		t.Fatalf("model = %q", s.Embedding.Model)
	}
	if s.Embedding.BaseURL != "http://127.0.0.1:11434" {
		t.Fatalf("baseURL = %q", s.Embedding.BaseURL)
	}
	if s.Embedding.APIKey != "" {
		t.Fatalf("apiKey should be empty, got %q", s.Embedding.APIKey)
	}
}
