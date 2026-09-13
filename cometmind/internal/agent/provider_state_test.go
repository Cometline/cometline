package agent

import (
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestScopeProviderStateStampsMissingModelID(t *testing.T) {
	got := scopeProviderState([]cometsdk.ProviderState{{
		ProviderID: "",
		Data:       `{"blocks":[{"type":"thinking","thinking":"plan","signature":"sig"}]}`,
	}}, "anthropic", "claude-sonnet-4-5")
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ProviderID != "anthropic" {
		t.Fatalf("provider = %q", got[0].ProviderID)
	}
	if got[0].ModelID != "claude-sonnet-4-5" {
		t.Fatalf("model = %q", got[0].ModelID)
	}
}

func TestScopeProviderStateKeepsExistingModelID(t *testing.T) {
	got := scopeProviderState([]cometsdk.ProviderState{{
		ProviderID: "openai",
		ModelID:    "gpt-5.6-luna",
		Data:       "opaque",
	}}, "custom-codex", "other-model")
	if got[0].ProviderID != "custom-codex" {
		t.Fatalf("provider = %q", got[0].ProviderID)
	}
	if got[0].ModelID != "gpt-5.6-luna" {
		t.Fatalf("model = %q, want original", got[0].ModelID)
	}
}
