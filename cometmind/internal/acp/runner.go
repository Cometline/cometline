package acp

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/process"
)

// Harness identifies the external coding agent CometMind delegates to.
type Harness string

const (
	HarnessOpenCode Harness = "opencode"
	HarnessClaude   Harness = "claude"
	HarnessCodex    Harness = "codex"
)

// ParseHarness normalizes a user-facing harness identifier. Unknown values
// fall back to OpenCode so legacy settings remain safe and usable.
func ParseHarness(value string) Harness {
	switch Harness(strings.ToLower(strings.TrimSpace(value))) {
	case HarnessClaude:
		return HarnessClaude
	case HarnessCodex:
		return HarnessCodex
	default:
		return HarnessOpenCode
	}
}

// Config controls how CometMind spawns an external coding agent.
type Config struct {
	// Enabled gates whether delegate_coding_task is registered. Default false
	// (native tools are preferred).
	Enabled bool
	Harness Harness
	// Timeout limits one harness run. Zero means no deadline.
	Timeout time.Duration
}

// ProgressUpdate is a normalized child-agent progress chunk for the parent
// session.
type ProgressUpdate struct {
	Kind    string
	Content string
	Title   string
	Status  string
}

// DefaultConfig returns the default OpenCode CLI profile.
func DefaultConfig() Config {
	return DefaultHarnessConfig(HarnessOpenCode)
}

// DefaultHarnessConfig returns the fixed runtime profile for a harness.
// Command names and arguments are intentionally kept internal so users can
// select a harness without changing how CometMind invokes it.
func DefaultHarnessConfig(harness Harness) Config {
	harness = ParseHarness(string(harness))
	return Config{Harness: harness}
}

func (c Config) normalized() Config {
	harness := ParseHarness(string(c.Harness))
	c.Harness = harness
	return c
}

// CommandAvailable reports whether the selected fixed CLI harness can be
// resolved in the same environment used to launch delegated tasks.
func (c Config) CommandAvailable() bool {
	return c.commandAvailableWithResolver(resolveAgentCommand)
}

func (c Config) commandAvailableWithResolver(resolve func(string) (string, error)) bool {
	command, _ := c.normalized().commandArgs()
	_, err := resolve(command)
	return err == nil
}

// Label returns the stable display label used in progress events and tool
// results.
func (c Config) Label() string {
	switch ParseHarness(string(c.Harness)) {
	case HarnessClaude:
		return "Claude Code"
	case HarnessCodex:
		return "Codex"
	default:
		return "OpenCode"
	}
}

// TaskResult summarizes a delegated coding turn.
type TaskResult struct {
	Status       string
	Summary      string
	VerifyOutput string
	AgentName    string
}

type cmdWaitCloser struct {
	cmd  *exec.Cmd
	once sync.Once
}

func (c Config) commandArgs() (string, []string) {
	switch ParseHarness(string(c.Harness)) {
	case HarnessClaude:
		return "claude", []string{"-p", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"}
	case HarnessCodex:
		return "codex", []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox"}
	default:
		return "opencode", []string{"run", "--format", "json", "--auto"}
	}
}

func (c *cmdWaitCloser) Close() error {
	var err error
	c.once.Do(func() {
		if c.cmd.Process != nil && c.cmd.ProcessState == nil {
			_ = c.cmd.Process.Kill()
		}
		err = c.cmd.Wait()
	})
	return err
}

func runVerifyCommand(ctx context.Context, workspaceRoot, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // delegated verify step
	cmd.Dir = workspaceRoot
	env := process.EnvForSession(process.SessionIDFrom(ctx))
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	text := process.RedactSecretValues(string(out), env)
	if err != nil {
		return text, err
	}
	return text, nil
}
