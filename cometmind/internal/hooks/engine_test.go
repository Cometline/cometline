package hooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/config"
)

type stubRunner struct {
	calls int
	fn    func(env ToolEnv) Outcome
}

func (s *stubRunner) Run(_ context.Context, _ resolvedHandler, env ToolEnv, _ string) (Outcome, error) {
	s.calls++
	if s.fn != nil {
		return s.fn(env), nil
	}
	return Outcome{}, nil
}

func TestMatcher(t *testing.T) {
	if !matcherMatches("run_command", "run_command") {
		t.Fatal("exact match")
	}
	if !matcherMatches("", "grep") {
		t.Fatal("empty matches all")
	}
	if !matcherMatches("run_.*", "run_command") {
		t.Fatal("regex match")
	}
	if matcherMatches("read_file", "run_command") {
		t.Fatal("should not match")
	}
}

func TestModifyRunCommand(t *testing.T) {
	base := json.RawMessage(`{"command":"git status","timeout_sec":10}`)
	patch := json.RawMessage(`{"command":"rtk git status"}`)
	out := mergeToolInput(base, patch)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["command"] != "rtk git status" {
		t.Fatalf("command = %v", m["command"])
	}
	if m["timeout_sec"].(float64) != 10 {
		t.Fatalf("timeout preserved: %v", m["timeout_sec"])
	}
}

func TestPreToolUseBlock(t *testing.T) {
	runner := &stubRunner{fn: func(_ ToolEnv) Outcome {
		return Outcome{Block: true, BlockReason: "nope"}
	}}
	cfg := config.HooksConfig{Enabled: true}
	eng := NewEngine(cfg, t.TempDir()).WithRunner(runner)
	eng.handlers = []resolvedHandler{{
		Event: EventPreToolUse, Matcher: "run_command", Command: "echo",
	}}
	pre := eng.RunPreToolUse(context.Background(), ToolEnv{ToolName: "run_command", ToolInput: json.RawMessage(`{}`)})
	if !pre.Blocked || pre.BlockReason != "nope" {
		t.Fatalf("pre = %+v", pre)
	}
	if runner.calls != 1 {
		t.Fatalf("calls = %d", runner.calls)
	}
}

func TestDisabledNoOp(t *testing.T) {
	runner := &stubRunner{}
	eng := NewEngine(config.HooksConfig{Enabled: false}, t.TempDir()).WithRunner(runner)
	eng.handlers = []resolvedHandler{{Event: EventPreToolUse, Matcher: "", Command: "x"}}
	pre := eng.RunPreToolUse(context.Background(), ToolEnv{ToolName: "run_command"})
	if pre.Blocked || runner.calls != 0 {
		t.Fatalf("disabled should no-op: blocked=%v calls=%d", pre.Blocked, runner.calls)
	}
}

func TestProjectHooksRequireTrust(t *testing.T) {
	dir := t.TempDir()
	projectHooks := filepath.Join(dir, ".cometmind", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(projectHooks), 0o700); err != nil {
		t.Fatal(err)
	}
	payload := `{"hooks":{"PreToolUse":[{"matcher":"run_command","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	if err := os.WriteFile(projectHooks, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(dir)
	untrusted := NewEngine(config.HooksConfig{Enabled: true}, abs)
	if len(untrusted.handlers) != 0 {
		t.Fatalf("untrusted workspace should skip project hooks, got %d handlers", len(untrusted.handlers))
	}
	trusted := NewEngine(config.HooksConfig{Enabled: true, TrustedWorkspacePaths: []string{abs}}, abs)
	if len(trusted.handlers) == 0 {
		t.Fatal("trusted workspace should load project hooks")
	}
}

func TestBrokenHooksJSONLogsAndSkips(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	hooksPath := filepath.Join(dataDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(config.HooksConfig{Enabled: true}, t.TempDir())
	if len(eng.handlers) != 0 {
		t.Fatalf("expected no handlers from broken config, got %d", len(eng.handlers))
	}
}

func TestDiscoverGlobalHooks(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("COMETMIND_DATA_DIR", dataDir)
	hooksPath := filepath.Join(dataDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(`{"hooks":{"PostToolUse":[{"matcher":"","hooks":[{"type":"command","command":"true"}]}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(config.HooksConfig{Enabled: true}, t.TempDir())
	found := false
	for _, h := range eng.handlers {
		if h.Event == EventPostToolUse {
			found = true
		}
	}
	if !found {
		t.Fatal("expected global PostToolUse hook")
	}
}

func TestExecutorBadJSON(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "bad.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho not-json\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := execRunner{}
	out, err := runner.Run(context.Background(), resolvedHandler{Command: script}, ToolEnv{HookEvent: EventPreToolUse, ToolName: "run_command"}, dir)
	if err == nil {
		t.Fatal("expected bad json error")
	}
	if out.Block {
		t.Fatal("bad json should not block")
	}
}

func TestExecutorExitCode2Blocks(t *testing.T) {
	dir := t.TempDir()
	runner := execRunner{}
	out, err := runner.Run(context.Background(), resolvedHandler{Command: "exit 2"}, ToolEnv{HookEvent: EventPreToolUse}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Block {
		t.Fatal("exit 2 should block")
	}
}

func TestExecutorTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	dir := t.TempDir()
	runner := execRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_, err := runner.Run(ctx, resolvedHandler{Command: "sleep 2", TimeoutSec: 1}, ToolEnv{HookEvent: EventPreToolUse}, dir)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
