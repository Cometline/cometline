package config

import "testing"

func TestEffectiveStorageConfig_DefaultsWhenUnset(t *testing.T) {
	cfg := &Config{}
	got := cfg.EffectiveStorageConfig()
	if got.RetentionDays != 90 {
		t.Fatalf("retention_days=%d want 90", got.RetentionDays)
	}
	if got.CleanupIntervalMinutes != 60 {
		t.Fatalf("cleanup_interval_minutes=%d want 60", got.CleanupIntervalMinutes)
	}
	if got.ArchivedMemoryPurgeDays != 90 {
		t.Fatalf("archived_memory_purge_days=%d want 90", got.ArchivedMemoryPurgeDays)
	}
	if !got.VacuumAfterPurge {
		t.Fatal("expected vacuum_after_purge default true")
	}
}

func TestEffectiveStorageConfig_ExplicitDisable(t *testing.T) {
	cfg := &Config{Storage: StorageConfig{RetentionDays: 0, ArchivedMemoryPurgeDays: 30, VacuumAfterPurge: true}}
	got := cfg.EffectiveStorageConfig()
	if got.RetentionDays != 0 {
		t.Fatalf("retention_days=%d want 0", got.RetentionDays)
	}
	if got.ArchivedMemoryPurgeDays != 30 {
		t.Fatalf("purge_days=%d want 30", got.ArchivedMemoryPurgeDays)
	}
	if got.CleanupIntervalMinutes != 60 {
		t.Fatalf("cleanup_interval_minutes=%d want 60", got.CleanupIntervalMinutes)
	}
}

func TestEffectiveStorageConfig_DefaultsIncludeRuntimeFiles(t *testing.T) {
	cfg := &Config{}
	got := cfg.EffectiveStorageConfig()
	if got.ToolOutputRetentionDays != 7 {
		t.Fatalf("tool_output_retention_days=%d want 7", got.ToolOutputRetentionDays)
	}
	if got.AgentTmpRetentionDays != 3 {
		t.Fatalf("agent_tmp_retention_days=%d want 3", got.AgentTmpRetentionDays)
	}
}

func TestEffectiveStorageConfig_ExplicitDisableRuntimeFiles(t *testing.T) {
	cfg := &Config{Storage: StorageConfig{
		VacuumAfterPurge:        true,
		ToolOutputRetentionDays: 0,
		AgentTmpRetentionDays:   0,
	}}
	got := cfg.EffectiveStorageConfig()
	if got.ToolOutputRetentionDays != 0 {
		t.Fatalf("tool_output_retention_days=%d want 0", got.ToolOutputRetentionDays)
	}
	if got.AgentTmpRetentionDays != 0 {
		t.Fatalf("agent_tmp_retention_days=%d want 0", got.AgentTmpRetentionDays)
	}
}

func TestAdaptStorageConfig_OmittedRuntimeKeysGetDefaults(t *testing.T) {
	got := adaptStorageConfig(cometlineStorageJSON{
		RetentionDays:    90,
		VacuumAfterPurge: true,
	})
	if got.ToolOutputRetentionDays != 7 {
		t.Fatalf("tool_output_retention_days=%d want 7", got.ToolOutputRetentionDays)
	}
	if got.AgentTmpRetentionDays != 3 {
		t.Fatalf("agent_tmp_retention_days=%d want 3", got.AgentTmpRetentionDays)
	}
}

func TestAdaptStorageConfig_ExplicitZeroDisables(t *testing.T) {
	zero := 0
	got := adaptStorageConfig(cometlineStorageJSON{
		RetentionDays:           90,
		VacuumAfterPurge:        true,
		ToolOutputRetentionDays: &zero,
		AgentTmpRetentionDays:   &zero,
	})
	if got.ToolOutputRetentionDays != 0 {
		t.Fatalf("tool_output_retention_days=%d want 0", got.ToolOutputRetentionDays)
	}
	if got.AgentTmpRetentionDays != 0 {
		t.Fatalf("agent_tmp_retention_days=%d want 0", got.AgentTmpRetentionDays)
	}
}
