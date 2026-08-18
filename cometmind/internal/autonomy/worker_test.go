package autonomy

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/tools"
	"github.com/cometline/cometmind/internal/usage"
)

type testRunGuard struct {
	mu      sync.Mutex
	running map[string]bool
}

func newTestRunGuard() *testRunGuard {
	return &testRunGuard{running: make(map[string]bool)}
}

func (g *testRunGuard) Start(parent context.Context, sessionID string) (context.Context, func(), error) {
	g.mu.Lock()
	if g.running[sessionID] {
		g.mu.Unlock()
		return nil, nil, fmt.Errorf("session %s already running", sessionID)
	}
	g.running[sessionID] = true
	g.mu.Unlock()

	ctx, cancel := context.WithCancel(parent)
	finish := func() {
		cancel()
		g.mu.Lock()
		delete(g.running, sessionID)
		g.mu.Unlock()
	}
	return ctx, finish, nil
}

func (g *testRunGuard) Running(sessionID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running[sessionID]
}

type staticProvider struct{}

func (p *staticProvider) ID() string { return "static" }

func (p *staticProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, 3)
	ch <- cometsdk.TextDeltaEvent{Text: "done"}
	ch <- cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop}
	ch <- cometsdk.DoneEvent{}
	close(ch)
	return ch, nil
}

type blockingProvider struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	active  int32
	maxSeen int32
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (p *blockingProvider) ID() string { return "blocking" }

func (p *blockingProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	active := atomic.AddInt32(&p.active, 1)
	defer atomic.AddInt32(&p.active, -1)
	for {
		max := atomic.LoadInt32(&p.maxSeen)
		if active <= max || atomic.CompareAndSwapInt32(&p.maxSeen, max, active) {
			break
		}
	}
	p.once.Do(func() { close(p.started) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.release:
	}
	ch := make(chan cometsdk.Event, 2)
	ch <- cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop}
	ch <- cometsdk.DoneEvent{}
	close(ch)
	return ch, nil
}

type workerFixture struct {
	jobs     *jobs.Service
	sessions *session.Service
	root     string
}

func newWorkerFixture(t *testing.T) workerFixture {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.OpenSQLite(ctx, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return workerFixture{
		jobs:     jobs.NewService(db, nil, nil),
		sessions: session.New(db),
		root:     filepath.Join(dir, "workspace"),
	}
}

func newWorker(t *testing.T, fx workerFixture, provider cometsdk.Provider, cfg config.AutonomousJobsConfig) *Worker {
	t.Helper()
	return &Worker{
		Jobs:     fx.jobs,
		Sessions: fx.sessions,
		NewRunner: func(sess session.Session, workspacePath string, maxSteps int) (*agent.Runner, error) {
			return &agent.Runner{
				Provider:  provider,
				Sessions:  fx.sessions,
				Registry:  tools.NewRegistry(t.TempDir()),
				Jobs:      fx.jobs,
				MaxSteps:  maxSteps,
				MaxTokens: 1024,
			}, nil
		},
		Guard:             newTestRunGuard(),
		Config:            cfg,
		DefaultModelID:    "test-model",
		DefaultProviderID: "test-provider",
	}
}

func TestWorkerRunDisabledReturnsWithoutPolling(t *testing.T) {
	fx := newWorkerFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	job, err := fx.jobs.Create(ctx, jobs.CreateInput{Description: "stay queued", WorkspacePath: fx.root})
	if err != nil {
		t.Fatal(err)
	}
	w := newWorker(t, fx, &staticProvider{}, config.AutonomousJobsConfig{Enabled: false})

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(ctx)
	}()
	// Disabled workers idle until cancel (so Reload can enable them live).
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("disabled worker did not exit after cancel")
	}

	got, err := fx.jobs.Get(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != jobs.StatusTodo || got.AssignedSessionID != "" {
		t.Fatalf("job=%+v, want untouched todo job", got)
	}
}

func TestWorkerRunWakesWhenEnabled(t *testing.T) {
	fx := newWorkerFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := fx.jobs.Create(ctx, jobs.CreateInput{Description: "wake up", WorkspacePath: fx.root}); err != nil {
		t.Fatal(err)
	}
	provider := newBlockingProvider()
	w := newWorker(t, fx, provider, config.AutonomousJobsConfig{Enabled: false})

	go w.Run(ctx)
	time.Sleep(20 * time.Millisecond)
	w.UpdateConfig(config.AutonomousJobsConfig{
		Enabled:             true,
		MaxConcurrent:       1,
		PollIntervalSeconds: 1,
		MaxStepsPerRun:      2,
	}, "test-model", "test-provider")

	select {
	case <-provider.started:
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("worker did not wake after being enabled")
	}
	close(provider.release)
}

func TestWorkerRunJobReleasesWhenAgentDoesNotCompleteJob(t *testing.T) {
	fx := newWorkerFixture(t)
	ctx := context.Background()
	job, err := fx.jobs.Create(ctx, jobs.CreateInput{Description: "finish safely", WorkspacePath: fx.root})
	if err != nil {
		t.Fatal(err)
	}
	// MaxSteps must exceed the runner completion-gate budget so a text-only
	// model exhausts the gate and hits the worker finalize path cleanly.
	w := newWorker(t, fx, &staticProvider{}, config.AutonomousJobsConfig{
		Enabled:             true,
		MaxConcurrent:       1,
		PollIntervalSeconds: 1,
		MaxStepsPerRun:      10,
	})
	var runnerMaxSteps int
	newRunner := w.NewRunner
	w.NewRunner = func(sess session.Session, workspacePath string, maxSteps int) (*agent.Runner, error) {
		runnerMaxSteps = maxSteps
		return newRunner(sess, workspacePath, maxSteps)
	}

	w.runJob(ctx, job)
	if runnerMaxSteps != 10 {
		t.Fatalf("runner MaxSteps = %d, want 10", runnerMaxSteps)
	}

	got, err := fx.jobs.Get(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != jobs.StatusTodo || got.AssignedSessionID != "" || got.FailureCount != 1 || got.NextRetryAt == nil {
		t.Fatalf("job=%+v, want failed todo job with retry metadata", got)
	}
	ws, err := fx.sessions.LookupWorkspaceByPath(ctx, fx.root)
	if err != nil {
		t.Fatal(err)
	}
	allSessions, err := fx.sessions.ListSessions(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(allSessions) != 1 {
		t.Fatalf("ListSessions() len = %d want 1", len(allSessions))
	}
	if allSessions[0].Origin != "autonomy" {
		t.Fatalf("worker session origin = %q want autonomy", allSessions[0].Origin)
	}
	events, err := fx.jobs.ListEvents(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sawFailure bool
	for _, ev := range events {
		if ev.Action == jobs.EventFailed && ev.Detail == "worker: run ended without terminal job tool (complete_job/release_job)" {
			sawFailure = true
		}
	}
	if !sawFailure {
		t.Fatalf("failure event not found in %+v", events)
	}
}

func TestWorkerPollOnceHonorsMaxConcurrent(t *testing.T) {
	fx := newWorkerFixture(t)
	ctx := context.Background()
	first, err := fx.jobs.Create(ctx, jobs.CreateInput{Description: "first", WorkspacePath: fx.root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := fx.jobs.Create(ctx, jobs.CreateInput{Description: "second", WorkspacePath: fx.root})
	if err != nil {
		t.Fatal(err)
	}
	provider := newBlockingProvider()
	w := newWorker(t, fx, provider, config.AutonomousJobsConfig{
		Enabled:             true,
		MaxConcurrent:       1,
		PollIntervalSeconds: 1,
		MaxStepsPerRun:      2,
	})

	w.pollOnce(ctx)
	select {
	case <-provider.started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start first job")
	}

	gotFirst, err := fx.jobs.Get(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotSecond, err := fx.jobs.Get(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotFirst.Status != jobs.StatusOngoing || gotFirst.AssignedSessionID == "" {
		t.Fatalf("first job=%+v, want ongoing assigned job", gotFirst)
	}
	if gotSecond.Status != jobs.StatusTodo || gotSecond.AssignedSessionID != "" {
		t.Fatalf("second job=%+v, want unclaimed todo job", gotSecond)
	}
	if max := atomic.LoadInt32(&provider.maxSeen); max != 1 {
		t.Fatalf("max concurrent provider streams=%d, want 1", max)
	}

	close(provider.release)
	deadline := time.After(2 * time.Second)
	for {
		got, err := fx.jobs.Get(ctx, first.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == jobs.StatusTodo && got.AssignedSessionID == "" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("first job was not released after provider unblock: %+v", got)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWorkerSemaphoreReleaseSurvivesConcurrencyReload(t *testing.T) {
	w := &Worker{Config: config.AutonomousJobsConfig{MaxConcurrent: 1}}
	release, ok := w.tryAcquire()
	if !ok {
		t.Fatal("first semaphore acquisition failed")
	}
	w.UpdateConfig(config.AutonomousJobsConfig{MaxConcurrent: 2}, "", "")

	released := make(chan struct{})
	go func() {
		release()
		close(released)
	}()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("semaphore release blocked after concurrency reload")
	}

	first, ok := w.tryAcquire()
	if !ok {
		t.Fatal("first acquisition at new concurrency failed")
	}
	defer first()
	second, ok := w.tryAcquire()
	if !ok {
		t.Fatal("second acquisition at new concurrency failed")
	}
	defer second()
	if _, ok := w.tryAcquire(); ok {
		t.Fatal("acquisition exceeded reloaded concurrency")
	}
}

func TestScopeUsageUsesJobWorkspace(t *testing.T) {
	fx := newWorkerFixture(t)
	ctx := context.Background()
	ws, err := fx.sessions.EnsureWorkspace(ctx, fx.root)
	if err != nil {
		t.Fatal(err)
	}
	w := newWorker(t, fx, &staticProvider{}, config.AutonomousJobsConfig{})
	scoped := w.scopeUsage(ctx, jobs.Job{WorkspacePath: fx.root, AssignedSessionID: "sess-job"})
	got := usage.ScopeFrom(scoped)
	if got.WorkspaceID != ws.ID || got.SessionID != "sess-job" {
		t.Fatalf("scope=%+v, want workspace %s session sess-job", got, ws.ID)
	}
}
