package inboxworker

import (
	"testing"

	"github.com/cometline/cometmind/internal/config"
)

func TestWorkerUpdateConfigSignalsReload(t *testing.T) {
	w := &Worker{}
	changed := w.reloadChannel()
	w.UpdateConfig(config.InboxConfig{PollIntervalSeconds: 15}, "model", "provider")

	select {
	case <-changed:
	default:
		t.Fatal("UpdateConfig did not signal the worker")
	}
	if got := w.configSnapshot().PollIntervalSeconds; got != 15 {
		t.Fatalf("poll interval = %d, want 15", got)
	}
	if w.DefaultModelID != "model" || w.DefaultProviderID != "provider" {
		t.Fatalf("defaults = %q/%q, want model/provider", w.DefaultModelID, w.DefaultProviderID)
	}
}
