package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RunCommand runs a shell command with cwd set to the workspace root.
//
// This is a convenience tool with best-effort guardrails, not a hard process
// sandbox. A few obviously destructive patterns are rejected, but callers
// should not rely on this as a complete security boundary.
type RunCommand struct{ Workspace Workspace }

func (RunCommand) Spec() ToolSpec {
	return ToolSpec{
		Name:        "run_command",
		Description: "Run a shell command in the workspace root. Best-effort guardrails reject a few obviously dangerous patterns.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Command with arguments (shell interpretation)"}},"required":["command"]}`),
	}
}

func (r RunCommand) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Command *string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	command, bad, ok := requiredTrimmedString(in.Command, "command")
	if !ok {
		return bad, nil
	}
	if err := denylistCheck(command); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if _, err := r.Workspace.Resolve("."); err != nil {
		return Result{}, err
	}
	root := filepath.Clean(r.Workspace.Root)

	// Acquire a per-workspace mutex so concurrent sessions do not run
	// conflicting shell commands (e.g. git commit, go test) simultaneously
	// against the same workspace root.
	release := acquireWorkspaceLock(root)
	defer release()

	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command) //nolint:gosec
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	text := string(out)

	var exit *int
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			c := ee.ExitCode()
			exit = &c
			return Result{OK: false, Output: text, ExitCode: exit}, nil
		}
		return Result{OK: false, Output: text + err.Error()}, nil
	}
	c := 0
	exit = &c
	return Result{OK: true, Output: text, ExitCode: exit}, nil
}

var deniedCommandPatterns = []struct {
	re  *regexp.Regexp
	msg string
}{
	{re: regexp.MustCompile(`(^|[^[:alnum:]_./-])mkfs([^[:alnum:]_./-]|$)`), msg: "command rejected by safety guardrail (matched mkfs)"},
	{re: regexp.MustCompile(`\bdd\s+if=`), msg: "command rejected by safety guardrail (matched dd if=)"},
	{re: regexp.MustCompile(`>\s*/dev/`), msg: "command rejected by safety guardrail (matched > /dev/)"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_./-])sudo\s+rm([^[:alnum:]_./-]|$)`), msg: "command rejected by safety guardrail (matched sudo rm)"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_./-])rm\s+-rf\s+/([^[:alnum:]_./-]|$)`), msg: "command rejected by safety guardrail (matched rm -rf /)"},
}

func denylistCheck(cmd string) error {
	c := strings.ToLower(cmd)
	for _, pattern := range deniedCommandPatterns {
		if pattern.re.MatchString(c) {
			return fmt.Errorf("%s", pattern.msg)
		}
	}
	return nil
}
