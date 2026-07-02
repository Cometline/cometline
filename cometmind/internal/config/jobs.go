package config

// JobNotificationSettings controls job status notifications.
type JobNotificationSettings struct {
	Enabled     bool `json:"enabled" mapstructure:"enabled"`
	OnClaimed   bool `json:"on_claimed" mapstructure:"on_claimed"`
	OnCompleted bool `json:"on_completed" mapstructure:"on_completed"`
	OnReleased  bool `json:"on_released" mapstructure:"on_released"`
	OnBlocked   bool `json:"on_blocked" mapstructure:"on_blocked"`
}

// JobSettings holds runtime job configuration.
type JobSettings struct {
	Notifications           JobNotificationSettings `json:"notifications"`
	LeaseMinutes            int                     `json:"lease_minutes"`
	DeletedPurgeDays        int                     `json:"deleted_purge_days"`
	ArchivedPurgeDays       int                     `json:"archived_purge_days"`
	StaleReviewMinutes      int                     `json:"stale_review_minutes"`
	MaxConsecutiveFailures  int                     `json:"max_consecutive_failures"`
	RetryCooldownMinutes    int                     `json:"retry_cooldown_minutes"`
	MaxRetryCooldownMinutes int                     `json:"max_retry_cooldown_minutes"`
	ReconcileIntervalS      int                     `json:"reconcile_interval_seconds"`
}

// DefaultJobSettings returns the default job settings.
func DefaultJobSettings() JobSettings {
	return JobSettings{
		Notifications: JobNotificationSettings{
			Enabled:     true,
			OnClaimed:   true,
			OnCompleted: true,
			OnReleased:  false,
			OnBlocked:   true,
		},
		LeaseMinutes:            30,
		DeletedPurgeDays:        30,
		ArchivedPurgeDays:       30,
		StaleReviewMinutes:      30,
		MaxConsecutiveFailures:  3,
		RetryCooldownMinutes:    5,
		MaxRetryCooldownMinutes: 60,
		ReconcileIntervalS:      120,
	}
}

// JobsConfig controls the global jobs queue.
type JobsConfig struct {
	Notifications            JobNotificationSettings `json:"notifications" mapstructure:"notifications"`
	LeaseMinutes             int                     `json:"lease_minutes" mapstructure:"lease_minutes"`
	DeletedPurgeDays         int                     `json:"deleted_purge_days" mapstructure:"deleted_purge_days"`
	ArchivedPurgeDays        int                     `json:"archived_purge_days" mapstructure:"archived_purge_days"`
	StaleReviewMinutes       int                     `json:"stale_review_minutes" mapstructure:"stale_review_minutes"`
	MaxConsecutiveFailures   int                     `json:"max_consecutive_failures" mapstructure:"max_consecutive_failures"`
	RetryCooldownMinutes     int                     `json:"retry_cooldown_minutes" mapstructure:"retry_cooldown_minutes"`
	MaxRetryCooldownMinutes  int                     `json:"max_retry_cooldown_minutes" mapstructure:"max_retry_cooldown_minutes"`
	ReconcileIntervalSeconds int                     `json:"reconcile_interval_seconds" mapstructure:"reconcile_interval_seconds"`
}

func defaultJobsConfig() JobsConfig {
	s := DefaultJobSettings()
	return JobsConfig{
		Notifications:            s.Notifications,
		LeaseMinutes:             s.LeaseMinutes,
		DeletedPurgeDays:         s.DeletedPurgeDays,
		ArchivedPurgeDays:        s.ArchivedPurgeDays,
		StaleReviewMinutes:       s.StaleReviewMinutes,
		MaxConsecutiveFailures:   s.MaxConsecutiveFailures,
		RetryCooldownMinutes:     s.RetryCooldownMinutes,
		MaxRetryCooldownMinutes:  s.MaxRetryCooldownMinutes,
		ReconcileIntervalSeconds: s.ReconcileIntervalS,
	}
}

// JobsSettings returns runtime job settings with defaults applied.
func (c *Config) JobsSettings() JobSettings {
	if c == nil {
		return DefaultJobSettings()
	}
	def := DefaultJobSettings()
	j := c.Jobs
	s := JobSettings{
		Notifications:           j.Notifications,
		LeaseMinutes:            j.LeaseMinutes,
		DeletedPurgeDays:        j.DeletedPurgeDays,
		ArchivedPurgeDays:       j.ArchivedPurgeDays,
		StaleReviewMinutes:      j.StaleReviewMinutes,
		MaxConsecutiveFailures:  j.MaxConsecutiveFailures,
		RetryCooldownMinutes:    j.RetryCooldownMinutes,
		MaxRetryCooldownMinutes: j.MaxRetryCooldownMinutes,
		ReconcileIntervalS:      j.ReconcileIntervalSeconds,
	}
	if s.LeaseMinutes <= 0 {
		s.LeaseMinutes = def.LeaseMinutes
	}
	if s.DeletedPurgeDays <= 0 && j.DeletedPurgeDays != 0 {
		s.DeletedPurgeDays = def.DeletedPurgeDays
	}
	if s.DeletedPurgeDays == 0 && c.Storage.DeletedJobPurgeDays > 0 {
		s.DeletedPurgeDays = c.Storage.DeletedJobPurgeDays
	}
	if s.DeletedPurgeDays <= 0 {
		s.DeletedPurgeDays = def.DeletedPurgeDays
	}
	if s.ArchivedPurgeDays <= 0 {
		s.ArchivedPurgeDays = def.ArchivedPurgeDays
	}
	if s.StaleReviewMinutes <= 0 {
		s.StaleReviewMinutes = def.StaleReviewMinutes
	}
	if s.MaxConsecutiveFailures <= 0 {
		s.MaxConsecutiveFailures = def.MaxConsecutiveFailures
	}
	if s.RetryCooldownMinutes <= 0 {
		s.RetryCooldownMinutes = def.RetryCooldownMinutes
	}
	if s.MaxRetryCooldownMinutes <= 0 {
		s.MaxRetryCooldownMinutes = def.MaxRetryCooldownMinutes
	}
	if s.MaxRetryCooldownMinutes < s.RetryCooldownMinutes {
		s.MaxRetryCooldownMinutes = s.RetryCooldownMinutes
	}
	if s.ReconcileIntervalS <= 0 {
		s.ReconcileIntervalS = def.ReconcileIntervalS
	}
	if s.Notifications == (JobNotificationSettings{}) {
		s.Notifications = def.Notifications
	}
	return s
}
