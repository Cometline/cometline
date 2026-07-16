// Package autonomy implements a background worker that claims ready jobs
// from the job queue and runs them to completion without a human opening a
// chat session first ("autonomous job pickup").
package autonomy

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
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
)

// jobHeartbeatInterval mirrors internal/gateway/job_heartbeat.go's interval.
// Duplicated locally because that helper is package-private to gateway.
const jobHeartbeatInterval = 5 * time.Minute

// RunGuard registers a session as "currently running an agent turn" so the
// rest of the system (job reconciliation, the HTTP API's own run tracking)
// knows not to treat it as orphaned or let a second turn start concurrently.
//
// *server.RunManager already implements this interface structurally; no
// changes to that type are required.
type RunGuard interface {
	// Start registers sessionID as running, returning a cancelable context
	// derived from parent and a finish func to call when the run ends. It
	// returns an error if sessionID is already registered as running.
	Start(parent context.Context, sessionID string) (context.Context, func(), error)
	// Running reports whether sessionID is currently registered as running.
	Running(sessionID string) bool
}

// RunnerFactory builds an *agent.Runner for a session bound to a workspace
// path. This matches (*runtime.Runtime).RunnerFor's signature.
type RunnerFactory func(sess session.Session, workspacePath string) (*agent.Runner, error)

// Worker polls the job queue and autonomously claims + executes ready jobs.
type Worker struct {
	Jobs      *jobs.Service
	Sessions  *session.Service
	Memory    *memory.Service // optional; nil disables the task_outcome memory write
	NewRunner RunnerFactory
	Guard     RunGuard

	mu     sync.RWMutex
	Config config.AutonomousJobsConfig

	// DefaultModelID/DefaultProviderID are used for sessions the worker
	// creates itself (mirrors the codebase convention of using the global
	// default model/provider for programmatically-created sessions).
	DefaultModelID    string
	DefaultProviderID string

	sem chan struct{}
}

// Run starts the poll loop and blocks until ctx is canceled.
// Enabled/interval are re-read each cycle so Reload can update autonomy live.
func (w *Worker) Run(ctx context.Context) {
	if w == nil {
		return
	}
	for {
		cfg := w.configSnapshot()
		if !cfg.Enabled {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}
		w.ensureSemaphore()
		interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 30 * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			w.pollOnce(ctx)
		}
	}
}

// UpdateConfig replaces autonomy settings used by the next poll cycle.
func (w *Worker) UpdateConfig(cfg config.AutonomousJobsConfig, defaultModelID, defaultProviderID string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Config = cfg
	if strings.TrimSpace(defaultModelID) != "" {
		w.DefaultModelID = defaultModelID
	}
	if strings.TrimSpace(defaultProviderID) != "" {
		w.DefaultProviderID = defaultProviderID
	}
	// Force semaphore rebuild if concurrency changed.
	max := cfg.MaxConcurrent
	if max <= 0 {
		max = 1
	}
	if w.sem == nil || cap(w.sem) != max {
		w.sem = make(chan struct{}, max)
	}
}

func (w *Worker) configSnapshot() config.AutonomousJobsConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Config
}

// pollOnce lists ready jobs and attempts to claim+run as many as the
// concurrency cap allows without blocking the poll loop itself.
func (w *Worker) pollOnce(ctx context.Context) {
	w.ensureSemaphore()
	ready, err := w.Jobs.ListReady(ctx)
	if err != nil {
		log.Printf("autonomy: list ready jobs: %v", err)
		return
	}
	for _, job := range ready {
		job := job
		select {
		case w.sem <- struct{}{}:
		default:
			// At capacity; stop attempting more claims this tick.
			return
		}
		go func() {
			defer func() { <-w.sem }()
			w.runJob(ctx, job)
		}()
	}
}

func (w *Worker) ensureSemaphore() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.sem != nil {
		return
	}
	max := w.Config.MaxConcurrent
	if max <= 0 {
		max = 1
	}
	w.sem = make(chan struct{}, max)
}

// runJob claims, executes, and finalizes a single job end to end.
func (w *Worker) runJob(ctx context.Context, job jobs.Job) {
	ws, err := w.Sessions.EnsureWorkspace(ctx, job.WorkspacePath)
	if err != nil {
		log.Printf("autonomy: ensure workspace for job %s: %v", job.ID, err)
		return
	}

	sess, err := w.Sessions.NewAutonomySession(ctx, ws.ID, w.DefaultModelID, w.DefaultProviderID)
	if err != nil {
		log.Printf("autonomy: create session for job %s: %v", job.ID, err)
		return
	}

	runCtx, finish, err := w.Guard.Start(ctx, sess.ID)
	if err != nil {
		// Someone else (a human, or a concurrent autonomy tick) is already
		// running this session; back off, the job remains unclaimed.
		log.Printf("autonomy: run guard rejected session %s for job %s: %v", sess.ID, job.ID, err)
		return
	}
	defer finish()

	claimed, err := w.Jobs.Claim(runCtx, job.ID, sess.ID)
	if err != nil {
		// Lost the race to claim (another worker/session claimed first);
		// this is expected and not an error worth logging loudly.
		return
	}

	prompt := jobs.ExecutionPrompt(claimed)
	if _, err := w.Sessions.AppendUserMessage(runCtx, sess.ID, prompt); err != nil {
		log.Printf("autonomy: seed prompt for job %s: %v", claimed.ID, err)
		_, _ = w.Jobs.ReleaseWithClass(runCtx, claimed.ID, sess.ID, "worker: failed to seed execution prompt", jobs.FailureWorkerError)
		return
	}

	runner, err := w.NewRunner(sess, ws.Path)
	if err != nil {
		log.Printf("autonomy: build runner for job %s: %v", claimed.ID, err)
		_, _ = w.Jobs.ReleaseWithClass(runCtx, claimed.ID, sess.ID, "worker: failed to build runner", jobs.FailureWorkerError)
		return
	}

	stopHeartbeat := startJobHeartbeatDuringTurn(runCtx, w.Jobs, sess.ID)
	defer stopHeartbeat()

	runErr := agent.RunHostedTurn(runCtx, runner, session.AgentTurnFromSession(sess), func(_ event.Event) {
		// Autonomous runs have no SSE subscriber; drain and discard.
		// (Phase 1 non-goal: no SSE forwarding for autonomous turns.)
	})

	w.finalizeJob(ctx, claimed, sess, runErr)
}

// finalizeJob is the safety-net path: the agent is expected to call
// complete_job/release_job itself during the run (per the execution
// prompt's instructions). This only acts if the job is still assigned and
// ongoing when the run ends (e.g. hit MaxSteps, crashed, or was canceled).
func (w *Worker) finalizeJob(ctx context.Context, job jobs.Job, sess session.Session, runErr error) {
	current, ok, err := w.Jobs.JobForSession(ctx, sess.ID)
	if err != nil {
		log.Printf("autonomy: lookup job for session %s after run: %v", sess.ID, err)
		return
	}
	if ok && current.ID == job.ID && current.Status == jobs.StatusOngoing {
		// Deterministic safety net: after the runner's hard completion gate is
		// exhausted (or the turn crashed), still-ongoing means the model never
		// issued a terminal job tool. Progress is preserved for resume.
		reason := "worker: run ended without terminal job tool (complete_job/release_job)"
		if runErr != nil {
			reason = fmt.Sprintf("worker: run ended with error: %v", runErr)
		}
		if _, err := w.Jobs.ReleaseWithClass(ctx, job.ID, sess.ID, reason, jobs.FailureWorkerError); err != nil {
			log.Printf("autonomy: release job %s after run: %v", job.ID, err)
		}
	}

	w.recordOutcome(ctx, job, runErr)
}

// recordOutcome writes a task_outcome memory record summarizing the
// attempt. Write-path only for Phase 1 (no retrieval/prompt injection yet).
func (w *Worker) recordOutcome(ctx context.Context, job jobs.Job, runErr error) {
	if w.Memory == nil {
		return
	}
	status := "completed or released by agent"
	if runErr != nil {
		status = fmt.Sprintf("ended with error: %v", runErr)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Autonomous job attempt for: %s\n", job.Description)
	fmt.Fprintf(&b, "Outcome: %s\n", status)
	if progress := strings.TrimSpace(job.Progress); progress != "" {
		fmt.Fprintf(&b, "Progress notes: %s\n", progress)
	}
	if _, err := w.Memory.CreateManual(ctx, b.String(), "task_outcome", false, 1.0); err != nil {
		log.Printf("autonomy: record task_outcome memory for job %s: %v", job.ID, err)
	}
}

// heartbeatJobOnce and startJobHeartbeatDuringTurn duplicate
// internal/gateway/job_heartbeat.go's unexported helpers so the lease stays
// alive during a long autonomous run.
func heartbeatJobOnce(ctx context.Context, svc *jobs.Service, sessionID string) {
	if svc == nil || sessionID == "" {
		return
	}
	job, ok, err := svc.JobForSession(ctx, sessionID)
	if err != nil || !ok {
		return
	}
	_ = svc.Heartbeat(ctx, job.ID, sessionID)
}

func startJobHeartbeatDuringTurn(ctx context.Context, svc *jobs.Service, sessionID string) func() {
	heartbeatJobOnce(ctx, svc, sessionID)
	if svc == nil || sessionID == "" {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(jobHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				heartbeatJobOnce(ctx, svc, sessionID)
			}
		}
	}()
	return func() { close(stop) }
}
