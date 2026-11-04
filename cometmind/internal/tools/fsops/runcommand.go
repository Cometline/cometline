package fsops

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

	"github.com/Cometline/cometline/cometmind/internal/paths"
	"github.com/Cometline/cometline/cometmind/internal/process"
	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// RunCommand runs a shell command with cwd set to the workspace root.
//
// This is a convenience tool with best-effort guardrails, not a hard process
// sandbox. A few obviously destructive patterns are rejected, but callers
// should not rely on this as a complete security boundary.
type RunCommand struct{ Workspace Workspace }

const (
	runCommandDefaultTimeoutSec = 300
	runCommandMaxTimeoutSec     = 1800
)

func (RunCommand) Spec() ToolSpec {
	return ToolSpec{
		Name: "run_command",
		Description: "Run a shell command in the workspace root. " +
			"Best-effort guardrails reject a few obviously dangerous patterns. " +
			"Default timeout is 300s; pass timeout_sec to override (max 1800). " +
			"Very large output is truncated in the tool result; the full text is saved under @runtime/tool-output/ and the result includes that alias. " +
			"Read it with read_file. Do not redirect to @runtime. " +
			"The shell does not expand @runtime; a command containing @runtime is rejected. " +
			"To pass a file to an external program, use $HOME/.cometmind/agent-tmp, $HOME/.cometmind/tool-output, or $HOME/.cometmind/wiki.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"command":{"type":"string","description":"Command with arguments (shell interpretation). Do not include @runtime; the shell does not expand that alias."},
				"timeout_sec":{"type":"integer","description":"Optional timeout in seconds (default 300, max 1800)"}
			},
			"required":["command"]
		}`),
	}
}

func (r RunCommand) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Command    *string `json:"command"`
		TimeoutSec *int    `json:"timeout_sec"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	command, bad, ok := toolkit.RequiredTrimmedString(in.Command, "command")
	if !ok {
		return bad, nil
	}
	if err := denylistCheck(command); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if err := rejectRuntimeAliasInShell(command); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if paths.CommandMentionsTerminalEnv(command) {
		return Result{OK: false, Output: "path is private"}, nil
	}
	if _, err := r.Workspace.Resolve("."); err != nil {
		return Result{}, err
	}
	root := filepath.Clean(r.Workspace.Root)

	timeoutSec := runCommandDefaultTimeoutSec
	if in.TimeoutSec != nil {
		timeoutSec = *in.TimeoutSec
	}
	if timeoutSec < 1 {
		return Result{OK: false, Output: "timeout_sec must be >= 1"}, nil
	}
	if timeoutSec > runCommandMaxTimeoutSec {
		timeoutSec = runCommandMaxTimeoutSec
	}

	// Acquire a per-workspace mutex so concurrent sessions do not run
	// conflicting shell commands (e.g. git commit, go test) simultaneously
	// against the same workspace root.
	release := r.Workspace.LockWorkspace()
	defer release()

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := process.CommandContext(cmdCtx, "sh", "-c", command)
	cmd.Dir = root
	env := process.EnvForSession(process.SessionIDFrom(ctx))
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	text := toolkit.BoundToolOutput(process.RedactSecretValues(string(out), env))

	var exit *int
	if err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			msg := text
			if strings.TrimSpace(msg) != "" {
				msg += "\n"
			}
			msg += fmt.Sprintf("command timed out after %ds", timeoutSec)
			return Result{OK: false, Output: msg}, nil
		}
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
	{re: regexp.MustCompile(`(^|[^[:alnum:]_./-])sudo\s+rm([^[:alnum:]_./-]|$)`), msg: "command rejected by safety guardrail (matched sudo rm)"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_./-])rm\s+-rf\s+/([^[:alnum:]_./-]|$)`), msg: "command rejected by safety guardrail (matched rm -rf /)"},
}

var devRedirectPattern = regexp.MustCompile(`(?:^|[^[:alnum:]_./-])\d*>\s*(/dev/[[:alnum:]_.-]+)`)

// rejectRuntimeAliasInShell stops the shell from creating a literal @runtime
// directory in the workspace. File tools resolve that alias; sh does not.
func rejectRuntimeAliasInShell(cmd string) error {
	if strings.Contains(cmd, "@runtime") {
		return fmt.Errorf("command rejected: @runtime is a file-tool alias and is not expanded by the shell. " +
			"Do not mkdir or redirect to @runtime; that creates a literal folder in the workspace. " +
			"To read large command output, run the command with no redirect and read_file the @runtime/tool-output path from the tool result. " +
			"To pass a file to an external program, use $HOME/.cometmind/agent-tmp, $HOME/.cometmind/tool-output, or $HOME/.cometmind/wiki")
	}
	return nil
}

func denylistCheck(cmd string) error {
	c := strings.ToLower(cmd)
	for _, m := range devRedirectPattern.FindAllStringSubmatch(c, -1) {
		if len(m) > 1 && m[1] != "/dev/null" {
			return fmt.Errorf("command rejected by safety guardrail (matched > /dev/)")
		}
	}
	for _, pattern := range deniedCommandPatterns {
		if pattern.re.MatchString(c) {
			return fmt.Errorf("%s", pattern.msg)
		}
	}
	return nil
}
