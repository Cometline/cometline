package config

import (
	"testing"

	"github.com/cometline/cometmind/internal/acp"
)

func TestACPSettingsSelectsConfiguredHarness(t *testing.T) {
	cfg := &Config{ACP: ACPConfig{
		DefaultHarness: "codex",
	}}

	settings := cfg.ACPSettings()
	if settings.Harness != acp.HarnessCodex {
		t.Fatalf("harness = %q", settings.Harness)
	}
	if settings.Timeout.Minutes() != 30 {
		t.Fatalf("timeout = %s, want fixed 30m", settings.Timeout)
	}
}

func TestACPSettingsDefaultsToOpenCode(t *testing.T) {
	cfg := &Config{ACP: ACPConfig{}}

	settings := cfg.ACPSettings()
	if settings.Harness != acp.HarnessOpenCode {
		t.Fatalf("harness = %q", settings.Harness)
	}
}
