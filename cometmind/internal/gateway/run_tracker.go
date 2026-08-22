package gateway

import (
	"context"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/runstate"
)

type turnHandle struct {
	id    uint64
	lease *runstate.Lease
	done  chan struct{}
}

// TurnRunTracker tracks one in-flight agent turn per session for job reconcile.
type TurnRunTracker struct {
	mu      sync.Mutex
	nextID  uint64
	cancels map[string]turnHandle
	state   *runstate.Service
}

func NewTurnRunTracker(state *runstate.Service) *TurnRunTracker {
	return &TurnRunTracker{cancels: make(map[string]turnHandle), state: state}
}

func (m *TurnRunTracker) Start(parent context.Context, sessionID string) (context.Context, func(), error) {
	lease, err := m.state.Acquire(parent, sessionID, runstate.OwnerGateway)
	if err != nil {
		return nil, nil, err
	}

	m.mu.Lock()
	m.nextID++
	handle := turnHandle{id: m.nextID, lease: lease, done: make(chan struct{})}
	m.cancels[sessionID] = handle
	m.mu.Unlock()

	finish := func() {
		m.mu.Lock()
		if current, ok := m.cancels[sessionID]; ok && current.id == handle.id {
			delete(m.cancels, sessionID)
			close(handle.done)
		}
		m.mu.Unlock()
		lease.Finish()
	}

	return lease.Context(), finish, nil
}

// Stop cancels the active turn for sessionID and returns a channel that closes
// after the turn and its cleanup have finished.
func (m *TurnRunTracker) Stop(sessionID string) (<-chan struct{}, bool) {
	requested, _ := m.state.RequestAbort(context.Background(), sessionID)
	m.mu.Lock()
	handle, ok := m.cancels[sessionID]
	m.mu.Unlock()
	if ok {
		handle.lease.Cancel()
		return handle.done, true
	}
	if !requested {
		return nil, false
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if !m.Running(sessionID) {
				return
			}
		}
	}()
	return done, true
}

// Running reports whether a session has an in-flight gateway turn.
func (m *TurnRunTracker) Running(sessionID string) bool {
	running, err := m.state.Running(context.Background(), sessionID)
	return err == nil && running
}

func (m *TurnRunTracker) RunID(sessionID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if handle, ok := m.cancels[sessionID]; ok {
		return handle.lease.RunID()
	}
	return ""
}
