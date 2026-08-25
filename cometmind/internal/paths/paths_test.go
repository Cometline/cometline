package paths

import (
	"errors"
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

func TestTerminalEnvFileUsesDataDir(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", override)

	got, err := TerminalEnvFile("sess_abc")
	if err != nil {
		t.Fatalf("TerminalEnvFile() error = %v", err)
	}
	if got != filepath.Join(override, "terminal-env", "sess_abc", "environ") {
		t.Fatalf("TerminalEnvFile() = %q", got)
	}
}

func TestTerminalEnvFileRejectsUnsafeID(t *testing.T) {
	if _, err := TerminalEnvFile("../etc"); !errors.Is(err, ErrInvalidSessionID) {
		t.Fatalf("err = %v, want ErrInvalidSessionID", err)
	}
}

func TestIsTerminalEnvPath(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", override)
	file := filepath.Join(override, "terminal-env", "sess_abc", "environ")
	if !IsTerminalEnvPath(file) {
		t.Fatal("expected env file to be private")
	}
	if !IsTerminalEnvPath(filepath.Join(override, "terminal-env")) {
		t.Fatal("expected env root to be private")
	}
	if IsTerminalEnvPath(filepath.Join(override, "tool-output", "x.txt")) {
		t.Fatal("tool-output should not be treated as terminal-env")
	}
}

func TestCommandMentionsTerminalEnv(t *testing.T) {
	override := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", override)
	if !CommandMentionsTerminalEnv("cat " + filepath.Join(override, "terminal-env", "sess", "environ")) {
		t.Fatal("expected absolute snapshot path to be detected")
	}
	if !CommandMentionsTerminalEnv("cat ~/.cometmind/terminal-env/sess/environ") {
		t.Fatal("expected home-relative snapshot path to be detected")
	}
	if CommandMentionsTerminalEnv("echo hello") {
		t.Fatal("plain command should not match")
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
