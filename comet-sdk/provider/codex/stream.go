package codex

import (
	"context"
	"io"
	"log/slog"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/responsesproto"
)

type codexStreamState = responsesproto.StreamState

func toSDKEvents(eventType, data string, state *codexStreamState) ([]cometsdk.Event, error) {
	return responsesproto.ToSDKEvents(providerID, eventType, data, state)
}

func parseLoop(ctx context.Context, providerID, modelID string, emitToolStart bool, body io.ReadCloser, ch chan<- cometsdk.Event, log *slog.Logger, idleTimeout time.Duration) {
	responsesproto.ParseLoop(ctx, providerID, modelID, emitToolStart, body, ch, log, idleTimeout)
}

func redactEncryptedReasoning(data string) string {
	return responsesproto.RedactEncryptedReasoning(data)
}
