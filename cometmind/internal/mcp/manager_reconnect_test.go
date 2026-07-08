package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// withFastBackoff overrides the package-level auto-reconnect backoff policy
// for the duration of a test, so tests exercising autoReconnect don't have to
// wait out the real 2s/8s/30s schedule.
func withFastBackoff(t *testing.T, delays []time.Duration) {
	t.Helper()
	original := mcpAutoReconnectBackoff
	mcpAutoReconnectBackoff = delays
	t.Cleanup(func() { mcpAutoReconnectBackoff = original })
}

// newInMemoryConnectedServer wires up a client session against an in-process
// MCP server over an in-memory transport pair, mirroring the pattern used by
// TestManagerInMemoryToolCall. It returns the connected client-side server
// plus a func to close/kill the server side (simulating a transport death).
func newInMemoryConnectedServer(t *testing.T, cfg ServerConfig) (*connectedServer, func()) {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	conn, err := connectServerWithTransport(ctx, cfg, t2)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("connectServerWithTransport: %v", err)
	}
	t.Cleanup(func() { _ = conn.session.Close() })
	return conn, func() { _ = serverSession.Close() }
}

// TestMonitorConnectionDetectsSessionDeathAndSetsError verifies that when the
// underlying transport dies (session.Wait() returns), monitorConnection
// corrects the cached status from Connected to Error and records a
// last-error message, instead of leaving the stale "connected" status in
// place forever.
func TestMonitorConnectionDetectsSessionDeathAndSetsError(t *testing.T) {
	withFastBackoff(t, []time.Duration{time.Millisecond})

	cfg := ServerConfig{ID: "demo", Name: "Demo", Enabled: false, Transport: TransportStdio}
	conn, kill := newInMemoryConnectedServer(t, cfg)

	mgr := NewManager(Config{Enabled: true})
	mgr.mu.Lock()
	mgr.servers["demo"] = &managedServer{cfg: cfg, status: StatusConnected, generation: 1}
	mgr.mu.Unlock()

	kill() // server-side close unblocks conn.session.Wait()

	done := make(chan struct{})
	go func() {
		mgr.monitorConnection("demo", 1, conn)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitorConnection did not return in time")
	}

	mgr.mu.RLock()
	entry := mgr.servers["demo"]
	status, lastError := entry.status, entry.lastError
	mgr.mu.RUnlock()

	if status != StatusError {
		t.Fatalf("status = %q, want %q", status, StatusError)
	}
	if lastError == "" {
		t.Fatal("lastError = \"\", want a non-empty message describing the session closure")
	}
}

// TestMonitorConnectionStaleGenerationIsNoop verifies that a monitorConnection
// goroutine watching a superseded connection (generation bumped by a newer
// connect/reconnect/reload/close) does not clobber the entry's current state
// when its Wait() call eventually unblocks.
func TestMonitorConnectionStaleGenerationIsNoop(t *testing.T) {
	cfg := ServerConfig{ID: "demo", Name: "Demo", Enabled: true, Transport: TransportStdio}
	conn, kill := newInMemoryConnectedServer(t, cfg)

	mgr := NewManager(Config{Enabled: true})
	mgr.mu.Lock()
	mgr.servers["demo"] = &managedServer{cfg: cfg, status: StatusConnected, generation: 5}
	mgr.mu.Unlock()

	kill()

	done := make(chan struct{})
	go func() {
		// gen=3 is stale relative to the entry's live generation (5).
		mgr.monitorConnection("demo", 3, conn)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitorConnection did not return in time")
	}

	mgr.mu.RLock()
	status := mgr.servers["demo"].status
	mgr.mu.RUnlock()

	if status != StatusConnected {
		t.Fatalf("status = %q, want %q (stale-generation monitor should be a no-op)", status, StatusConnected)
	}
}

// TestMonitorConnectionUnknownServerIsNoop verifies that if the server entry
// has been removed entirely (e.g. Reload rebuilt the servers map) by the time
// Wait() unblocks, monitorConnection returns without panicking.
func TestMonitorConnectionUnknownServerIsNoop(t *testing.T) {
	cfg := ServerConfig{ID: "demo", Name: "Demo", Enabled: true, Transport: TransportStdio}
	conn, kill := newInMemoryConnectedServer(t, cfg)

	mgr := NewManager(Config{Enabled: true})
	// Intentionally do not register "demo" in mgr.servers.

	kill()

	done := make(chan struct{})
	go func() {
		mgr.monitorConnection("demo", 1, conn)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitorConnection did not return in time")
	}
}

// TestAutoReconnectBailsWhenAlreadyConnected verifies autoReconnect exits on
// its first backoff tick without attempting a connect when some other path
// (a manual Reconnect racing this loop) already restored the connection.
func TestAutoReconnectBailsWhenAlreadyConnected(t *testing.T) {
	withFastBackoff(t, []time.Duration{time.Millisecond, time.Hour})

	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"}},
	})
	mgr.mu.Lock()
	mgr.servers["a"] = &managedServer{cfg: mgr.cfg.Servers[0], status: StatusConnected}
	mgr.mu.Unlock()

	done := make(chan struct{})
	go func() {
		mgr.autoReconnect("a")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("autoReconnect did not bail promptly when already connected")
	}

	mgr.mu.RLock()
	status := mgr.servers["a"].status
	mgr.mu.RUnlock()
	if status != StatusConnected {
		t.Fatalf("status = %q, want %q (should be untouched)", status, StatusConnected)
	}
}

// TestAutoReconnectBailsWhenDisabled verifies autoReconnect stops retrying
// once the server (or MCP globally) has been disabled, rather than
// hammering a connect attempt against a server the user turned off.
func TestAutoReconnectBailsWhenDisabled(t *testing.T) {
	withFastBackoff(t, []time.Duration{time.Millisecond, time.Hour})

	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: "a", Name: "A", Enabled: false, Transport: TransportStdio, Command: "false"}},
	})
	mgr.mu.Lock()
	mgr.servers["a"] = &managedServer{cfg: mgr.cfg.Servers[0], status: StatusError, lastError: "prior failure"}
	mgr.mu.Unlock()

	done := make(chan struct{})
	go func() {
		mgr.autoReconnect("a")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("autoReconnect did not bail promptly when disabled")
	}

	mgr.mu.RLock()
	entry := mgr.servers["a"]
	status, lastError := entry.status, entry.lastError
	mgr.mu.RUnlock()
	if status != StatusError {
		t.Fatalf("status = %q, want %q (untouched)", status, StatusError)
	}
	if lastError != "prior failure" {
		t.Fatalf("lastError = %q, want unchanged %q (no connect attempt should have run)", lastError, "prior failure")
	}
}

// TestAutoReconnectBailsWhileReloading verifies autoReconnect defers to an
// in-flight Reload rather than racing Start()'s from-scratch rebuild of the
// servers map.
func TestAutoReconnectBailsWhileReloading(t *testing.T) {
	withFastBackoff(t, []time.Duration{time.Millisecond, time.Hour})

	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"}},
	})
	mgr.mu.Lock()
	mgr.servers["a"] = &managedServer{cfg: mgr.cfg.Servers[0], status: StatusError, lastError: "prior failure"}
	mgr.reloading = true
	mgr.mu.Unlock()

	done := make(chan struct{})
	go func() {
		mgr.autoReconnect("a")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("autoReconnect did not bail promptly while reloading")
	}

	mgr.mu.RLock()
	lastError := mgr.servers["a"].lastError
	mgr.mu.RUnlock()
	if lastError != "prior failure" {
		t.Fatalf("lastError = %q, want unchanged %q (no connect attempt should have run)", lastError, "prior failure")
	}
}

// TestAutoReconnectBailsWhenServerRemoved verifies autoReconnect stops if the
// server entry has been removed entirely (e.g. deleted from settings).
func TestAutoReconnectBailsWhenServerRemoved(t *testing.T) {
	withFastBackoff(t, []time.Duration{time.Millisecond, time.Hour})

	mgr := NewManager(Config{Enabled: true})

	done := make(chan struct{})
	go func() {
		mgr.autoReconnect("missing")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("autoReconnect did not bail promptly for a removed server")
	}
}

// TestAutoReconnectRetriesUntilExhausted verifies autoReconnect attempts a
// connect on every backoff tick (via connectOne) and, once the schedule is
// exhausted against a server that keeps failing, leaves the server in
// StatusError with a fresh lastError from the final attempt rather than
// retrying forever.
func TestAutoReconnectRetriesUntilExhausted(t *testing.T) {
	withFastBackoff(t, []time.Duration{time.Millisecond, time.Millisecond})

	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"}},
	})
	mgr.mu.Lock()
	mgr.servers["a"] = &managedServer{cfg: mgr.cfg.Servers[0], status: StatusError, lastError: "prior failure"}
	mgr.mu.Unlock()

	done := make(chan struct{})
	go func() {
		mgr.autoReconnect("a")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("autoReconnect did not finish its bounded retry schedule")
	}

	mgr.mu.RLock()
	entry := mgr.servers["a"]
	status, lastError, gen := entry.status, entry.lastError, entry.generation
	mgr.mu.RUnlock()
	if status != StatusError {
		t.Fatalf("status = %q, want %q", status, StatusError)
	}
	if lastError == "prior failure" || lastError == "" {
		t.Fatalf("lastError = %q, want it replaced by a fresh connect failure message", lastError)
	}
	if gen == 0 {
		t.Fatal("generation was never bumped; connectOne does not appear to have run")
	}
}

// TestConnectOneDiscardsResultWhenSuperseded verifies the race-safety
// invariant documented on connectOne: if a newer connect/reconnect/reload
// bumps the entry's generation while an in-flight connectServer call is
// still unlocked, the original (now-stale) connectOne discards its result
// instead of clobbering the fresher state installed by the winner.
func TestConnectOneDiscardsResultWhenSuperseded(t *testing.T) {
	cfg := ServerConfig{
		ID: "a", Name: "A", Enabled: true, Transport: TransportStdio,
		Command: "sh", Args: []string{"-c", "sleep 0.3; exit 1"},
	}
	mgr := NewManager(Config{Enabled: true, Servers: []ServerConfig{cfg}})
	mgr.mu.Lock()
	mgr.servers["a"] = &managedServer{cfg: cfg, status: StatusDisconnected}
	mgr.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- mgr.connectOne(context.Background(), "a")
	}()

	// Give the goroutine time to capture its generation and enter the
	// unlocked connectServer call before we simulate a superseding
	// connect/reconnect having already installed fresher state.
	time.Sleep(50 * time.Millisecond)

	mgr.mu.Lock()
	mgr.servers["a"].generation++
	mgr.servers["a"].status = StatusConnected
	mgr.servers["a"].lastError = ""
	mgr.mu.Unlock()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("connectOne did not return in time")
	}

	mgr.mu.RLock()
	status, lastError := mgr.servers["a"].status, mgr.servers["a"].lastError
	mgr.mu.RUnlock()
	if status != StatusConnected {
		t.Fatalf("status = %q, want %q (stale connectOne must not overwrite fresher state)", status, StatusConnected)
	}
	if lastError != "" {
		t.Fatalf("lastError = %q, want empty (stale connectOne must not overwrite fresher state)", lastError)
	}
}
