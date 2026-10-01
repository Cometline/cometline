// Package inboxworker runs background internalization of user inbox replies.
package inboxworker

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/agent"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
	"github.com/Cometline/cometline/cometmind/internal/wakeup"
)

// RunGuard registers a session as currently running an agent turn.
type RunGuard interface {
	Start(parent context.Context, sessionID string) (context.Context, func(), error)
}

// RunnerFactory builds an agent.Runner for an inbox process session.
type RunnerFactory func(sess session.Session, workspacePath string, registry *tools.Registry, maxSteps int) (*agent.Runner, error)

// RegistryOptionsFunc supplies the parent-session tool dependencies for one inbox run.
type RegistryOptionsFunc func(sess session.Session, workspacePath string) tools.RegistryOptions

// Worker periodically internalizes user replies on archived inbox messages.
type Worker struct {
	Inbox           *inbox.Service
	Sessions        *session.Service
	Jobs            *jobs.Service
	Memory          *memory.Service
	Events          *event.Hub
	NewRunner       RunnerFactory
	RegistryOptions RegistryOptionsFunc
	Guard           RunGuard

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
	wakeup.Drain(configChanged)
	for {
		cfg := w.configSnapshot()
		interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 10 * time.Minute
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-configChanged:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			continue
		case <-timer.C:
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
	wakeup.Signal(configChanged)
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

	workspaceID, workspacePath, err := w.resolveWorkspace(ctx, claimed)
	if err != nil {
		_ = w.finishWithError(ctx, claimed, err.Error())
		return
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

	registry := tools.NewInboxProcessRegistry(workspacePath, w.registryOptions(sess, workspacePath))
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
	w.finishRun(ctx, claimed, sess.ID, runErr)
}

// resolveWorkspace returns the message's workspace, or the first registered
// workspace when the message has none. Errors carry the user-facing skip reason.
func (w *Worker) resolveWorkspace(ctx context.Context, msg inbox.Message) (id, path string, err error) {
	id = strings.TrimSpace(msg.WorkspaceID)
	if id == "" {
		workspaces, listErr := w.Sessions.ListWorkspaces(ctx)
		if listErr != nil || len(workspaces) == 0 {
			return "", "", fmt.Errorf("no workspace available for inbox processing")
		}
		return workspaces[0].ID, workspaces[0].Path, nil
	}
	path, pathErr := w.Sessions.WorkspacePath(ctx, id)
	if pathErr != nil {
		return "", "", fmt.Errorf("workspace lookup failed: %w", pathErr)
	}
	return id, path, nil
}

// registryOptions builds the inbox run's tool dependencies, filling any gaps
// left by the RegistryOptions hook with the worker's own services.
func (w *Worker) registryOptions(sess session.Session, workspacePath string) tools.RegistryOptions {
	if w.RegistryOptions == nil {
		return tools.RegistryOptions{
			Jobs:         w.Jobs,
			Memory:       w.Memory,
			MemoryEvents: w.Events,
			SessionID:    sess.ID,
		}
	}
	opt := w.RegistryOptions(sess, workspacePath)
	if opt.Jobs == nil {
		opt.Jobs = w.Jobs
	}
	if opt.Memory == nil {
		opt.Memory = w.Memory
	}
	if opt.MemoryEvents == nil {
		opt.MemoryEvents = w.Events
	}
	if strings.TrimSpace(opt.SessionID) == "" {
		opt.SessionID = sess.ID
	}
	return opt
}

// finishRun marks the message processed after success or its final failed
// attempt; earlier failed attempts are only logged and keep their session.
func (w *Worker) finishRun(ctx context.Context, msg inbox.Message, sessionID string, runErr error) {
	if runErr != nil {
		if msg.ProcessAttempts >= inbox.MaxProcessAttempts {
			_, _ = w.Inbox.MarkProcessed(ctx, msg.ID, runErr.Error())
			w.discardSession(ctx, sessionID)
			return
		}
		log.Printf("inbox: process %s failed (attempt %d): %v", msg.ID, msg.ProcessAttempts, runErr)
		return
	}
	_, _ = w.Inbox.MarkProcessed(ctx, msg.ID, "")
	w.discardSession(ctx, sessionID)
}

func (w *Worker) discardSession(ctx context.Context, sessionID string) {
	if w == nil || w.Sessions == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	if err := w.Sessions.DiscardEphemeralSession(ctx, sessionID); err != nil {
		logging.L().Warn("inbox.session_discard_failed", "session_id", sessionID, "error", err)
	}
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
	b.WriteString("Use the available tools when the reply asks for real work, a lookup, or a durable artifact. This session cannot spawn subagents.\n")
	b.WriteString("Save durable memory when the reply states a lasting preference or fact. If it is only an acknowledgement, do not invent work and call no tools.\n")
	b.WriteString("When the reply asks to remember a reusable workflow as a skill, draft it with write_skill_draft. This session cannot write or promote live skills; drafts stay pending human review.\n")
	b.WriteString("If write_skill_draft is blocked as a near-duplicate, update the same-name draft with overwrite=true or leave the overlap for the user. Do not force a new draft without an explicit yes.\n\n")
	fmt.Fprintf(&b, "Inbox title: %s\n", msg.Title)
	fmt.Fprintf(&b, "Inbox body:\n%s\n\n", msg.Body)
	fmt.Fprintf(&b, "User reply:\n%s\n", msg.UserReply)
	if msg.JobID != "" {
		fmt.Fprintf(&b, "\nRelated job_id: %s (use get_job if needed)\n", msg.JobID)
	}
	return b.String()
}
