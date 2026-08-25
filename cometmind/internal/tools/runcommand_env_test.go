package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/process"
)

func writeTerminalEnv(t *testing.T, sessionID string, pairs ...string) {
	t.Helper()
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	if err := process.WriteSessionEnvSnapshot(sessionID, pairs...); err != nil {
		t.Fatal(err)
	}
}

func TestRunCommandMergesSessionEnv(t *testing.T) {
	writeTerminalEnv(t, "sess_env", "CAPTURED_VAR=from-terminal")
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(
		WithToolSession(t.Context(), "sess_env"),
		json.RawMessage(`{"command":"printf %s \"$CAPTURED_VAR\""}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Output != "from-terminal" {
		t.Fatalf("result = %+v", res)
	}
}

func TestRunCommandRedactsSessionSecrets(t *testing.T) {
	const token = "supersecrettokenvalue"
	writeTerminalEnv(t, "sess_env", "AWS_SESSION_TOKEN="+token)
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	res, err := tool.Execute(
		WithToolSession(t.Context(), "sess_env"),
		json.RawMessage(`{"command":"printf %s \"$AWS_SESSION_TOKEN\""}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %+v", res)
	}
	if strings.Contains(res.Output, token) {
		t.Fatalf("token leaked: %q", res.Output)
	}
	if !strings.Contains(res.Output, "[redacted]") {
		t.Fatalf("expected redaction, got %q", res.Output)
	}
}

func TestRunCommandRejectsTerminalEnvPath(t *testing.T) {
	data := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", data)
	tool := RunCommand{Workspace: Workspace{Root: t.TempDir()}}
	cmd, _ := json.Marshal(map[string]string{
		"command": "cat " + data + "/terminal-env/sess/environ",
	})
	res, err := tool.Execute(WithToolSession(t.Context(), "sess"), cmd)
	if err != nil {
		t.Fatal(err)
	}
	if res.OK || !strings.Contains(res.Output, "private") {
		t.Fatalf("result = %+v, want private", res)
	}
}
