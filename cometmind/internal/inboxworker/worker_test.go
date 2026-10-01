package inboxworker

import (
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
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

func TestWorkerRegistryOptionsFillsHookGaps(t *testing.T) {
	jobsSvc := &jobs.Service{}
	hub := event.NewHub()
	sess := session.Session{ID: "sess-1"}

	w := &Worker{Jobs: jobsSvc, Events: hub}
	if got := w.registryOptions(sess, "/ws"); got.Jobs != jobsSvc || got.MemoryEvents != hub || got.SessionID != "sess-1" {
		t.Fatalf("default options = %+v", got)
	}

	var hookPath string
	w.RegistryOptions = func(_ session.Session, workspacePath string) tools.RegistryOptions {
		hookPath = workspacePath
		return tools.RegistryOptions{SessionID: "  "}
	}
	got := w.registryOptions(sess, "/ws")
	if hookPath != "/ws" {
		t.Fatalf("hook workspace path = %q, want /ws", hookPath)
	}
	if got.Jobs != jobsSvc || got.MemoryEvents != hub || got.SessionID != "sess-1" {
		t.Fatalf("hook options = %+v, want worker services and session filled in", got)
	}
}
