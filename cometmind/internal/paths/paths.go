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

// SettingsPath returns ~/.cometmind/cometline-settings.json (agent-editable runtime settings).
func SettingsPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometline-settings.json"), nil
}

// DesktopSettingsPath returns ~/.cometmind/cometline-desktop.json (Electron-only UI state).
func DesktopSettingsPath() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cometline-desktop.json"), nil
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

// LogsDir returns ~/.cometmind/logs (created if missing).
func LogsDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ToolOutputDir returns ~/.cometmind/tool-output (created if missing).
func ToolOutputDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "tool-output")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// AgentTmpDir returns ~/.cometmind/agent-tmp (created if missing).
func AgentTmpDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "agent-tmp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// WikiDir returns ~/.cometmind/wiki (created if missing).
// Persistent LLM Wiki storage; not age-purged by retention.
func WikiDir() (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "wiki")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
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

// ProcessReloadResultPath returns the path a long-lived process writes its most
// recent settings-reload outcome to, so a short-lived CLI invocation (e.g.
// `cometmind settings reload`) can confirm the reload actually completed
// instead of merely delivering a SIGHUP.
func ProcessReloadResultPath(mode string) (string, error) {
	d, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, mode+".reload.json"), nil
}
