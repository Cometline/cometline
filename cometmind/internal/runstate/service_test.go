package runstate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/db"
	_ "modernc.org/sqlite"
)

func openRunStateDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT OR IGNORE INTO workspaces (id, name, path) VALUES ('workspace-1', 'test', '/test')`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT OR IGNORE INTO sessions (id, workspace_id, model_id, provider_id) VALUES ('session-1', 'workspace-1', 'model', 'provider')`); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestServiceExcludesAnotherProcessAndReleasesByRunID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	serve := New(openRunStateDB(t, path))
	gateway := New(openRunStateDB(t, path))

	lease, err := serve.Acquire(context.Background(), "session-1", OwnerHTTP)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Acquire(context.Background(), "session-1", OwnerGateway); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Acquire() error = %v, want ErrAlreadyRunning", err)
	}

	lease.Finish()
	replacement, err := gateway.Acquire(context.Background(), "session-1", OwnerGateway)
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	replacement.Finish()
}

func TestServiceAbortCrossesProcessBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	gateway := New(openRunStateDB(t, path))
	serve := New(openRunStateDB(t, path))

	lease, err := gateway.Acquire(context.Background(), "session-1", OwnerGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Finish()

	requested, err := serve.RequestAbort(context.Background(), "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if !requested {
		t.Fatal("RequestAbort() = false, want true")
	}

	select {
	case <-lease.Context().Done():
	case <-time.After(3 * time.Second):
		t.Fatal("remote abort did not cancel the owning lease")
	}
	select {
	case <-lease.done:
		t.Fatal("abort stopped the lease heartbeat before cleanup finished")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestServiceKeepsLeaseAliveAfterParentCancellationUntilFinish(t *testing.T) {
	service := New(openRunStateDB(t, filepath.Join(t.TempDir(), "shared.db")))
	parent, cancel := context.WithCancel(context.Background())
	lease, err := service.Acquire(parent, "session-1", OwnerGateway)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-lease.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("parent cancellation did not cancel run context")
	}
	select {
	case <-lease.done:
		t.Fatal("parent cancellation stopped lease ownership before Finish")
	case <-time.After(100 * time.Millisecond):
	}
	lease.Finish()
}

func TestServiceReadPathsPruneStaleRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.db")
	conn := openRunStateDB(t, path)
	service := New(conn)
	if _, err := conn.Exec(`INSERT INTO session_runs (session_id, run_id, owner, updated_at) VALUES ('session-1', 'stale-run', 'gateway', 0)`); err != nil {
		t.Fatal(err)
	}

	running, err := service.Running(context.Background(), "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("Running() = true for stale row")
	}
	requested, err := service.RequestAbort(context.Background(), "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if requested {
		t.Fatal("RequestAbort() = true for stale row")
	}
}
