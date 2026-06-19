package acp

import (
	"context"
	"io"
	"os/exec"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// Config controls how CometMind spawns an external ACP coding agent.
type Config struct {
	Command     string
	Args        []string
	Timeout     time.Duration
	Interactive bool
}

// DefaultConfig returns defaults for OpenCode in ACP mode.
func DefaultConfig() Config {
	return Config{
		Command: "opencode",
		Args:    []string{"acp"},
		Timeout: 30 * time.Minute,
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

// AgentRunner connects to an ACP agent subprocess and runs one prompt turn.
type AgentRunner struct {
	Config Config
	// ProcessStarter spawns the agent; defaults to exec.Command when nil.
	ProcessStarter func(ctx context.Context, cfg Config) (io.WriteCloser, io.ReadCloser, io.Closer, error)
}

// Run executes a single delegated task against an ACP agent.
func (r *AgentRunner) Run(ctx context.Context, req TaskRequest) (TaskResult, error) {
	cfg := r.Config
	if cfg.Command == "" {
		cfg = DefaultConfig()
	}
	mgr := NewSessionManager(cfg)
	mgr.ProcessStarter = r.ProcessStarter
	return mgr.Run(ctx, RunOptions{
		WorkspaceRoot: req.WorkspaceRoot,
		Task:          req.Task,
		Context:       req.Context,
		VerifyCommand: req.VerifyCommand,
		Interactive:   cfg.Interactive,
		OnProgress:    req.OnProgress,
	})
}

// Cancel sends session/cancel when a connection is active.
func Cancel(conn *acpsdk.ClientSideConnection, sessionID acpsdk.SessionId) error {
	if conn == nil {
		return nil
	}
	return conn.Cancel(context.Background(), acpsdk.CancelNotification{SessionId: sessionID})
}

func defaultProcessStarter(ctx context.Context, cfg Config) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Dir = "."
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	return stdin, stdout, &cmdWaitCloser{cmd: cmd}, nil
}

type cmdWaitCloser struct {
	cmd *exec.Cmd
	once sync.Once
}

func (c *cmdWaitCloser) Close() error {
	var err error
	c.once.Do(func() {
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		err = c.cmd.Wait()
	})
	return err
}

func runVerifyCommand(ctx context.Context, workspaceRoot, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command) //nolint:gosec // delegated verify step
	cmd.Dir = workspaceRoot
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		return text, err
	}
	return text, nil
}
