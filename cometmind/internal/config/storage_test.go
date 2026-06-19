package config

import "testing"

func TestEffectiveStorageConfig_DefaultsWhenUnset(t *testing.T) {
	cfg := &Config{}
	got := cfg.EffectiveStorageConfig()
	if got.RetentionDays != 90 {
		t.Fatalf("retention_days=%d want 90", got.RetentionDays)
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
}
