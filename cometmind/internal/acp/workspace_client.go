package acp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// ProgressUpdate is a normalized child-agent progress chunk for the parent session.
type ProgressUpdate struct {
	Kind    string
	Content string
	Title   string
	Status  string
}

// WorkspaceClient implements acp.Client with workspace-sandboxed file and terminal access.
type WorkspaceClient struct {
	WorkspaceRoot string
	OnProgress    func(ProgressUpdate)
	Interactive   bool
	PermissionHandler func(ctx context.Context, params acpsdk.RequestPermissionRequest) (acpsdk.PermissionOptionId, error)

	mu        sync.Mutex
	terminals map[string]*terminalSession
	lastAgent string
}

type terminalSession struct {
	cmd    *exec.Cmd
	output strings.Builder
	done   chan struct{}
	exit   int
}

var _ acpsdk.Client = (*WorkspaceClient)(nil)

func (c *WorkspaceClient) SessionUpdate(ctx context.Context, params acpsdk.SessionNotification) error {
	if c.OnProgress == nil {
		return nil
	}
	u := params.Update
	switch {
	case u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil:
		text := u.AgentMessageChunk.Content.Text.Text
		c.mu.Lock()
		c.lastAgent += text
		c.mu.Unlock()
		c.OnProgress(ProgressUpdate{Kind: "message", Content: text})
	case u.AgentThoughtChunk != nil && u.AgentThoughtChunk.Content.Text != nil:
		c.OnProgress(ProgressUpdate{Kind: "thought", Content: u.AgentThoughtChunk.Content.Text.Text})
	case u.ToolCall != nil:
		c.OnProgress(ProgressUpdate{Kind: "tool_call", Title: u.ToolCall.Title, Status: string(u.ToolCall.Status)})
	case u.ToolCallUpdate != nil:
		status := ""
		if u.ToolCallUpdate.Status != nil {
			status = string(*u.ToolCallUpdate.Status)
		}
		c.OnProgress(ProgressUpdate{
			Kind:   "tool_call_update",
			Title:  string(u.ToolCallUpdate.ToolCallId),
			Status: status,
		})
	case u.Plan != nil:
		var b strings.Builder
		for _, entry := range u.Plan.Entries {
			if entry.Content != "" {
				b.WriteString(entry.Content)
				b.WriteByte('\n')
			}
		}
		c.OnProgress(ProgressUpdate{Kind: "plan", Content: strings.TrimSpace(b.String())})
	}
	return nil
}

func (c *WorkspaceClient) RequestPermission(ctx context.Context, params acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	if c.Interactive && c.PermissionHandler != nil {
		optionID, err := c.PermissionHandler(ctx, params)
		if err != nil {
			return acpsdk.RequestPermissionResponse{}, err
		}
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: optionID},
			},
		}, nil
	}
	// Local dogfood mode: auto-approve the first allow-style option.
	for _, opt := range params.Options {
		if opt.Kind == acpsdk.PermissionOptionKindAllowOnce || opt.Kind == acpsdk.PermissionOptionKindAllowAlways {
			return acpsdk.RequestPermissionResponse{
				Outcome: acpsdk.RequestPermissionOutcome{
					Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: opt.OptionId},
				},
			}, nil
		}
	}
	if len(params.Options) > 0 {
		return acpsdk.RequestPermissionResponse{
			Outcome: acpsdk.RequestPermissionOutcome{
				Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId},
			},
		}, nil
	}
	return acpsdk.RequestPermissionResponse{}, errors.New("no permission options")
}

// LastAgentMessage returns accumulated agent message text for the current prompt turn.
func (c *WorkspaceClient) LastAgentMessage() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastAgent
}

// ResetAgentMessage clears accumulated agent text before a new prompt turn.
func (c *WorkspaceClient) ResetAgentMessage() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastAgent = ""
}

func (c *WorkspaceClient) ReadTextFile(ctx context.Context, params acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	abs, err := c.resolvePath(params.Path)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	content := string(b)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = min(max(*params.Line-1, 0), len(lines))
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 && start+*params.Limit < end {
			end = start + *params.Limit
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acpsdk.ReadTextFileResponse{Content: content}, nil
}

func (c *WorkspaceClient) WriteTextFile(ctx context.Context, params acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	abs, err := c.resolvePath(params.Path)
	if err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	if err := os.WriteFile(abs, []byte(params.Content), 0o644); err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	return acpsdk.WriteTextFileResponse{}, nil
}

func (c *WorkspaceClient) CreateTerminal(ctx context.Context, params acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.terminals == nil {
		c.terminals = make(map[string]*terminalSession)
	}
	id := fmt.Sprintf("term-%d", time.Now().UnixNano())
	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command) //nolint:gosec // ACP terminal delegation
	cmd.Dir = c.WorkspaceRoot
	ts := &terminalSession{cmd: cmd, done: make(chan struct{})}
	c.terminals[id] = ts
	go func() {
		out, err := cmd.CombinedOutput()
		ts.output.Write(out)
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				ts.exit = ee.ExitCode()
			} else {
				ts.exit = 1
				ts.output.WriteString(err.Error())
			}
		}
		close(ts.done)
	}()
	return acpsdk.CreateTerminalResponse{TerminalId: id}, nil
}

func (c *WorkspaceClient) TerminalOutput(ctx context.Context, params acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	c.mu.Lock()
	ts, ok := c.terminals[params.TerminalId]
	c.mu.Unlock()
	if !ok {
		return acpsdk.TerminalOutputResponse{}, fmt.Errorf("unknown terminal %q", params.TerminalId)
	}
	select {
	case <-ts.done:
	case <-ctx.Done():
		return acpsdk.TerminalOutputResponse{}, ctx.Err()
	}
	out := ts.output.String()
	return acpsdk.TerminalOutputResponse{Output: out, Truncated: false}, nil
}

func (c *WorkspaceClient) WaitForTerminalExit(ctx context.Context, params acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	c.mu.Lock()
	ts, ok := c.terminals[params.TerminalId]
	c.mu.Unlock()
	if !ok {
		return acpsdk.WaitForTerminalExitResponse{}, fmt.Errorf("unknown terminal %q", params.TerminalId)
	}
	select {
	case <-ts.done:
		code := ts.exit
		return acpsdk.WaitForTerminalExitResponse{ExitCode: &code}, nil
	case <-ctx.Done():
		return acpsdk.WaitForTerminalExitResponse{}, ctx.Err()
	}
}

func (c *WorkspaceClient) ReleaseTerminal(ctx context.Context, params acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	c.mu.Lock()
	delete(c.terminals, params.TerminalId)
	c.mu.Unlock()
	return acpsdk.ReleaseTerminalResponse{}, nil
}

func (c *WorkspaceClient) KillTerminal(ctx context.Context, params acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	c.mu.Lock()
	ts, ok := c.terminals[params.TerminalId]
	c.mu.Unlock()
	if !ok {
		return acpsdk.KillTerminalResponse{}, nil
	}
	if ts.cmd.Process != nil {
		_ = ts.cmd.Process.Kill()
	}
	return acpsdk.KillTerminalResponse{}, nil
}

func (c *WorkspaceClient) resolvePath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return sandbox.ResolveWorkspacePath(c.WorkspaceRoot, p)
	}
	return sandbox.ResolveWorkspacePath(c.WorkspaceRoot, p)
}
