package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"go.uber.org/zap"
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

	logging.L().Error("mcp.session_closed", zap.String("server", serverID), zap.Error(waitErr))
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
			logging.L().Info("mcp.auto_reconnect_succeeded", zap.String("server", serverID), zap.Int("attempt", attempt+1))
			return
		}
		if skipAutoReconnect(errorCodeOf(err)) {
			return
		}
		logging.L().Warn("mcp.auto_reconnect_failed", zap.String("server", serverID), zap.Int("attempt", attempt+1), zap.Error(err))
	}
}

func wakeRecoveryIDs(servers map[string]*managedServer) []string {
	ids := make([]string, 0)
	for id, entry := range servers {
		// StatusConnected is included because a lid close does not kill the
		// process. At resume the transport is already dead, but the cached
		// status stays connected until the next keepalive. Waiting for that
		// ping misses the only wake pass.
		if entry == nil || !entry.cfg.Enabled {
			continue
		}
		if entry.status != StatusError && entry.status != StatusConnected {
			continue
		}
		if skipAutoReconnect(entry.errorCode) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

const wakeReconnectSpacing = 2 * time.Second

// wakeSpacing is a var so tests can collapse the gap between servers.
var wakeSpacing = wakeReconnectSpacing

// RecoverAfterWake reconnects enabled servers left in a transient error after
// the machine slept. A lid close does not kill this process, so the session
// can look connected until the next ping, and the short death-retry budget
// often expires before the network is back. Auth, config, and missing-binary
// errors are left for the user.
func (m *Manager) RecoverAfterWake(parent context.Context) int {
	if m == nil {
		return 0
	}
	if parent == nil {
		parent = context.Background()
	}
	m.mu.Lock()
	if m.wakeInFlight || m.reloading || !m.cfg.Enabled {
		m.mu.Unlock()
		return 0
	}
	ids := wakeRecoveryIDs(m.servers)
	if len(ids) == 0 {
		m.mu.Unlock()
		return 0
	}
	m.wakeInFlight = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.wakeInFlight = false
		m.mu.Unlock()
	}()

	recovered := 0
	for i, id := range ids {
		if i > 0 {
			select {
			case <-parent.Done():
				return recovered
			case <-time.After(wakeSpacing):
			}
		}
		m.mu.RLock()
		entry, ok := m.servers[id]
		stillDue := ok &&
			(entry.status == StatusError || entry.status == StatusConnected) &&
			!skipAutoReconnect(entry.errorCode)
		m.mu.RUnlock()
		if !stillDue {
			continue
		}
		if err := m.connectOneWithBudget(context.Background(), id); err != nil {
			logging.L().Warn("mcp.wake_reconnect_failed", zap.String("server", id), zap.Error(err))
			continue
		}
		recovered++
		logging.L().Info("mcp.wake_reconnect_succeeded", zap.String("server", id))
	}
	return recovered
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
