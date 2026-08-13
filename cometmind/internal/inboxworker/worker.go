// Package inboxworker runs background internalization of user inbox replies.
package inboxworker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

// RunGuard registers a session as currently running an agent turn.
type RunGuard interface {
	Start(parent context.Context, sessionID string) (context.Context, func(), error)
}

// RunnerFactory builds an agent.Runner for an inbox process session.
type RunnerFactory func(sess session.Session, workspacePath string, registry *tools.Registry, maxSteps int) (*agent.Runner, error)

// Worker periodically internalizes user replies on archived inbox messages.
type Worker struct {
	Inbox     *inbox.Service
	Sessions  *session.Service
	Jobs      *jobs.Service
	Memory    *memory.Service
	Events    *event.Hub
	NewRunner RunnerFactory
	Guard     RunGuard

	mu                sync.RWMutex
	Config            config.InboxConfig
	DefaultModelID    string
	DefaultProviderID string
	configChanged     chan struct{}
}

// Run starts the poll loop and blocks until ctx is canceled.
func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.Inbox == nil {
		return
	}
	configChanged := w.reloadChannel()
	drain(configChanged)
	for {
		cfg := w.configSnapshot()
		interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 10 * time.Minute
		}
		select {
		case <-ctx.Done():
			return
		case <-configChanged:
			continue
		case <-time.After(interval):
			w.pollOnce(ctx)
		}
	}
}

// UpdateConfig replaces inbox settings used by the next poll cycle.
func (w *Worker) UpdateConfig(cfg config.InboxConfig, defaultModelID, defaultProviderID string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.Config = cfg
	if strings.TrimSpace(defaultModelID) != "" {
		w.DefaultModelID = defaultModelID
	}
	if strings.TrimSpace(defaultProviderID) != "" {
		w.DefaultProviderID = defaultProviderID
	}
	configChanged := w.ensureReloadChannelLocked()
	w.mu.Unlock()
	signal(configChanged)
}

func (w *Worker) reloadChannel() <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ensureReloadChannelLocked()
}

func (w *Worker) ensureReloadChannelLocked() chan struct{} {
	if w.configChanged == nil {
		w.configChanged = make(chan struct{}, 1)
	}
	return w.configChanged
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func drain(ch <-chan struct{}) {
	select {
	case <-ch:
	default:
	}
}

func (w *Worker) configSnapshot() config.InboxConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Config
}

func (w *Worker) pollOnce(ctx context.Context) {
	pending, err := w.Inbox.ListPendingProcess(ctx, 5)
	if err != nil {
		log.Printf("inbox: list pending: %v", err)
		return
	}
	for _, msg := range pending {
		w.processOne(ctx, msg)
	}
}

func (w *Worker) processOne(ctx context.Context, msg inbox.Message) {
	claimed, err := w.Inbox.ClaimForProcess(ctx, msg.ID)
	if err != nil {
		return
	}

	workspaceID := strings.TrimSpace(claimed.WorkspaceID)
	workspacePath := ""
	if workspaceID == "" {
		workspaces, listErr := w.Sessions.ListWorkspaces(ctx)
		if listErr != nil || len(workspaces) == 0 {
			_ = w.finishWithError(ctx, claimed, "no workspace available for inbox processing")
			return
		}
		workspaceID = workspaces[0].ID
		workspacePath = workspaces[0].Path
	} else {
		path, pathErr := w.Sessions.WorkspacePath(ctx, workspaceID)
		if pathErr != nil {
			_ = w.finishWithError(ctx, claimed, fmt.Sprintf("workspace lookup failed: %v", pathErr))
			return
		}
		workspacePath = path
	}

	cfg := w.configSnapshot()
	sess, err := w.Sessions.NewInboxSession(ctx, workspaceID, w.DefaultModelID, w.DefaultProviderID)
	if err != nil {
		_ = w.finishWithError(ctx, claimed, fmt.Sprintf("create inbox session: %v", err))
		return
	}

	runCtx := ctx
	finish := func() {}
	if w.Guard != nil {
		var startErr error
		runCtx, finish, startErr = w.Guard.Start(ctx, sess.ID)
		if startErr != nil {
			_ = w.finishWithError(ctx, claimed, fmt.Sprintf("run guard: %v", startErr))
			return
		}
	}
	defer finish()

	prompt := internalizationPrompt(claimed)
	if _, err := w.Sessions.AppendUserMessage(runCtx, sess.ID, prompt); err != nil {
		_ = w.finishWithError(ctx, claimed, fmt.Sprintf("seed prompt: %v", err))
		return
	}

	registry := tools.NewInboxProcessRegistry(tools.RegistryOptions{
		Jobs:         w.Jobs,
		Memory:       w.Memory,
		MemoryEvents: w.Events,
	})
	maxSteps := cfg.MaxStepsPerRun
	if maxSteps <= 0 {
		maxSteps = 8
	}
	if w.NewRunner == nil {
		_ = w.finishWithError(ctx, claimed, "runner factory unavailable")
		return
	}
	runner, err := w.NewRunner(sess, workspacePath, registry, maxSteps)
	if err != nil {
		_ = w.finishWithError(ctx, claimed, fmt.Sprintf("build runner: %v", err))
		return
	}

	runErr := agent.RunHostedTurn(runCtx, runner, session.AgentTurnFromSession(sess), func(_ event.Event) {})
	if runErr != nil {
		if claimed.ProcessAttempts >= inbox.MaxProcessAttempts {
			_, _ = w.Inbox.MarkProcessed(ctx, claimed.ID, runErr.Error())
			return
		}
		log.Printf("inbox: process %s failed (attempt %d): %v", claimed.ID, claimed.ProcessAttempts, runErr)
		return
	}
	_, _ = w.Inbox.MarkProcessed(ctx, claimed.ID, "")
}

func (w *Worker) finishWithError(ctx context.Context, msg inbox.Message, reason string) error {
	if msg.ProcessAttempts >= inbox.MaxProcessAttempts {
		_, err := w.Inbox.MarkProcessed(ctx, msg.ID, reason)
		return err
	}
	log.Printf("inbox: process %s skipped: %s", msg.ID, reason)
	return nil
}

func internalizationPrompt(msg inbox.Message) string {
	var b strings.Builder
	b.WriteString("You are processing a user reply to an inbox note you previously left.\n")
	b.WriteString("Decide whether to save durable memory from this exchange, or do nothing.\n")
	b.WriteString("Allowed tools only: list_memories, search_memories, create_memory, update_memory, get_job.\n")
	b.WriteString("Do not invent work; if the reply is a simple acknowledgement with no lasting preference or fact, call no tools.\n\n")
	fmt.Fprintf(&b, "Inbox title: %s\n", msg.Title)
	fmt.Fprintf(&b, "Inbox body:\n%s\n\n", msg.Body)
	fmt.Fprintf(&b, "User reply:\n%s\n", msg.UserReply)
	if msg.JobID != "" {
		fmt.Fprintf(&b, "\nRelated job_id: %s (use get_job if needed)\n", msg.JobID)
	}
	return b.String()
}
