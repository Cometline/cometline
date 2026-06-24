package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDirUsesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(dataDirEnv, dir)

	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error = %v", err)
	}
	if got != dir {
		t.Fatalf("DataDir() = %q, want %q", got, dir)
	}

	settings, err := SettingsPath()
	if err != nil {
		t.Fatalf("SettingsPath() error = %v", err)
	}
	if settings != filepath.Join(dir, "cometline-settings.json") {
		t.Fatalf("SettingsPath() = %q", settings)
	}

	skills, err := SkillsDir()
	if err != nil {
		t.Fatalf("SkillsDir() error = %v", err)
	}
	if skills != filepath.Join(dir, "skills") {
		t.Fatalf("SkillsDir() = %q", skills)
	}
	if _, err := os.Stat(skills); err != nil {
		t.Fatalf("SkillsDir() not created: %v", err)
	}
}

func TestDataDirDefaultsToHomeDotCometmind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(dataDirEnv, "")

	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error = %v", err)
	}
	want := filepath.Join(home, ".cometmind")
	if got != want {
		t.Fatalf("DataDir() = %q, want %q", got, want)
	}
}
