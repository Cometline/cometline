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
	Harness Harness
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
	return Config{Harness: harness, Timeout: 30 * time.Minute}
}

func (c Config) normalized() Config {
	harness := ParseHarness(string(c.Harness))
	defaults := DefaultHarnessConfig(harness)
	if c.Timeout <= 0 {
		c.Timeout = defaults.Timeout
	}
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

// TaskRequest is one delegated coding turn.
type TaskRequest struct {
	WorkspaceRoot string
	Task          string
	Context       string
	VerifyCommand string
	OnProgress    func(ProgressUpdate)
}

// TaskResult summarizes a delegated coding turn.
type TaskResult struct {
	Status       string
	Summary      string
	VerifyOutput string
	AgentName    string
}

// AgentRunner runs one prompt turn against a fixed CLI coding harness.
type AgentRunner struct {
	Config Config
	// ProcessStarter starts the harness; defaults to exec.Command when nil.
	ProcessStarter CLIProcessStarter
}

// Run executes a single delegated task against the selected CLI harness.
func (r *AgentRunner) Run(ctx context.Context, req TaskRequest) (TaskResult, error) {
	cfg := r.Config.normalized()
	mgr := NewSessionManager(cfg)
	mgr.CLIProcessStarter = r.ProcessStarter
	return mgr.Run(ctx, RunOptions{
		WorkspaceRoot: req.WorkspaceRoot,
		Task:          req.Task,
		Context:       req.Context,
		VerifyCommand: req.VerifyCommand,
		OnProgress:    req.OnProgress,
	})
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
	cmd.Env = process.Env()
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		return text, err
	}
	return text, nil
}
