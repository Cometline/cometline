package config

// InboxConfig controls the agent inbox background worker and retention.
type InboxConfig struct {
	PollIntervalSeconds int `mapstructure:"poll_interval_seconds" json:"poll_interval_seconds"`
	RetentionHours      int `mapstructure:"retention_hours" json:"retention_hours"`
	MaxStepsPerRun      int `mapstructure:"max_steps_per_run" json:"max_steps_per_run"`
}

func defaultInboxConfig() InboxConfig {
	return InboxConfig{
		PollIntervalSeconds: 600, // 10 minutes
		RetentionHours:      24,
		MaxStepsPerRun:      8,
	}
}

// EffectiveInboxSettings returns inbox settings with defaults applied.
func (c *Config) EffectiveInboxSettings() InboxConfig {
	s := InboxConfig{}
	if c != nil {
		s = c.Inbox
	}
	def := defaultInboxConfig()
	if s.PollIntervalSeconds <= 0 {
		s.PollIntervalSeconds = def.PollIntervalSeconds
	}
	if s.RetentionHours <= 0 {
		s.RetentionHours = def.RetentionHours
	}
	if s.MaxStepsPerRun <= 0 {
		s.MaxStepsPerRun = def.MaxStepsPerRun
	}
	return s
}
