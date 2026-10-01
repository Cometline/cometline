package anthropic

import (
	"context"
	"io"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/internal/providerbase"
	"github.com/Cometline/cometline/comet-sdk/internal/sse"
	"go.uber.org/zap"
)

// parseLoop reads SSE events from body and dispatches typed cometsdk.Events to ch.
// It is run in a goroutine by Stream(). It always closes ch and body before returning.
func parseLoop(ctx context.Context, providerID string, body io.ReadCloser, ch chan<- cometsdk.Event, log *zap.Logger, idleTimeout time.Duration) {
	defer close(ch)
	defer body.Close()

	scanner := sse.NewIdleScanner(body, idleTimeout)
	defer scanner.Close()
	state := newStreamState()

	for scanner.Next() {
		// Respect context cancellation between events.
		select {
		case <-ctx.Done():
			providerbase.SendEvent(ctx, ch, cometsdk.ErrorEvent{Err: &cometsdk.StreamError{
				ProviderID: providerID,
				Cause:      ctx.Err(),
			}})
			return
		default:
		}

		ev := scanner.Event()
		log.Debug("sse.event", zap.String("type", ev.Type), zap.String("data", ev.Data))

		events, err := toSDKEvents(ev.Type, ev.Data, state)
		if err != nil {
			log.Debug("sse.parse_error", zap.Error(err))
			providerbase.SendEvent(ctx, ch, cometsdk.ErrorEvent{Err: &cometsdk.StreamError{
				ProviderID: providerID,
				Cause:      err,
			}})
			return
		}

		for _, e := range events {
			log.Debug("sdk.event", zap.Any("type", e))
			if !providerbase.SendEvent(ctx, ch, e) {
				return
			}
			// DoneEvent is terminal — stop reading.
			if _, ok := e.(cometsdk.DoneEvent); ok {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Debug("sse.scanner_error", zap.Error(err))
		providerbase.SendEvent(ctx, ch, cometsdk.ErrorEvent{Err: &cometsdk.StreamError{
			ProviderID: providerID,
			Cause:      err,
		}})
	}
}
