package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateDiscordGateway(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".cometmind")
	if err := os.MkdirAll(settingsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	base := `{"providers":[{"id":"anthropic","name":"Anthropic","method":"anthropic","enabled":true,"enabledModels":["claude-sonnet-4-5"],"models":["claude-sonnet-4-5"]}],"cometmind":{"gateway":{"discord":{"enabled":false,"botTokenEnv":"DISCORD_BOT_TOKEN"}}}}`
	if err := os.WriteFile(filepath.Join(settingsDir, "cometline-settings.json"), []byte(base), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("HOME", dir)

	enabled := true
	workspace := "/workspace"
	if err := UpdateDiscordGateway(DiscordGatewayPatch{
		Enabled:       &enabled,
		WorkspacePath: &workspace,
		AllowedUsers:  []string{"user-1"},
	}); err != nil {
		t.Fatalf("UpdateDiscordGateway() error = %v", err)
	}

	cfg, err := LoadDiscordGateway()
	if err != nil {
		t.Fatalf("LoadDiscordGateway() error = %v", err)
	}
	if !cfg.Enabled || cfg.WorkspacePath != "/workspace" {
		t.Fatalf("LoadDiscordGateway() = %+v", cfg)
	}
	if len(cfg.AllowedUsers) != 1 || cfg.AllowedUsers[0] != "user-1" {
		t.Fatalf("AllowedUsers = %+v", cfg.AllowedUsers)
	}
}

func TestValidateCurrentSettings(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".cometmind")
	if err := os.MkdirAll(settingsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	data, err := os.ReadFile("testdata/cometline-settings.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "cometline-settings.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("HOME", dir)

	if err := ValidateCurrentSettings(); err != nil {
		t.Fatalf("ValidateCurrentSettings() error = %v", err)
	}
}

func TestValidateCurrentSettingsMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	err := ValidateCurrentSettings()
	if err == nil {
		t.Fatal("ValidateCurrentSettings() error = nil, want missing settings")
	}
	if !strings.Contains(err.Error(), "settings file does not exist") {
		t.Fatalf("error = %v", err)
	}
}
