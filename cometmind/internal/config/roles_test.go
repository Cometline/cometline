package config

import "testing"

func TestResolveRoleLLMPrefersAtomicPin(t *testing.T) {
	cfg := &Config{
		DefaultProviderID: "codex",
		DefaultModelID:    "gpt-5.4",
	}
	providerID, modelID := cfg.ResolveRoleLLM("opencode-go", "qwen3.7-plus")
	if providerID != "opencode-go" || modelID != "qwen3.7-plus" {
		t.Fatalf("got %s/%s, want pin", providerID, modelID)
	}
}

func TestResolveRoleLLMIgnoresPartialPin(t *testing.T) {
	cfg := &Config{
		DefaultProviderID: "codex",
		DefaultModelID:    "gpt-5.4",
	}
	providerID, modelID := cfg.ResolveRoleLLM("opencode-go", "")
	if providerID != "codex" || modelID != "gpt-5.4" {
		t.Fatalf("got %s/%s, want default (partial pin ignored)", providerID, modelID)
	}
}

func TestResolveRoleLLMFallsBackToDefault(t *testing.T) {
	cfg := &Config{
		Provider:          "legacy-active",
		Model:             "legacy-model",
		DefaultProviderID: "opencode-go",
		DefaultModelID:    "deepseek-v4-flash",
	}
	providerID, modelID := cfg.ResolveRoleLLM("", "")
	if providerID != "opencode-go" || modelID != "deepseek-v4-flash" {
		t.Fatalf("got %s/%s, want default", providerID, modelID)
	}
}

func TestResolveRoleLLMMirrorsProviderModelWhenDefaultEmpty(t *testing.T) {
	cfg := &Config{Provider: "codex", Model: "gpt-5.4"}
	providerID, modelID := cfg.ResolveRoleLLM("", "")
	if providerID != "codex" || modelID != "gpt-5.4" {
		t.Fatalf("got %s/%s, want mirrored Provider/Model", providerID, modelID)
	}
}
