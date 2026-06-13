package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// RunCommand runs a shell command with cwd set to the workspace root.
type RunCommand struct{ Root string }

func (RunCommand) Name() string { return "run_command" }

func (RunCommand) Description() string {
	return "Run a shell command. Working directory is the workspace root. Dangerous patterns are rejected."
}

func (RunCommand) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Command with arguments (shell interpretation)"}},"required":["command"]}`)
}

func (r RunCommand) Execute(ctx context.Context, workspaceRoot string, input json.RawMessage) (Result, error) {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	if err := denylistCheck(in.Command); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if _, err := sandbox.ResolveWorkspacePath(r.Root, "."); err != nil {
		return Result{}, err
	}
	root := filepath.Clean(r.Root)

	cmdCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", in.Command) //nolint:gosec // agent-invoked shell tool by design
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

func denylistCheck(cmd string) error {
	c := strings.TrimSpace(strings.ToLower(cmd))
	patterns := []string{
		"curl ", "wget ", "ssh ", "scp ", "mkfs", "dd if=", "> /dev/", "sudo rm ", "rm -rf /",
	}
	for _, p := range patterns {
		if strings.Contains(c, p) {
			return fmt.Errorf("command rejected by safety deny-list (matched %q)", p)
		}
	}
	return nil
}
