package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestListServersReportsReloadingOverlay verifies that while a Reload is in
// flight, ListServers only overlays StatusDisconnected as Reloading. connecting,
// connected, and error stay visible so one slow handshake cannot mask neighbors.
func TestListServersReportsReloadingOverlay(t *testing.T) {
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{
			{ID: "disc", Name: "Disc", Enabled: true, Transport: TransportStdio, Command: "false"},
			{ID: "err", Name: "Err", Enabled: true, Transport: TransportStdio, Command: "false"},
			{ID: "wait", Name: "Wait", Enabled: true, Transport: TransportStdio, Command: "false"},
			{ID: "ok", Name: "OK", Enabled: true, Transport: TransportStdio, Command: "false"},
			{ID: "off", Name: "Off", Enabled: false, Transport: TransportStdio, Command: "false"},
		},
	})
	mgr.Start(context.Background())

	mgr.mu.Lock()
	mgr.reloading = true
	mgr.servers["disc"].status = StatusDisconnected
	mgr.servers["err"].status = StatusError
	mgr.servers["wait"].status = StatusConnecting
	mgr.servers["ok"].status = StatusConnected
	mgr.mu.Unlock()

	during := statusByID(mgr.ListServers())
	if during["disc"] != StatusReloading {
		t.Fatalf("disconnected during reload = %q, want %q", during["disc"], StatusReloading)
	}
	if during["err"] != StatusError {
		t.Fatalf("error during reload = %q, want %q", during["err"], StatusError)
	}
	if during["wait"] != StatusConnecting {
		t.Fatalf("connecting during reload = %q, want %q", during["wait"], StatusConnecting)
	}
	if during["ok"] != StatusConnected {
		t.Fatalf("connected during reload = %q, want %q", during["ok"], StatusConnected)
	}
	if during["off"] != StatusDisabled {
		t.Fatalf("disabled during reload = %q, want %q", during["off"], StatusDisabled)
	}
}

// TestReconnectRejectsWhileReloading guards against a Reconnect call racing
// Reload's Start(), which rebuilds the servers map from scratch and would
// otherwise silently discard or corrupt a concurrent manual reconnect.
func TestReconnectRejectsWhileReloading(t *testing.T) {
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{
			{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"},
		},
	})
	mgr.Start(context.Background())

	mgr.mu.Lock()
	mgr.reloading = true
	mgr.mu.Unlock()

	err := mgr.Reconnect(context.Background(), "a")
	if err == nil {
		t.Fatal("Reconnect() during reload: want error, got nil")
	}
	if !strings.Contains(err.Error(), "reloading") {
		t.Fatalf("Reconnect() error = %q, want mention of reloading", err.Error())
	}
}

// TestTestServerRejectsWhileReloading mirrors TestReconnectRejectsWhileReloading
// for the ephemeral connect-test path.
func TestTestServerRejectsWhileReloading(t *testing.T) {
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{
			{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"},
		},
	})
	mgr.Start(context.Background())

	mgr.mu.Lock()
	mgr.reloading = true
	mgr.mu.Unlock()

	result := mgr.TestServer(context.Background(), "a")
	if result.OK {
		t.Fatal("TestServer() during reload: OK = true, want false")
	}
	if !strings.Contains(result.Error, "reloading") {
		t.Fatalf("TestServer() error = %q, want mention of reloading", result.Error)
	}
}

// TestReloadPublishesNewConfigBeforeReconnect ensures Settings UI refreshes see
// the just-saved server list immediately, even though actual reconnects run in
// the background.
func TestReloadPublishesNewConfigBeforeReconnect(t *testing.T) {
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{
			{ID: "old", Name: "Old", Enabled: true, Transport: TransportStdio, Command: "false"},
		},
	})
	mgr.Start(context.Background())

	if err := mgr.Reload(context.Background(), Config{
		Enabled: false,
		Servers: []ServerConfig{
			{ID: "new", Name: "New", Enabled: true, Transport: TransportStdio, Command: "false"},
		},
	}); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	statuses := statusByID(mgr.ListServers())
	if _, ok := statuses["old"]; ok {
		t.Fatalf("old server still present after Reload(): %#v", statuses)
	}
	if statuses["new"] != StatusDisabled {
		t.Fatalf("status[new] = %q, want %q", statuses["new"], StatusDisabled)
	}
}

// TestReloadClearsReloadingFlagAfterBackgroundReconnect ensures the reloading
// overlay is cleared after Reload's background reconnect finishes — a stuck
// true here would permanently mask real server status.
func TestReloadClearsReloadingFlagOnCompletion(t *testing.T) {
	mgr := NewManager(Config{Enabled: true})
	mgr.Start(context.Background())

	if err := mgr.Reload(context.Background(), Config{
		Enabled: true,
		Servers: []ServerConfig{
			{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"},
		},
	}); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		mgr.mu.RLock()
		reloading := mgr.reloading
		mgr.mu.RUnlock()
		if !reloading {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reloading flag still true after background reconnect should have finished")
		}
		time.Sleep(10 * time.Millisecond)
	}

	statuses := statusByID(mgr.ListServers())
	if statuses["a"] == StatusReloading {
		t.Fatal("status[a] still reports Reloading after Reload() returned")
	}
}

func statusByID(statuses []ServerRuntimeStatus) map[string]ServerStatus {
	out := make(map[string]ServerStatus, len(statuses))
	for _, s := range statuses {
		out[s.ID] = s.Status
	}
	return out
}
