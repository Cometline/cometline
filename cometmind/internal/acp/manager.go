package acp

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// RunOptions configures one delegated run.
type RunOptions struct {
	ChildSessionID string
	WorkspaceRoot  string
	Task           string
	Context        string
	VerifyCommand  string
	OnProgress     func(ProgressUpdate)
	OnACPSessionID func(sessionID string)
}

// SessionManager keeps long-lived ACP connections keyed by child session ID.
type SessionManager struct {
	Config         Config
	ProcessStarter func(ctx context.Context, cfg Config) (io.WriteCloser, io.ReadCloser, io.Closer, error)

	mu     sync.Mutex
	active map[string]*activeSession
}

type activeSession struct {
	mu        sync.Mutex
	conn      *acpsdk.ClientSideConnection
	sessionID acpsdk.SessionId
	closer    io.Closer
	client    *WorkspaceClient
	cancel    context.CancelFunc
}

// NewSessionManager returns a manager for ACP delegations.
func NewSessionManager(cfg Config) *SessionManager {
	return &SessionManager{
		Config: cfg,
		active: make(map[string]*activeSession),
	}
}

// UpdateConfig replaces the config used for future delegations.
func (m *SessionManager) UpdateConfig(cfg Config) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.Config = cfg
	m.mu.Unlock()
}

// Run executes a delegated task.
func (m *SessionManager) Run(ctx context.Context, opts RunOptions) (TaskResult, error) {
	cfg := m.Config
	if cfg.Command == "" {
		cfg = DefaultConfig()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultConfig().Timeout
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	start := m.ProcessStarter
	if start == nil {
		start = defaultProcessStarter
	}
	stdin, stdout, closer, err := start(runCtx, cfg)
	if err != nil {
		return TaskResult{Status: "failed", Summary: err.Error()}, err
	}
	defer closer.Close()

	client := &WorkspaceClient{
		WorkspaceRoot: opts.WorkspaceRoot,
		OnProgress:    opts.OnProgress,
	}

	conn := acpsdk.NewClientSideConnection(client, stdin, stdout)

	initResp, err := conn.Initialize(runCtx, acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{
			Fs: acpsdk.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
	})
	if err != nil {
		return TaskResult{Status: "failed", Summary: err.Error()}, err
	}
	agentName := "acp-agent"
	if initResp.AgentInfo != nil && initResp.AgentInfo.Name != "" {
		agentName = initResp.AgentInfo.Name
	}

	sess, err := conn.NewSession(runCtx, acpsdk.NewSessionRequest{
		Cwd:        opts.WorkspaceRoot,
		McpServers: []acpsdk.McpServer{},
	})
	if err != nil {
		return TaskResult{Status: "failed", Summary: err.Error(), AgentName: agentName}, err
	}
	if opts.OnACPSessionID != nil {
		opts.OnACPSessionID(string(sess.SessionId))
	}

	act := &activeSession{
		conn:      conn,
		sessionID: sess.SessionId,
		closer:    closer,
		client:    client,
		cancel:    cancel,
	}
	if opts.ChildSessionID != "" {
		m.register(opts.ChildSessionID, act)
		defer m.unregister(opts.ChildSessionID)
	}

	promptText := opts.Task
	if strings.TrimSpace(opts.Context) != "" {
		promptText = opts.Context + "\n\nTask:\n" + opts.Task
	}

	var chunks []string
	prev := client.OnProgress
	client.OnProgress = func(u ProgressUpdate) {
		if prev != nil {
			prev(u)
		}
		if u.Content != "" {
			chunks = append(chunks, u.Content)
		}
	}

	promptResp, err := conn.Prompt(runCtx, acpsdk.PromptRequest{
		SessionId: sess.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(promptText)},
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || runCtx.Err() != nil {
			return TaskResult{Status: "cancelled", AgentName: agentName}, context.Canceled
		}
		return TaskResult{Status: "failed", Summary: err.Error(), AgentName: agentName}, err
	}
	if promptResp.StopReason == acpsdk.StopReasonCancelled {
		return TaskResult{Status: "cancelled", AgentName: agentName}, nil
	}

	verifyOut := ""
	if strings.TrimSpace(opts.VerifyCommand) != "" {
		verifyOut, _ = runVerifyCommand(runCtx, opts.WorkspaceRoot, opts.VerifyCommand)
	}

	summary := strings.TrimSpace(strings.Join(chunks, "\n"))
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
		AgentName:    agentName,
	}, nil
}

// Cancel stops an active delegated session.
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
	if act.conn != nil {
		_ = Cancel(act.conn, act.sessionID)
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
