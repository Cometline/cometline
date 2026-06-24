package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const dataDirEnv = "COMETMIND_DATA_DIR"

// Home returns the user's home directory or an error if unset.
func Home() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return h, nil
}

// DataDir returns the CometMind data root (created if missing).
// When COMETMIND_DATA_DIR is set, that path is used; otherwise ~/.cometmind.
func DataDir() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv(dataDirEnv)); explicit != "" {
		dir := filepath.Clean(explicit)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		return dir, nil
	}
	h, err := Home()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(h, ".cometmind")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SettingsPath returns cometline-settings.json under DataDir.
func SettingsPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometline-settings.json"), nil
}

// ConfigPath returns cometline-settings.json (legacy name retained for callers).
func ConfigPath() (string, error) {
	return SettingsPath()
}

// DBPath returns cometmind.db under DataDir.
func DBPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometmind.db"), nil
}

// MCPOAuthDir returns mcp-oauth under DataDir (created if missing).
func MCPOAuthDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "mcp-oauth")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SkillsDir returns the managed skills directory under DataDir (created if missing).
func SkillsDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "skills")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// BuiltinSkillsDir returns the materialized bundled-skills root under DataDir.
func BuiltinSkillsDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "builtin-skills")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// WorkspaceStorePath returns cometline-workspace.json under DataDir.
func WorkspaceStorePath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometline-workspace.json"), nil
}
