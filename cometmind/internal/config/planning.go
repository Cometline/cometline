package config

// PlanningConfig controls agent-visible session planning tools.
type PlanningConfig struct {
	Enabled bool `json:"enabled" mapstructure:"enabled"`
}

func defaultPlanningConfig() PlanningConfig {
	return PlanningConfig{Enabled: false}
}

// EffectivePlanningSettings applies defaults for omitted planning settings.
func (c *Config) EffectivePlanningSettings() PlanningConfig {
	if c == nil {
		return defaultPlanningConfig()
	}
	return c.Planning
}
