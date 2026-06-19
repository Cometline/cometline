package server

import (
	"context"
	"fmt"
	"sync"
)

type runHandle struct {
	id     uint64
	cancel context.CancelFunc
}

// RunManager tracks one in-flight agent loop per session so abort can cancel it.
type RunManager struct {
	mu      sync.Mutex
	nextID  uint64
	cancels map[string]runHandle
}

func NewRunManager() *RunManager {
	return &RunManager{cancels: make(map[string]runHandle)}
}

func (m *RunManager) Start(parent context.Context, sessionID string) (context.Context, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cancels[sessionID]; exists {
		return nil, nil, fmt.Errorf("session %s already running", sessionID)
	}

	ctx, cancel := context.WithCancel(parent)
	m.nextID++
	handle := runHandle{id: m.nextID, cancel: cancel}
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

func (m *RunManager) Cancel(sessionID string) bool {
	m.mu.Lock()
	handle, ok := m.cancels[sessionID]
	m.mu.Unlock()
	if ok {
		handle.cancel()
	}
	return ok
}
