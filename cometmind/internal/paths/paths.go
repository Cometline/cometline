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

// DataDir returns ~/.cometmind (created if missing).
func DataDir() (string, error) {
	if raw := strings.TrimSpace(os.Getenv(dataDirEnv)); raw != "" {
		if err := os.MkdirAll(raw, 0o700); err != nil {
			return "", err
		}
		return raw, nil
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

// LegacyConfigPath returns ~/.cometmind/config.toml or the overridden data dir equivalent.
func LegacyConfigPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.toml"), nil
}

// SettingsPath returns ~/.cometmind/cometline-settings.json.
func SettingsPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometline-settings.json"), nil
}

// ConfigPath returns ~/.cometmind/cometline-settings.json (legacy name retained for callers).
func ConfigPath() (string, error) {
	return SettingsPath()
}

// DBPath returns ~/.cometmind/cometmind.db.
func DBPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometmind.db"), nil
}

// MCPOAuthDir returns ~/.cometmind/mcp-oauth (created if missing).
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

// ProcessPIDPath returns the pidfile path for one long-lived process mode.
func ProcessPIDPath(mode string) (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, mode+".pid"), nil
}

// ProcessMetaPath returns the JSON metadata path for one long-lived process mode.
func ProcessMetaPath(mode string) (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, mode+".json"), nil
}
