package config

import "testing"

func TestProviderAPIKeyUsesProviderSpecificVariable(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("COMETMIND_API_KEY", "generic-key")

	key, err := ProviderAPIKey(&Config{Provider: ProviderOpenAI}, nil, ProviderOpenAI)
	if err != nil {
		t.Fatalf("ProviderAPIKey() error = %v", err)
	}
	if key != "openai-key" {
		t.Fatalf("key = %q, want %q", key, "openai-key")
	}
}

func TestProviderAPIKeyFallsBackToGenericVariable(t *testing.T) {
	t.Setenv("COMETMIND_API_KEY", "generic-key")

	key, err := ProviderAPIKey(&Config{Provider: ProviderOpenAI}, nil, ProviderOpenAI)
	if err != nil {
		t.Fatalf("ProviderAPIKey() error = %v", err)
	}
	if key != "generic-key" {
		t.Fatalf("key = %q, want %q", key, "generic-key")
	}
}

func TestProviderAPIKeyUsesProviderEntryKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")

	entry := &ProviderEntry{
		ID:      "my-provider",
		Method:  ProviderOpenAI,
		APIKey:  "entry-key",
		BaseURL: "http://example.com",
	}
	key, err := ProviderAPIKey(&Config{Provider: "my-provider"}, entry, ProviderOpenAI)
	if err != nil {
		t.Fatalf("ProviderAPIKey() error = %v", err)
	}
	if key != "entry-key" {
		t.Fatalf("key = %q, want %q", key, "entry-key")
	}
}

func TestProviderAPIKeyEmptyEntryKeyDoesNotFallBackToEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")

	entry := &ProviderEntry{
		ID:      "local-llm",
		Method:  ProviderOpenAICompat,
		APIKey:  "",
		BaseURL: "http://localhost:11434/v1",
	}
	key, err := ProviderAPIKey(&Config{Provider: "local-llm"}, entry, ProviderOpenAICompat)
	if err != nil {
		t.Fatalf("ProviderAPIKey() error = %v", err)
	}
	if key != "" {
		t.Fatalf("key = %q, want empty", key)
	}
}
