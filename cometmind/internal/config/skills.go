package config

import skillpkg "github.com/cometline/cometmind/internal/skills"

// SkillSettings converts config to runtime skill discovery settings.
func (c *Config) SkillSettings() skillpkg.Config {
	out := skillpkg.Config{Enabled: true, IncludeOpenCode: true, IncludeClaude: true}
	if c == nil {
		return out
	}
	out.Enabled = c.Skills.Enabled
	out.Roots = append([]string(nil), c.Skills.Roots...)
	out.IncludeOpenCode = c.Skills.IncludeOpenCode
	out.IncludeClaude = c.Skills.IncludeClaude
	return out
}
