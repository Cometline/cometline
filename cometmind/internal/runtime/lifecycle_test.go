package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSupervisorStopCancelsAndWaitsForWorkers(t *testing.T) {
	s := newSupervisor()
	var exited atomic.Int32
	const workers = 5
	for range workers {
		if !s.Go(context.Background(), func(ctx context.Context) {
			<-ctx.Done()
			time.Sleep(10 * time.Millisecond)
			exited.Add(1)
		}) {
			t.Fatal("Go() = false before Stop")
		}
	}
	s.Stop()
	if got := exited.Load(); got != workers {
		t.Fatalf("exited workers after Stop = %d, want %d", got, workers)
	}
	s.Stop()
	if s.Go(context.Background(), func(context.Context) { t.Error("worker ran after Stop") }) {
		t.Fatal("Go() = true after Stop")
	}
}

func TestSupervisorWorkerStopsWithParentContext(t *testing.T) {
	s := newSupervisor()
	defer s.Stop()
	parent, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.Go(parent, func(ctx context.Context) {
		<-ctx.Done()
		close(done)
	})
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not observe parent cancellation")
	}
}

func TestNilSupervisorRunsUnsupervised(t *testing.T) {
	var s *supervisor
	done := make(chan struct{})
	if !s.Go(context.Background(), func(context.Context) { close(done) }) {
		t.Fatal("nil supervisor Go() = false")
	}
	<-done
	s.Stop()
}

func TestRuntimeCloseStopsBackgroundWorkersPromptly(t *testing.T) {
	ctx := context.Background()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	rt, err := New(ctx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rt.StartScheduler(ctx)
	rt.StartJobsMaintenance(ctx)
	rt.StartRetentionMaintenance(ctx)
	rt.StartBackupMaintenance(ctx)
	rt.StartAutonomousJobWorker(ctx, nil, nil, nil)
	rt.StartInboxWorker(ctx, nil)
	probeExited := make(chan struct{})
	rt.workers.Go(ctx, func(ctx context.Context) {
		<-ctx.Done()
		close(probeExited)
	})

	closed := make(chan error, 1)
	go func() { closed <- rt.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close() did not return promptly with workers running")
	}
	select {
	case <-probeExited:
	default:
		t.Fatal("Close() returned before supervised workers exited")
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	rt.StartJobsMaintenance(ctx)
}

func TestRuntimeCloseWithoutWorkers(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rt, err := New(context.Background())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	var zero Runtime
	if err := zero.Close(); err != nil {
		t.Fatalf("zero Runtime Close() error = %v", err)
	}
}
