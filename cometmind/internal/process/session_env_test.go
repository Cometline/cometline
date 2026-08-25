package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSessionEnv(t *testing.T, sessionID string, pairs ...string) {
	t.Helper()
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	if err := WriteSessionEnvSnapshot(sessionID, pairs...); err != nil {
		t.Fatal(err)
	}
}

func TestMergeSessionEnvOverlaysValues(t *testing.T) {
	writeSessionEnv(t, "sess1", "CAPTURED_VAR=from-terminal", "PATH=/tmp/term-bin")
	t.Setenv("PATH", "/usr/bin:/bin")

	merged := MergeSessionEnv("sess1", []string{"PATH=/usr/bin:/bin", "FOO=base"})
	if envValue(merged, "CAPTURED_VAR") != "from-terminal" {
		t.Fatalf("missing overlay: %v", merged)
	}
	if envValue(merged, "FOO") != "base" {
		t.Fatalf("lost base: %v", merged)
	}
	path := envValue(merged, "PATH")
	if !strings.Contains(path, "/tmp/term-bin") {
		t.Fatalf("PATH missing terminal entry: %q", path)
	}
	home, _ := os.UserHomeDir()
	if home != "" && !strings.Contains(path, filepath.Join(home, ".cometmind", "bin")) {
		t.Fatalf("PATH missing augmented entry: %q", path)
	}
}

func TestMergeSessionEnvDropsDeniedKeys(t *testing.T) {
	writeSessionEnv(t, "sess1",
		"COMETMIND_API_KEY=from-terminal",
		"COMETLINE_ENV_DIR=/tmp/env",
		"DYLD_INSERT_LIBRARIES=/tmp/evil.dylib",
		"LD_PRELOAD=/tmp/evil.so",
		"PWD=/tmp",
		"OLDPWD=/tmp",
		"SAFE_VAR=ok",
	)
	merged := MergeSessionEnv("sess1", []string{"COMETMIND_API_KEY=from-sidecar", "PATH=/bin"})
	if envValue(merged, "COMETMIND_API_KEY") != "from-sidecar" {
		t.Fatalf("COMETMIND_API_KEY = %q", envValue(merged, "COMETMIND_API_KEY"))
	}
	if envValue(merged, "COMETLINE_ENV_DIR") != "" {
		t.Fatalf("COMETLINE leaked: %v", merged)
	}
	if envValue(merged, "DYLD_INSERT_LIBRARIES") != "" {
		t.Fatalf("DYLD leaked: %v", merged)
	}
	if envValue(merged, "LD_PRELOAD") != "" {
		t.Fatalf("LD_PRELOAD leaked: %v", merged)
	}
	if envValue(merged, "PWD") != "" || envValue(merged, "OLDPWD") != "" {
		t.Fatalf("cwd leaked: %v", merged)
	}
	if envValue(merged, "SAFE_VAR") != "ok" {
		t.Fatalf("SAFE_VAR = %q", envValue(merged, "SAFE_VAR"))
	}
}

func TestMergeSessionEnvMissingFile(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	base := []string{"FOO=1", "PATH=/bin"}
	got := MergeSessionEnv("missing", base)
	if envValue(got, "FOO") != "1" {
		t.Fatalf("got %v", got)
	}
}

func TestMergeSessionEnvRejectsTraversalID(t *testing.T) {
	base := []string{"FOO=1"}
	got := MergeSessionEnv("../etc", base)
	if envValue(got, "FOO") != "1" || len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

func TestRedactSecretValues(t *testing.T) {
	env := []string{
		"AWS_SESSION_TOKEN=supersecrettokenvalue",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG",
		"SAFE=visible",
		"SHORT_TOKEN=ab",
	}
	in := "token=supersecrettokenvalue key=wJalrXUtnFEMI/K7MDENG SAFE=visible SHORT_TOKEN=ab"
	got := RedactSecretValues(in, env)
	if strings.Contains(got, "supersecrettokenvalue") || strings.Contains(got, "wJalrXUtnFEMI/K7MDENG") {
		t.Fatalf("secrets leaked: %q", got)
	}
	if !strings.Contains(got, "SAFE=visible") || !strings.Contains(got, "SHORT_TOKEN=ab") {
		t.Fatalf("over-redacted: %q", got)
	}
}

func TestRedactExplicitAWSKeysOfAnyLength(t *testing.T) {
	got := RedactSecretValues("x=abcd", []string{"AWS_SESSION_TOKEN=abcd"})
	if strings.Contains(got, "abcd") {
		t.Fatalf("explicit AWS token not redacted: %q", got)
	}
}

func TestWithSessionID(t *testing.T) {
	ctx := WithSessionID(t.Context(), "sess1")
	if SessionIDFrom(ctx) != "sess1" {
		t.Fatalf("SessionIDFrom = %q", SessionIDFrom(ctx))
	}
}
