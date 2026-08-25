package process

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
)

const (
	minRedactValueLen = 8
	redactedValue     = "[redacted]"
)

// EnvForSession returns Env() overlaid with the desktop terminal snapshot for sessionID.
// A missing or unreadable snapshot is ignored.
func EnvForSession(sessionID string) []string {
	return MergeSessionEnv(sessionID, Env())
}

// MergeSessionEnv overlays a terminal env snapshot onto base.
// COMETMIND_*, DYLD_*, and LD_PRELOAD from the snapshot are dropped.
// PATH is re-augmented after the overlay.
func MergeSessionEnv(sessionID string, base []string) []string {
	extra, ok := readSessionEnv(sessionID)
	if !ok || len(extra) == 0 {
		return base
	}
	merged := overlayEnv(base, extra)
	return replacePath(merged, AugmentedPath(envValue(merged, "PATH")))
}

func readSessionEnv(sessionID string) (map[string]string, bool) {
	if !paths.ValidSessionID(sessionID) {
		return nil, false
	}
	file, err := paths.TerminalEnvFile(sessionID)
	if err != nil {
		return nil, false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}
	return parseEnvironFile(data), true
}

func parseEnvironFile(data []byte) map[string]string {
	out := make(map[string]string)
	for _, entry := range bytes.Split(data, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		key, value, ok := strings.Cut(string(entry), "=")
		if !ok || key == "" || !allowSessionEnvKey(key) {
			continue
		}
		out[key] = value
	}
	return out
}

var deniedSessionEnvKeys = map[string]struct{}{
	"BASH_ENV":        {},
	"LD_LIBRARY_PATH": {},
	"LD_PRELOAD":      {},
	"LINENO":          {},
	"OLDPWD":          {},
	"PROMPT":          {},
	"PS1":             {},
	"PWD":             {},
	"RPROMPT":         {},
	"SHLVL":           {},
	"ZDOTDIR":         {},
	"_":               {},
}

func allowSessionEnvKey(key string) bool {
	if _, denied := deniedSessionEnvKeys[key]; denied {
		return false
	}
	if strings.HasPrefix(key, "COMETMIND_") || strings.HasPrefix(key, "COMETLINE_") || strings.HasPrefix(key, "DYLD_") {
		return false
	}
	return true
}

func overlayEnv(base []string, extra map[string]string) []string {
	seen := make(map[string]int, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		key, _, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			continue
		}
		if idx, exists := seen[key]; exists {
			out[idx] = kv
			continue
		}
		seen[key] = len(out)
		out = append(out, kv)
	}
	for key, value := range extra {
		kv := key + "=" + value
		if idx, exists := seen[key]; exists {
			out[idx] = kv
			continue
		}
		seen[key] = len(out)
		out = append(out, kv)
	}
	return out
}

func replacePath(env []string, path string) []string {
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + path
			return env
		}
	}
	return append(env, "PATH="+path)
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

func isExplicitSecretKey(key string) bool {
	switch strings.ToUpper(key) {
	case "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_SECURITY_TOKEN":
		return true
	default:
		return false
	}
}

func isGenericSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "API_KEY")
}

func isSecretEnvKey(key string) bool {
	return isExplicitSecretKey(key) || isGenericSecretKey(key)
}

// RedactSecretValues replaces secret-looking values from env in text.
func RedactSecretValues(text string, env []string) string {
	if text == "" || len(env) == 0 {
		return text
	}
	values := secretValues(env)
	if len(values) == 0 {
		return text
	}
	for _, value := range values {
		text = strings.ReplaceAll(text, value, redactedValue)
	}
	return text
}

// WriteSessionEnvSnapshot writes a null-delimited KEY=VALUE snapshot for tests and tools.
func WriteSessionEnvSnapshot(sessionID string, pairs ...string) error {
	file, err := paths.TerminalEnvFile(sessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	var b bytes.Buffer
	for _, pair := range pairs {
		b.WriteString(pair)
		b.WriteByte(0)
	}
	return os.WriteFile(file, b.Bytes(), 0o600)
}

func secretValues(env []string) []string {
	seen := make(map[string]struct{})
	var values []string
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || value == "" || !isSecretEnvKey(key) {
			continue
		}
		if !isExplicitSecretKey(key) && len(value) < minRedactValueLen {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return len(values[i]) > len(values[j])
	})
	return values
}
