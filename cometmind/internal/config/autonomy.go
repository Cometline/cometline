package config

// AutonomousJobsConfig controls unattended (autonomous) job pickup: a
// background worker that claims ready jobs from the queue and runs them to
// completion without a human opening a chat session first.
type AutonomousJobsConfig struct {
	Enabled             bool `json:"enabled" mapstructure:"enabled"`
	MaxConcurrent       int  `json:"max_concurrent" mapstructure:"max_concurrent"`
	PollIntervalSeconds int  `json:"poll_interval_seconds" mapstructure:"poll_interval_seconds"`
	MaxStepsPerRun      int  `json:"max_steps_per_run" mapstructure:"max_steps_per_run"`
}

func defaultAutonomousJobsConfig() AutonomousJobsConfig {
	return AutonomousJobsConfig{
		Enabled:             false,
		MaxConcurrent:       1,
		PollIntervalSeconds: 30,
		MaxStepsPerRun:      0, // derived from main max_steps when unset
	}
}

// EffectiveAutonomousJobsSettings returns autonomy settings with defaults applied.
func (c *Config) EffectiveAutonomousJobsSettings() AutonomousJobsConfig {
	s := c.Autonomy
	def := defaultAutonomousJobsConfig()
	if s.MaxConcurrent <= 0 {
		s.MaxConcurrent = def.MaxConcurrent
	}
	if s.PollIntervalSeconds <= 0 {
		s.PollIntervalSeconds = def.PollIntervalSeconds
	}
	if s.MaxStepsPerRun <= 0 {
		mainSteps := c.MaxSteps
		if mainSteps <= 0 {
			mainSteps = Defaults().MaxSteps
		}
		s.MaxStepsPerRun = mainSteps
	}
	return s
}
