package paths

import (
	"os"
	"path/filepath"
)

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
