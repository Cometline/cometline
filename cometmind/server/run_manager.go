package server

import (
	"context"
	"sync"

	"github.com/cometline/cometmind/internal/runstate"
)

type runHandle struct {
	id    uint64
	lease *runstate.Lease
}

// RunManager tracks one in-flight agent loop per session so abort can cancel it.
type RunManager struct {
	mu         sync.Mutex
	nextID     uint64
	cancels    map[string]runHandle
	state      *runstate.Service
	onFinished func(sessionID, runID string)
}

func NewRunManager(state *runstate.Service) *RunManager {
	return &RunManager{cancels: make(map[string]runHandle), state: state}
}

func (m *RunManager) SetOnFinished(fn func(sessionID, runID string)) {
	m.mu.Lock()
	m.onFinished = fn
	m.mu.Unlock()
}

func (m *RunManager) notifyFinished(sessionID, runID string) {
	m.mu.Lock()
	fn := m.onFinished
	m.mu.Unlock()
	if fn != nil {
		fn(sessionID, runID)
	}
}

func (m *RunManager) Start(parent context.Context, sessionID string) (context.Context, func(), error) {
	lease, err := m.state.Acquire(parent, sessionID, runstate.OwnerHTTP)
	if err != nil {
		return nil, nil, err
	}

	m.mu.Lock()
	m.nextID++
	handle := runHandle{id: m.nextID, lease: lease}
	m.cancels[sessionID] = handle
	m.mu.Unlock()

	finish := func() {
		m.mu.Lock()
		if current, ok := m.cancels[sessionID]; ok && current.id == handle.id {
			delete(m.cancels, sessionID)
		}
		m.mu.Unlock()
		if lease.Finish() {
			m.notifyFinished(sessionID, lease.RunID())
		}
	}

	return lease.Context(), finish, nil
}

// Release removes a run owned by another process and emits the same lifecycle
// callback as an in-process lease finish.
func (m *RunManager) Release(ctx context.Context, sessionID, runID string) (bool, error) {
	released, err := m.state.Release(ctx, sessionID, runID)
	if released {
		m.notifyFinished(sessionID, runID)
	}
	return released, err
}

func (m *RunManager) Cancel(sessionID string) bool {
	requested, _ := m.state.RequestAbort(context.Background(), sessionID)
	m.mu.Lock()
	handle, ok := m.cancels[sessionID]
	m.mu.Unlock()
	if ok {
		handle.lease.Cancel()
	}
	return requested || ok
}

// Running reports whether a session has an in-flight agent loop.
func (m *RunManager) Running(sessionID string) bool {
	running, err := m.state.Running(context.Background(), sessionID)
	return err == nil && running
}

func (m *RunManager) Current(ctx context.Context, sessionID string) (runID, owner string, ok bool) {
	run, err := m.state.Current(ctx, sessionID)
	if err != nil {
		return "", "", false
	}
	return run.RunID, run.Owner, true
}
