package agent

import (
	"context"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

const turnEventBuffer = 64

// TurnExecutor produces the events for one persisted agent turn.
//
// The interface is deliberately limited to the runner behavior shared by the
// HTTP, gateway, and autonomous adapters. Each adapter retains ownership of
// event delivery and completion policy.
type TurnExecutor interface {
	Run(context.Context, session.AgentTurn, chan<- event.Event) error
}

// RunHostedTurn owns the concurrent lifetime of one agent turn: start the
// runner, drain every emitted event, then return the runner's completion
// error. The event handler runs synchronously so adapters keep their existing
// ordering and backpressure behavior.
func RunHostedTurn(ctx context.Context, runner TurnExecutor, turn session.AgentTurn, onEvent func(event.Event)) error {
	events := make(chan event.Event, turnEventBuffer)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx, turn, events)
		close(events)
	}()

	for ev := range events {
		if onEvent != nil {
			onEvent(ev)
		}
	}
	return <-errCh
}
