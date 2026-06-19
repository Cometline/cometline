package acp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// RespondInput carries user follow-up for an interactive delegated session.
type RespondInput struct {
	Text               string
	PermissionOptionID string
}

// AwaitingInfo describes why a child session paused for human input.
type AwaitingInfo struct {
	Kind     string // "input" or "permission"
	Question string
	Options  []PermissionOptionInfo
}

// PermissionOptionInfo is a user-selectable permission outcome.
type PermissionOptionInfo struct {
	ID   string
	Kind string
	Name string
}

// AwaitingCallback fires when a delegated session needs user input.
type AwaitingCallback func(AwaitingInfo)

// RunOptions configures one interactive or one-shot delegated run.
type RunOptions struct {
	ChildSessionID string
	WorkspaceRoot  string
	Task           string
	Context        string
	VerifyCommand  string
	Interactive    bool
	OnProgress     func(ProgressUpdate)
	OnAwaiting     AwaitingCallback
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
	respondCh chan respondMsg
	cancel    context.CancelFunc
}

type respondMsg struct {
	input RespondInput
}

// NewSessionManager returns a manager for interactive ACP delegations.
func NewSessionManager(cfg Config) *SessionManager {
	return &SessionManager{
		Config: cfg,
		active: make(map[string]*activeSession),
	}
}

// Run executes a delegated task, optionally looping for interactive follow-ups.
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
		Interactive:   opts.Interactive,
	}
	client.PermissionHandler = func(pctx context.Context, params acpsdk.RequestPermissionRequest) (acpsdk.PermissionOptionId, error) {
		return m.handlePermissionRequest(pctx, opts.ChildSessionID, opts.OnAwaiting, params)
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
		respondCh: make(chan respondMsg, 1),
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

	for {
		client.ResetAgentMessage()
		promptResp, err := conn.Prompt(runCtx, acpsdk.PromptRequest{
			SessionId: sess.SessionId,
			Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(promptText)},
		})
		if err != nil {
			return TaskResult{Status: "failed", Summary: err.Error(), AgentName: agentName}, err
		}
		if promptResp.StopReason == acpsdk.StopReasonCancelled {
			return TaskResult{Status: "cancelled", AgentName: agentName}, nil
		}

		needFollowUp := opts.Interactive &&
			promptResp.StopReason == acpsdk.StopReasonEndTurn &&
			looksLikeQuestion(client.LastAgentMessage())
		if !needFollowUp {
			break
		}

		question := strings.TrimSpace(client.LastAgentMessage())
		if question == "" {
			question = strings.TrimSpace(strings.Join(chunks, "\n"))
		}
		if question == "" {
			question = "Waiting for your input."
		}
		if opts.OnAwaiting != nil {
			opts.OnAwaiting(AwaitingInfo{Kind: "input", Question: question})
		}

		input, err := m.waitRespond(runCtx, act)
		if err != nil {
			if runCtx.Err() != nil {
				return TaskResult{Status: "cancelled", AgentName: agentName}, nil
			}
			return TaskResult{Status: "failed", Summary: err.Error(), AgentName: agentName}, err
		}
		promptText = strings.TrimSpace(input.Text)
		if promptText == "" && input.PermissionOptionID != "" {
			promptText = input.PermissionOptionID
		}
		if promptText == "" {
			return TaskResult{Status: "failed", Summary: "empty follow-up response", AgentName: agentName}, fmt.Errorf("empty follow-up response")
		}
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

func (m *SessionManager) handlePermissionRequest(
	ctx context.Context,
	childID string,
	onAwaiting AwaitingCallback,
	params acpsdk.RequestPermissionRequest,
) (acpsdk.PermissionOptionId, error) {
	question := "Permission required"
	if params.ToolCall.Title != nil && strings.TrimSpace(*params.ToolCall.Title) != "" {
		question = strings.TrimSpace(*params.ToolCall.Title)
	}
	options := make([]PermissionOptionInfo, 0, len(params.Options))
	for _, opt := range params.Options {
		options = append(options, PermissionOptionInfo{
			ID:   string(opt.OptionId),
			Kind: string(opt.Kind),
			Name: opt.Name,
		})
	}
	if onAwaiting != nil {
		onAwaiting(AwaitingInfo{Kind: "permission", Question: question, Options: options})
	}

	act := m.get(childID)
	if act == nil {
		return "", fmt.Errorf("no active session for child %s", childID)
	}
	// Use the caller-supplied context (the ACP SDK's per-request context,
	// derived from runCtx) instead of context.Background(). This ensures
	// the goroutine unblocks and exits when the parent run is cancelled or
	// times out, preventing a goroutine leak when the user never responds.
	input, err := m.waitRespond(ctx, act)
	if err != nil {
		return "", err
	}
	if input.PermissionOptionID != "" {
		return acpsdk.PermissionOptionId(input.PermissionOptionID), nil
	}
	return "", fmt.Errorf("permission option required")
}

func (m *SessionManager) waitRespond(ctx context.Context, act *activeSession) (RespondInput, error) {
	select {
	case msg := <-act.respondCh:
		return msg.input, nil
	case <-ctx.Done():
		return RespondInput{}, ctx.Err()
	}
}

// Respond delivers user input to a waiting child session.
func (m *SessionManager) Respond(childSessionID string, input RespondInput) error {
	act := m.get(childSessionID)
	if act == nil {
		return fmt.Errorf("no active delegated session %s", childSessionID)
	}
	select {
	case act.respondCh <- respondMsg{input: input}:
		return nil
	default:
		return fmt.Errorf("child session %s is not awaiting input", childSessionID)
	}
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

func looksLikeQuestion(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.ContainsAny(text, "?？") {
		return true
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "which ") || strings.Contains(lower, "what ") {
		return true
	}
	for _, cue := range []string{
		"请问", "請問",
		"你想", "你要",
		"是否", "要不要",
		"哪个", "哪個", "哪一",
		"吗", "嗎",
	} {
		if strings.Contains(text, cue) {
			return true
		}
	}
	return false
}
