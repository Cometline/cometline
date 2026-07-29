package config

import "testing"

func TestEffectiveInboxSettingsUsesDoubledDefaultStepBudget(t *testing.T) {
	got := (&Config{}).EffectiveInboxSettings()
	if got.MaxStepsPerRun != 16 {
		t.Fatalf("MaxStepsPerRun = %d, want 16", got.MaxStepsPerRun)
	}
}
