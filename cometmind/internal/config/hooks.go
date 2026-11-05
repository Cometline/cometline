package config

// HooksConfig controls CometMind subprocess hooks (PreToolUse / PostToolUse).
// Disabled by default until explicitly enabled in settings.
type HooksConfig struct {
	Enabled               bool     `mapstructure:"enabled" json:"enabled"`
	TrustedWorkspacePaths []string `mapstructure:"trusted_workspace_paths" json:"trustedWorkspacePaths"`
}

func defaultHooksConfig() HooksConfig {
	return HooksConfig{Enabled: false}
}

// HooksSettings returns hook settings with defaults applied.
func (c *Config) HooksSettings() HooksConfig {
	if c == nil {
		return defaultHooksConfig()
	}
	out := c.Hooks
	return out
}
