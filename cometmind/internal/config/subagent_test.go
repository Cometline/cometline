package config

import "testing"

func TestGeneralMaxStepsFromMain(t *testing.T) {
	tests := []struct {
		main int
		want int
	}{
		{100, 80},
		{25, 20},
		{10, 8},
		{0, 80},
	}
	for _, tc := range tests {
		if got := GeneralMaxStepsFromMain(tc.main); got != tc.want {
			t.Fatalf("GeneralMaxStepsFromMain(%d)=%d want %d", tc.main, got, tc.want)
		}
	}
}

func TestEffectiveSubagentSettingsDefaults(t *testing.T) {
	cfg := &Config{}
	got := cfg.EffectiveSubagentSettings()
	if got.GeneralMaxSteps != 80 {
		t.Fatalf("GeneralMaxSteps=%d want 80 (80%% of main default 100)", got.GeneralMaxSteps)
	}
	if got.MaxConcurrentPerParent != 5 {
		t.Fatalf("MaxConcurrentPerParent=%d want 5", got.MaxConcurrentPerParent)
	}
}

func TestEffectiveSubagentSettingsFollowsMainMaxSteps(t *testing.T) {
	cfg := &Config{MaxSteps: 30}
	got := cfg.EffectiveSubagentSettings()
	if got.GeneralMaxSteps != 24 {
		t.Fatalf("GeneralMaxSteps=%d want 24", got.GeneralMaxSteps)
	}
}

func TestEffectiveSubagentSettingsExplicitOverride(t *testing.T) {
	cfg := &Config{
		MaxSteps: 50,
		Subagent: SubagentSettings{GeneralMaxSteps: 12},
	}
	got := cfg.EffectiveSubagentSettings()
	if got.GeneralMaxSteps != 12 {
		t.Fatalf("GeneralMaxSteps=%d want explicit 12", got.GeneralMaxSteps)
	}
}
