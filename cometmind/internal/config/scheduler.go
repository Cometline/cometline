package config

// SchedulerConfig controls deferred job materialization.
type SchedulerConfig struct {
	Enabled             bool `json:"enabled" mapstructure:"enabled"`
	PollIntervalSeconds int  `json:"poll_interval_seconds" mapstructure:"poll_interval_seconds"`
}

func defaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		Enabled:             false,
		PollIntervalSeconds: 60,
	}
}

// EffectiveSchedulerSettings returns scheduler settings with defaults applied.
func (c *Config) EffectiveSchedulerSettings() SchedulerConfig {
	s := c.Scheduler
	def := defaultSchedulerConfig()
	if s.PollIntervalSeconds <= 0 {
		s.PollIntervalSeconds = def.PollIntervalSeconds
	}
	return s
}
