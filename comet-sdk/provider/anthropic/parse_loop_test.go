package anthropic

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/stretchr/testify/require"
)

type closeTrackingBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *closeTrackingBody) Close() error {
	b.closed.Store(true)
	return nil
}

func TestParseLoop_AbandonedConsumerReturnsOnCancel(t *testing.T) {
	data, err := os.ReadFile("fixtures/text_response.sse")
	require.NoError(t, err)
	body := &closeTrackingBody{Reader: bytes.NewReader(data)}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan cometsdk.Event) // never read
	done := make(chan struct{})
	go func() {
		parseLoop(ctx, providerID, body, ch, slog.New(slog.DiscardHandler), 0)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("parseLoop blocked on an abandoned channel after ctx was cancelled")
	}
	require.True(t, body.closed.Load(), "response body should be closed")
}
