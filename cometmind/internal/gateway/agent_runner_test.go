package gateway

import (
	"context"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

type fakeTurnRunner struct {
	events []event.Event
}

func (f fakeTurnRunner) Run(_ context.Context, _ session.AgentTurn, ch chan<- event.Event) error {
	for _, ev := range f.events {
		ch <- ev
	}
	return nil
}

func TestAgentRunnerRunTurnClosesChannel(t *testing.T) {
	t.Parallel()
	var reply strings.Builder
	ar := AgentRunner{
		NewRunner: func(_ session.Session, _ string) (TurnRunner, error) {
			return fakeTurnRunner{
				events: []event.Event{
					event.TextDelta("hello"),
					event.Done(),
				},
			}, nil
		},
	}
	err := ar.RunTurn(context.Background(), session.Session{ID: "sess-1"}, "/tmp", "hi", func(ev event.Event) {
		if ev.Kind == event.KindTextDelta {
			reply.WriteString(ev.Delta)
		}
	})
	if err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}
	if reply.String() != "hello" {
		t.Fatalf("reply = %q, want hello", reply.String())
	}
}
