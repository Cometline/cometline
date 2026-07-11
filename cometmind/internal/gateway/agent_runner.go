package gateway

import (
	"context"

	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

// TurnRunner executes one persisted agent turn.
type TurnRunner interface {
	Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error
}

// AgentRunner runs one persisted turn using a factory-built agent loop.
type AgentRunner struct {
	NewRunner func(sess session.Session, workspacePath string, msg InboundMessage) (TurnRunner, error)
}

// RunTurn executes one user message and streams events to the callback.
func (a AgentRunner) RunTurn(ctx context.Context, sess session.Session, workspacePath string, msg InboundMessage, onEvent func(event.Event)) error {
	runner, err := a.NewRunner(sess, workspacePath, msg)
	if err != nil {
		return err
	}
	return agent.RunHostedTurn(ctx, runner, session.AgentTurnFromSession(sess), func(ev event.Event) {
		if onEvent != nil {
			onEvent(ev)
		}
	})
}
