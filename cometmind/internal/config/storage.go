package config

// StorageConfig controls automatic session retention and memory purge.
type StorageConfig struct {
	// RetentionDays deletes sessions with no activity for this many days. 0 disables.
	RetentionDays int `json:"retention_days" mapstructure:"retention_days"`
	// MaxSessionsPerWorkspace keeps only the N most recently updated sessions per workspace. 0 disables.
	MaxSessionsPerWorkspace int `json:"max_sessions_per_workspace" mapstructure:"max_sessions_per_workspace"`
	// ArchivedMemoryPurgeDays hard-deletes archived memories older than this many days. 0 disables.
	ArchivedMemoryPurgeDays int `json:"archived_memory_purge_days" mapstructure:"archived_memory_purge_days"`
	// VacuumAfterPurge runs SQLite VACUUM after any purge deleted rows.
	VacuumAfterPurge bool `json:"vacuum_after_purge" mapstructure:"vacuum_after_purge"`
}

func defaultStorageConfig() StorageConfig {
	return StorageConfig{
		RetentionDays:           90,
		MaxSessionsPerWorkspace: 0,
		ArchivedMemoryPurgeDays: 90,
		VacuumAfterPurge:        true,
	}
}

// RetentionEnabled reports whether any session retention rule is active.
func (s StorageConfig) RetentionEnabled() bool {
	return s.RetentionDays > 0 || s.MaxSessionsPerWorkspace > 0
}

// MemoryPurgeEnabled reports whether archived memory purge is active.
func (s StorageConfig) MemoryPurgeEnabled() bool {
	return s.ArchivedMemoryPurgeDays > 0
}

// EffectiveStorageConfig returns storage settings with defaults when nothing is configured.
func (c *Config) EffectiveStorageConfig() StorageConfig {
	if !c.storageConfigured() {
		return defaultStorageConfig()
	}
	return c.Storage
}

func (c *Config) storageConfigured() bool {
	s := c.Storage
	return s.RetentionDays != 0 ||
		s.MaxSessionsPerWorkspace != 0 ||
		s.ArchivedMemoryPurgeDays != 0 ||
		s.VacuumAfterPurge
}
