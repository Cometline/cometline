package anthropic

import (
	"context"
	"io"
	"log/slog"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/sse"
)

// parseLoop reads SSE events from body and dispatches typed cometsdk.Events to ch.
// It is run in a goroutine by Stream(). It always closes ch and body before returning.
func parseLoop(ctx context.Context, providerID string, body io.ReadCloser, ch chan<- cometsdk.Event, log *slog.Logger) {
	defer close(ch)
	defer body.Close()

	scanner := sse.NewScanner(body)
	state := newStreamState()

	for scanner.Next() {
		// Respect context cancellation between events.
		select {
		case <-ctx.Done():
			ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{
				ProviderID: providerID,
				Cause:      ctx.Err(),
			}}
			return
		default:
		}

		ev := scanner.Event()
		log.DebugContext(ctx, "sse.event", "type", ev.Type, "data", ev.Data)

		events, err := toSDKEvents(ev.Type, ev.Data, state)
		if err != nil {
			log.DebugContext(ctx, "sse.parse_error", "error", err)
			ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{
				ProviderID: providerID,
				Cause:      err,
			}}
			return
		}

		for _, e := range events {
			log.DebugContext(ctx, "sdk.event", "type", slog.AnyValue(e))
			ch <- e
			// DoneEvent is terminal — stop reading.
			if _, ok := e.(cometsdk.DoneEvent); ok {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.DebugContext(ctx, "sse.scanner_error", "error", err)
		ch <- cometsdk.ErrorEvent{Err: &cometsdk.StreamError{
			ProviderID: providerID,
			Cause:      err,
		}}
	}
}
