package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/logging"
)

// mcpAutoReconnectBackoff defines the bounded automatic-reconnect policy
// applied when monitorConnection observes a live session die unexpectedly.
// After these attempts are exhausted, the server is left in StatusError with
// lastError set — matching manual recovery via the existing
// reconnection-runs API / Settings UI reconnect button, rather than retrying
// silently forever.
var mcpAutoReconnectBackoff = []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second}

// monitorConnection watches one connected session for the underlying
// transport dying — a failed keepalive ping (see defaultKeepAlive in
// client.go), a stdio subprocess exiting, or any other transport-level
// error — and reacts by correcting the cached status (which would otherwise
// stay StatusConnected forever) and driving a bounded automatic reconnect.
//
// gen is the generation captured at the moment this conn was installed. If,
// by the time session.Wait() returns, the entry's live generation no longer
// matches gen, some other connect/reconnect/reload/close already superseded
// this connection intentionally, and this goroutine is a no-op.
func (m *Manager) monitorConnection(serverID string, gen uint64, conn *connectedServer) {
	waitErr := conn.session.Wait()

	m.mu.Lock()
	entry, ok := m.servers[serverID]
	if !ok || entry.generation != gen {
		m.mu.Unlock()
		return
	}
	entry.conn = nil
	entry.status = StatusError
	if waitErr != nil {
		classified := classifyConnectErrorFor(serverID, waitErr)
		entry.lastError = classified.Error()
		entry.errorCode = classified.Code
		entry.errorHint = classified.Hint
	} else {
		entry.lastError = "MCP session closed unexpectedly"
		entry.errorCode = CodeTransportTimeout
		entry.errorHint = "The MCP server stopped responding. Click Reconnect."
	}
	m.mu.Unlock()

	logging.L().Error("mcp.session_closed", "server", serverID, "error", waitErr)
	m.autoReconnect(serverID)
}

// autoReconnect retries connectOne a bounded number of times with backoff
// after monitorConnection detects an unexpected session death. It bails out
// early if the server was disabled, the manager started reloading, or
// something else (a manual Reconnect racing this loop) already restored the
// connection — connectOne itself remains the single source of truth for
// generation-safe connect races, so this loop only needs to decide whether
// it's still worth attempting another connect.
func (m *Manager) autoReconnect(serverID string) {
	for attempt, delay := range mcpAutoReconnectBackoff {
		time.Sleep(delay)

		m.mu.RLock()
		entry, ok := m.servers[serverID]
		reloading := m.reloading
		alreadyConnected := ok && entry.status == StatusConnected
		enabled := ok && m.cfg.Enabled && entry.cfg.Enabled
		skipRetry := ok && skipAutoReconnect(entry.errorCode)
		m.mu.RUnlock()
		if !ok || reloading || !enabled || alreadyConnected || skipRetry {
			return
		}

		err := m.connectOneWithBudget(context.Background(), serverID)
		if err == nil {
			logging.L().Info("mcp.auto_reconnect_succeeded", "server", serverID, "attempt", attempt+1)
			return
		}
		if skipAutoReconnect(errorCodeOf(err)) {
			return
		}
		logging.L().Warn("mcp.auto_reconnect_failed", "server", serverID, "attempt", attempt+1, "error", err)
	}
}

// Reconnect disconnects and reconnects one server. It refuses to run while a
// full Reload is in flight: Reload's Start() rebuilds the servers map from
// scratch, so a Reconnect racing that rebuild could write a stale connection
// into an entry Start() is about to discard or has already superseded.
func (m *Manager) Reconnect(_ context.Context, serverID string) error {
	m.mu.Lock()
	if m.reloading {
		m.mu.Unlock()
		return fmt.Errorf("MCP is reloading; try again in a moment")
	}
	entry, ok := m.servers[serverID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	if !m.cfg.Enabled || !entry.cfg.Enabled {
		entry.status = StatusDisabled
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()
	// Detach from the inbound HTTP request: a cancelled browser tab must not
	// abort a 45s remote handshake the user just asked for.
	return m.connectOneWithBudget(context.Background(), serverID)
}
