package config

import "testing"

func TestEffectiveInboxSettingsFollowsMainMaxSteps(t *testing.T) {
	got := (&Config{}).EffectiveInboxSettings()
	if got.MaxStepsPerRun != Defaults().MaxSteps {
		t.Fatalf("MaxStepsPerRun = %d, want main default %d", got.MaxStepsPerRun, Defaults().MaxSteps)
	}

	custom := (&Config{MaxSteps: 40}).EffectiveInboxSettings()
	if custom.MaxStepsPerRun != 40 {
		t.Fatalf("MaxStepsPerRun = %d, want 40", custom.MaxStepsPerRun)
	}

	explicit := (&Config{MaxSteps: 40, Inbox: InboxConfig{MaxStepsPerRun: 12}}).EffectiveInboxSettings()
	if explicit.MaxStepsPerRun != 12 {
		t.Fatalf("MaxStepsPerRun = %d, want explicit 12", explicit.MaxStepsPerRun)
	}
}
