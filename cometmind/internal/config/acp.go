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
	return acp.DefaultHarnessConfig(acp.ParseHarness(c.ACP.DefaultHarness))
}
