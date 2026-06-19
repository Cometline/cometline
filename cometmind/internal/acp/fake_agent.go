package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// FakeAgent is a minimal ACP agent for tests.
type FakeAgent struct {
	conn         *acpsdk.AgentSideConnection
	sessions     map[string]struct{}
	promptCounts map[acpsdk.SessionId]int
	Interactive  bool
	mu           sync.Mutex
}

var _ acpsdk.Agent = (*FakeAgent)(nil)

func NewFakeAgent() *FakeAgent {
	return &FakeAgent{
		sessions:     make(map[string]struct{}),
		promptCounts: make(map[acpsdk.SessionId]int),
	}
}

func (a *FakeAgent) SetAgentConnection(conn *acpsdk.AgentSideConnection) { a.conn = conn }

func (a *FakeAgent) Initialize(ctx context.Context, params acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentInfo:       &acpsdk.Implementation{Name: "fake-opencode"},
	}, nil
}

func (a *FakeAgent) Authenticate(ctx context.Context, params acpsdk.AuthenticateRequest) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *FakeAgent) Logout(ctx context.Context, params acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodLogout)
}

func (a *FakeAgent) NewSession(ctx context.Context, params acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	id := randomSessionID()
	a.mu.Lock()
	a.sessions[id] = struct{}{}
	a.mu.Unlock()
	return acpsdk.NewSessionResponse{SessionId: acpsdk.SessionId(id)}, nil
}

func (a *FakeAgent) Prompt(ctx context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	a.mu.Lock()
	count := a.promptCounts[params.SessionId]
	a.promptCounts[params.SessionId] = count + 1
	interactive := a.Interactive
	a.mu.Unlock()

	text := "done"
	for _, block := range params.Prompt {
		if block.Text != nil {
			text = "completed: " + block.Text.Text
			break
		}
	}

	if interactive && count == 0 {
		text = "Which branch should I use?"
		_ = a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.SessionUpdate{
				AgentMessageChunk: &acpsdk.SessionUpdateAgentMessageChunk{
					Content: acpsdk.TextBlock(text),
				},
			},
		})
		return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
	}

	_ = a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
		SessionId: params.SessionId,
		Update: acpsdk.SessionUpdate{
			AgentMessageChunk: &acpsdk.SessionUpdateAgentMessageChunk{
				Content: acpsdk.TextBlock(text),
			},
		},
	})
	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func (a *FakeAgent) Cancel(ctx context.Context, params acpsdk.CancelNotification) error {
	return nil
}

func (a *FakeAgent) CloseSession(ctx context.Context, params acpsdk.CloseSessionRequest) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionClose)
}

func (a *FakeAgent) ListSessions(ctx context.Context, params acpsdk.ListSessionsRequest) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionList)
}

func (a *FakeAgent) ResumeSession(ctx context.Context, params acpsdk.ResumeSessionRequest) (acpsdk.ResumeSessionResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.sessions[string(params.SessionId)]; !ok {
		return acpsdk.ResumeSessionResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionResume)
	}
	return acpsdk.ResumeSessionResponse{}, nil
}

func (a *FakeAgent) SetSessionConfigOption(ctx context.Context, params acpsdk.SetSessionConfigOptionRequest) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionSetConfigOption)
}

func (a *FakeAgent) SetSessionMode(ctx context.Context, params acpsdk.SetSessionModeRequest) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

// ServeFakeAgent runs a fake agent over stdio until the peer disconnects.
func ServeFakeAgent(agent *FakeAgent, stdin io.Reader, stdout io.Writer) error {
	conn := acpsdk.NewAgentSideConnection(agent, stdout, stdin)
	agent.SetAgentConnection(conn)
	<-conn.Done()
	return nil
}

func randomSessionID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// StartFakeAgentPipes returns pipes connected to a fake agent subprocess in-process.
func StartFakeAgentPipes(ctx context.Context) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
	return startFakeAgentPipes(NewFakeAgent())
}

// StartInteractiveFakeAgentPipes returns pipes for a multi-turn fake agent.
func StartInteractiveFakeAgentPipes(ctx context.Context) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
	agent := NewFakeAgent()
	agent.Interactive = true
	return startFakeAgentPipes(agent)
}

func startFakeAgentPipes(agent *FakeAgent) (io.WriteCloser, io.ReadCloser, io.Closer, error) {
	clientReader, agentWriter := io.Pipe()
	agentReader, clientWriter := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = ServeFakeAgent(agent, agentReader, agentWriter)
		_ = agentWriter.Close()
		_ = agentReader.Close()
	}()

	return clientWriter, clientReader, &pipeCloser{done: done, w: clientWriter, r: clientReader}, nil
}

type pipeCloser struct {
	done chan struct{}
	w    io.Closer
	r    io.Closer
}

func (p *pipeCloser) Close() error {
	_ = p.w.Close()
	_ = p.r.Close()
	<-p.done
	return nil
}
