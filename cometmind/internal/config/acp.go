package config

import (
	"time"

	"github.com/cometline/cometmind/internal/acp"
)

// ACPSettings converts config to runtime ACP settings.
func (c *Config) ACPSettings() acp.Config {
	out := acp.DefaultConfig()
	out.Interactive = true
	if c == nil {
		return out
	}
	if c.ACP.Command != "" {
		out.Command = c.ACP.Command
	}
	if len(c.ACP.Args) > 0 {
		out.Args = c.ACP.Args
	}
	if c.ACP.Timeout != "" {
		if d, err := time.ParseDuration(c.ACP.Timeout); err == nil {
			out.Timeout = d
		}
	}
	out.Interactive = c.ACP.Interactive
	return out
}
