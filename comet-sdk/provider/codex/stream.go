package codex

import (
	"context"
	"io"
	"log/slog"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/responsesproto"
)

func parseLoop(ctx context.Context, providerID, modelID string, emitToolStart bool, body io.ReadCloser, ch chan<- cometsdk.Event, log *slog.Logger, idleTimeout time.Duration) {
	responsesproto.ParseLoop(ctx, providerID, modelID, emitToolStart, body, ch, log, idleTimeout)
}
