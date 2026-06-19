package gateway

import (
	"context"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

// TurnRunner executes one persisted agent turn.
type TurnRunner interface {
	Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error
}

// AgentRunner runs one persisted turn using a factory-built agent loop.
type AgentRunner struct {
	NewRunner func(sess session.Session, workspacePath string) (TurnRunner, error)
}

// RunTurn executes one user message and streams events to the callback.
func (a AgentRunner) RunTurn(ctx context.Context, sess session.Session, workspacePath, text string, onEvent func(event.Event)) error {
	runner, err := a.NewRunner(sess, workspacePath)
	if err != nil {
		return err
	}
	ch := make(chan event.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx, session.AgentTurnFromSession(sess), ch)
		close(ch)
	}()
	for ev := range ch {
		if onEvent != nil {
			onEvent(ev)
		}
	}
	return <-errCh
}
