package paths

import (
	"path/filepath"
	"testing"
)

func TestDataDirUsesOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", override)

	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error = %v", err)
	}
	if got != override {
		t.Fatalf("DataDir() = %q, want %q", got, override)
	}
}

func TestProcessPathsUseDataDir(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", override)

	pidPath, err := ProcessPIDPath("serve")
	if err != nil {
		t.Fatalf("ProcessPIDPath() error = %v", err)
	}
	metaPath, err := ProcessMetaPath("serve")
	if err != nil {
		t.Fatalf("ProcessMetaPath() error = %v", err)
	}
	settingsPath, err := SettingsPath()
	if err != nil {
		t.Fatalf("SettingsPath() error = %v", err)
	}
	desktopPath, err := DesktopSettingsPath()
	if err != nil {
		t.Fatalf("DesktopSettingsPath() error = %v", err)
	}
	if pidPath != filepath.Join(override, "serve.pid") {
		t.Fatalf("pid path = %q", pidPath)
	}
	if metaPath != filepath.Join(override, "serve.json") {
		t.Fatalf("meta path = %q", metaPath)
	}
	if settingsPath != filepath.Join(override, "cometline-settings.json") {
		t.Fatalf("settings path = %q", settingsPath)
	}
	if desktopPath != filepath.Join(override, "cometline-desktop.json") {
		t.Fatalf("desktop path = %q", desktopPath)
	}
}

func TestWikiDirUsesDataDir(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", override)

	got, err := WikiDir()
	if err != nil {
		t.Fatalf("WikiDir() error = %v", err)
	}
	if got != filepath.Join(override, "wiki") {
		t.Fatalf("WikiDir() = %q, want %q/wiki", got, override)
	}
}
