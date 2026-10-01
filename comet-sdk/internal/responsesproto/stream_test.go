package responsesproto

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
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
	t.Parallel()

	tests := []struct {
		name string
		sse  string
	}{
		{"mid-stream delta", "event: response.output_text.delta\ndata: {\"delta\":\"hi\"}\n\n"},
		{"implicit finish at EOF", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := &closeTrackingBody{Reader: strings.NewReader(tt.sse)}
			ctx, cancel := context.WithCancel(context.Background())
			ch := make(chan cometsdk.Event) // never read
			done := make(chan struct{})
			go func() {
				ParseLoop(ctx, "codex", "gpt-5", true, body, ch, slog.New(slog.DiscardHandler), 0)
				close(done)
			}()

			time.Sleep(20 * time.Millisecond)
			cancel()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("ParseLoop blocked on an abandoned channel after ctx was cancelled")
			}
			if !body.closed.Load() {
				t.Fatal("response body should be closed")
			}
		})
	}
}
