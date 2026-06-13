package config

import "testing"

func TestProviderAPIKeyUsesProviderSpecificVariable(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("COMETMIND_API_KEY", "generic-key")

	key, err := ProviderAPIKey(&Config{Provider: ProviderOpenAI})
	if err != nil {
		t.Fatalf("ProviderAPIKey() error = %v", err)
	}
	if key != "openai-key" {
		t.Fatalf("key = %q, want %q", key, "openai-key")
	}
}

func TestProviderAPIKeyFallsBackToGenericVariable(t *testing.T) {
	t.Setenv("COMETMIND_API_KEY", "generic-key")

	key, err := ProviderAPIKey(&Config{Provider: ProviderOpenAI})
	if err != nil {
		t.Fatalf("ProviderAPIKey() error = %v", err)
	}
	if key != "generic-key" {
		t.Fatalf("key = %q, want %q", key, "generic-key")
	}
}
