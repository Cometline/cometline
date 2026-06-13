package tui

import "github.com/cometline/cometmind/internal/event"

// AgentEventMsg carries one streamed agent event onto the Bubble Tea update loop.
type AgentEventMsg struct {
	Event event.Event
}

// RunFinishedMsg is sent after the runner goroutine exits (after the event channel drains).
type RunFinishedMsg struct {
	Err error
}

// SessionBackMsg requests returning to the session list (from chat, Esc when idle).
type SessionBackMsg struct{}
