package gateway

import (
	"context"
	"fmt"
	"sync"
)

type turnHandle struct {
	id     uint64
	cancel context.CancelFunc
}

// TurnRunTracker tracks one in-flight agent turn per session for job reconcile.
type TurnRunTracker struct {
	mu      sync.Mutex
	nextID  uint64
	cancels map[string]turnHandle
}

func NewTurnRunTracker() *TurnRunTracker {
	return &TurnRunTracker{cancels: make(map[string]turnHandle)}
}

func (m *TurnRunTracker) Start(parent context.Context, sessionID string) (context.Context, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cancels[sessionID]; exists {
		return nil, nil, fmt.Errorf("session %s already running", sessionID)
	}

	ctx, cancel := context.WithCancel(parent)
	m.nextID++
	handle := turnHandle{id: m.nextID, cancel: cancel}
	m.cancels[sessionID] = handle

	finish := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if current, ok := m.cancels[sessionID]; ok && current.id == handle.id {
			delete(m.cancels, sessionID)
		}
		cancel()
	}

	return ctx, finish, nil
}

// Stop cancels the active turn for sessionID and reports whether one existed.
func (m *TurnRunTracker) Stop(sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	handle, ok := m.cancels[sessionID]
	if ok {
		handle.cancel()
	}
	return ok
}

// Running reports whether a session has an in-flight gateway turn.
func (m *TurnRunTracker) Running(sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cancels[sessionID]
	return ok
}
