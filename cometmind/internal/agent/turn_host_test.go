package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
)

type turnExecutorFunc func(context.Context, session.AgentTurn, chan<- event.Event) error

func (f turnExecutorFunc) Run(ctx context.Context, turn session.AgentTurn, events chan<- event.Event) error {
	return f(ctx, turn, events)
}

func TestRunHostedTurnDrainsEventsInOrder(t *testing.T) {
	runner := turnExecutorFunc(func(_ context.Context, _ session.AgentTurn, events chan<- event.Event) error {
		events <- event.TextDelta("first")
		events <- event.TextDelta("second")
		return nil
	})

	var got []string
	err := RunHostedTurn(context.Background(), runner, session.AgentTurn{ID: "turn"}, func(ev event.Event) {
		got = append(got, ev.Delta)
	})
	if err != nil {
		t.Fatalf("RunHostedTurn() error = %v", err)
	}
	if want := []string{"first", "second"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

func TestRunHostedTurnDrainsEventsBeforeReturningRunnerError(t *testing.T) {
	wantErr := errors.New("runner failed")
	runner := turnExecutorFunc(func(_ context.Context, _ session.AgentTurn, events chan<- event.Event) error {
		events <- event.TextDelta("persist me")
		return wantErr
	})

	var got []string
	err := RunHostedTurn(context.Background(), runner, session.AgentTurn{ID: "turn"}, func(ev event.Event) {
		got = append(got, ev.Delta)
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunHostedTurn() error = %v, want %v", err, wantErr)
	}
	if len(got) != 1 || got[0] != "persist me" {
		t.Fatalf("events = %q, want persisted event before error", got)
	}
}
