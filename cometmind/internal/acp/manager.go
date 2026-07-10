package acp

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

// RunOptions configures one delegated coding-harness run.
type RunOptions struct {
	ChildSessionID string
	WorkspaceRoot  string
	Task           string
	Context        string
	VerifyCommand  string
	OnProgress     func(ProgressUpdate)
}

// SessionManager keeps active CLI processes keyed by child session ID.
type SessionManager struct {
	Config            Config
	CLIProcessStarter CLIProcessStarter

	mu     sync.Mutex
	active map[string]*activeSession
}

type activeSession struct {
	mu     sync.Mutex
	closer io.Closer
	cancel context.CancelFunc
}

// NewSessionManager returns a manager for coding-harness delegations.
func NewSessionManager(cfg Config) *SessionManager {
	return &SessionManager{
		Config: cfg,
		active: make(map[string]*activeSession),
	}
}

// UpdateConfig replaces the harness selection used for future delegations.
func (m *SessionManager) UpdateConfig(cfg Config) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.Config = cfg
	m.mu.Unlock()
}

// Run executes a delegated task through the selected fixed CLI profile.
func (m *SessionManager) Run(ctx context.Context, opts RunOptions) (TaskResult, error) {
	cfg := m.configSnapshot().normalized()
	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	promptText := opts.Task
	if strings.TrimSpace(opts.Context) != "" {
		promptText = opts.Context + "\n\nTask:\n" + opts.Task
	}

	start := m.CLIProcessStarter
	if start == nil {
		start = defaultCLIProcessStarter
	}
	stdout, stderr, closer, err := start(runCtx, cfg, opts.WorkspaceRoot, promptText)
	if err != nil {
		return TaskResult{Status: "failed", Summary: err.Error(), AgentName: cfg.Label()}, err
	}
	defer stdout.Close()
	defer stderr.Close()
	defer closer.Close()

	if opts.ChildSessionID != "" {
		m.register(opts.ChildSessionID, &activeSession{closer: closer, cancel: cancel})
		defer m.unregister(opts.ChildSessionID)
	}

	summary, stderrText, scanErr, waitErr := collectCLIOutput(
		runCtx,
		cfg.Harness,
		stdout,
		stderr,
		closer,
		opts.OnProgress,
	)
	if errors.Is(runCtx.Err(), context.Canceled) || errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return TaskResult{Status: "cancelled", AgentName: cfg.Label()}, context.Canceled
	}
	if scanErr != nil {
		return TaskResult{Status: "failed", Summary: scanErr.Error(), AgentName: cfg.Label()}, scanErr
	}
	if waitErr != nil {
		if summary == "" {
			summary = stderrText
		}
		if summary == "" {
			summary = waitErr.Error()
		}
		return TaskResult{Status: "failed", Summary: summary, AgentName: cfg.Label()}, waitErr
	}

	verifyOut := ""
	if strings.TrimSpace(opts.VerifyCommand) != "" {
		verifyOut, _ = runVerifyCommand(runCtx, opts.WorkspaceRoot, opts.VerifyCommand)
	}
	if summary == "" {
		summary = "delegation finished"
	}
	if verifyOut != "" {
		summary += "\n\nVerify output:\n" + verifyOut
	}

	return TaskResult{
		Status:       "completed",
		Summary:      summary,
		VerifyOutput: verifyOut,
		AgentName:    cfg.Label(),
	}, nil
}

func (m *SessionManager) configSnapshot() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Config
}

// Cancel stops an active coding-harness process.
func (m *SessionManager) Cancel(childSessionID string) error {
	act := m.get(childSessionID)
	if act == nil {
		return nil
	}
	act.mu.Lock()
	defer act.mu.Unlock()
	if act.cancel != nil {
		act.cancel()
	}
	if act.closer != nil {
		_ = act.closer.Close()
	}
	return nil
}

func (m *SessionManager) register(childID string, act *activeSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active[childID] = act
}

func (m *SessionManager) unregister(childID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, childID)
}

func (m *SessionManager) get(childID string) *activeSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active[childID]
}
