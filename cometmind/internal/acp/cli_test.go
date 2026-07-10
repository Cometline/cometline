package acp

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type testCloser struct {
	err   error
	close bool
}

func (c *testCloser) Close() error {
	c.close = true
	return c.err
}

func TestParseCLIEventClaudeStream(t *testing.T) {
	message := parseCLIEvent(HarnessClaude, `{"type":"assistant","message":{"content":[{"type":"text","text":"updated files"}]}}`)
	if message.Update.Kind != "message" || message.Update.Content != "updated files" {
		t.Fatalf("message = %#v", message.Update)
	}

	result := parseCLIEvent(HarnessClaude, `{"type":"result","result":"all tests pass"}`)
	if result.Final != "all tests pass" {
		t.Fatalf("final = %q", result.Final)
	}
}

func TestParseCLIEventCodexJSONL(t *testing.T) {
	event := parseCLIEvent(HarnessCodex, `{"type":"item.completed","item":{"type":"agent_message","text":"implemented the fix"}}`)
	if event.Update.Content != "implemented the fix" {
		t.Fatalf("event = %#v", event.Update)
	}

	tool := parseCLIEvent(HarnessCodex, `{"type":"item.started","item":{"type":"command_execution","command":"go test ./..."}}`)
	if tool.Update.Kind != "tool_call" || tool.Update.Title != "go test ./..." {
		t.Fatalf("tool = %#v", tool.Update)
	}
}

func TestParseCLIEventOpenCodeJSON(t *testing.T) {
	message := parseCLIEvent(HarnessOpenCode, `{"type":"text","part":{"type":"text","text":"implemented the fix"}}`)
	if message.Update.Kind != "message" || message.Update.Content != "implemented the fix" {
		t.Fatalf("message = %#v", message.Update)
	}

	tool := parseCLIEvent(HarnessOpenCode, `{"type":"tool_use","part":{"type":"tool","tool":"bash","state":{"status":"completed"}}}`)
	if tool.Update.Kind != "tool_call" || tool.Update.Title != "bash" || tool.Update.Status != "completed" {
		t.Fatalf("tool = %#v", tool.Update)
	}

	errorEvent := parseCLIEvent(HarnessOpenCode, `{"type":"error","error":{"name":"Error","message":"permission denied"}}`)
	if errorEvent.Update.Kind != "status" || errorEvent.Update.Status != "failed" || errorEvent.Update.Title != "permission denied" {
		t.Fatalf("error = %#v", errorEvent.Update)
	}
}

func TestFixedHarnessCLIProfiles(t *testing.T) {
	tests := []struct {
		harness Harness
		command string
		args    []string
	}{
		{
			harness: HarnessOpenCode,
			command: "opencode",
			args:    []string{"run", "--format", "json", "--auto"},
		},
		{
			harness: HarnessClaude,
			command: "claude",
			args:    []string{"-p", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"},
		},
		{
			harness: HarnessCodex,
			command: "codex",
			args:    []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox"},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.harness), func(t *testing.T) {
			command, args := (Config{Harness: tt.harness}).commandArgs()
			if command != tt.command {
				t.Fatalf("command = %q, want %q", command, tt.command)
			}
			if strings.Join(args, "\x00") != strings.Join(tt.args, "\x00") {
				t.Fatalf("args = %#v, want %#v", args, tt.args)
			}
		})
	}
}

func TestConfigCommandAvailableUsesFixedHarnessCommand(t *testing.T) {
	var resolved string
	cfg := Config{Harness: HarnessCodex}
	if !cfg.commandAvailableWithResolver(func(command string) (string, error) {
		resolved = command
		return "/tmp/" + command, nil
	}) {
		t.Fatal("expected an available command to be reported")
	}
	if resolved != "codex" {
		t.Fatalf("resolved command = %q, want codex", resolved)
	}

	if (Config{Harness: HarnessOpenCode}).commandAvailableWithResolver(func(string) (string, error) {
		return "", errors.New("command not found")
	}) {
		t.Fatal("expected a missing command to be reported as unavailable")
	}
}

func TestSessionManagerRunCLI(t *testing.T) {
	t.Parallel()

	var gotPrompt string
	var progress []ProgressUpdate
	closer := &testCloser{}
	mgr := NewSessionManager(Config{
		Harness: HarnessClaude,
		Timeout: time.Minute,
	})
	mgr.CLIProcessStarter = func(
		ctx context.Context,
		cfg Config,
		workspaceRoot string,
		prompt string,
	) (io.ReadCloser, io.ReadCloser, io.Closer, error) {
		gotPrompt = prompt
		return io.NopCloser(strings.NewReader(`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]}}
{"type":"result","result":"done"}
`)), io.NopCloser(strings.NewReader("")), closer, nil
	}

	result, err := mgr.Run(context.Background(), RunOptions{
		ChildSessionID: "child-cli",
		WorkspaceRoot:  t.TempDir(),
		Task:           "fix the parser",
		Context:        "Use the existing tests",
		OnProgress: func(update ProgressUpdate) {
			progress = append(progress, update)
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != "completed" || result.AgentName != "Claude Code" {
		t.Fatalf("result = %#v", result)
	}
	if result.Summary != "done" {
		t.Fatalf("summary = %q", result.Summary)
	}
	if !strings.Contains(gotPrompt, "Use the existing tests\n\nTask:\nfix the parser") {
		t.Fatalf("prompt = %q", gotPrompt)
	}
	if len(progress) != 1 || progress[0].Content != "working" {
		t.Fatalf("progress = %#v", progress)
	}
	if !closer.close {
		t.Fatal("expected CLI closer to be called")
	}
}

func TestSessionManagerRunCLIFailureIncludesStderr(t *testing.T) {
	mgr := NewSessionManager(Config{Harness: HarnessCodex, Timeout: time.Minute})
	mgr.CLIProcessStarter = func(
		ctx context.Context,
		cfg Config,
		workspaceRoot string,
		prompt string,
	) (io.ReadCloser, io.ReadCloser, io.Closer, error) {
		return io.NopCloser(strings.NewReader("")), io.NopCloser(strings.NewReader("codex failed")), &testCloser{err: errors.New("exit status 1")}, nil
	}

	result, err := mgr.Run(context.Background(), RunOptions{WorkspaceRoot: t.TempDir(), Task: "run"})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Status != "failed" || result.AgentName != "Codex" || result.Summary != "codex failed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}
