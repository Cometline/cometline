package config

import (
	"github.com/cometline/cometmind/internal/acp"
)

// ACPSettings converts the selected coding harness into the fixed runtime
// profile. The legacy method name is retained for config/runtime compatibility.
func (c *Config) ACPSettings() acp.Config {
	if c == nil {
		return acp.DefaultConfig()
	}
	cfg := acp.DefaultHarnessConfig(acp.ParseHarness(c.ACP.DefaultHarness))
	cfg.Enabled = c.ACP.Enabled
	return cfg
}

func boolPtr(v bool) *bool { return &v }
