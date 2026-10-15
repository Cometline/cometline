package agent

import (
	"context"
	"strings"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/session"
)

func TestForceCompactSkipsWithoutPrefix(t *testing.T) {
	t.Parallel()

	opened := false
	outcome, err := (&ContextCompactor{
		Sessions: &fakeStore{},
		Config:   &config.Config{},
	}).ForceCompact(context.Background(), sessionWithModel(), func(string, string) (cometsdk.Provider, error) {
		opened = true
		return nil, nil
	})
	if err != nil {
		t.Fatalf("ForceCompact() error = %v", err)
	}
	if outcome.Status != CompactStatusSkipped || outcome.Reason != "no_prefix" {
		t.Fatalf("outcome = %+v, want skipped no_prefix", outcome)
	}
	if opened {
		t.Fatal("provider opened for an empty transcript")
	}
}

func TestForceCompactSkipsWhenRecentWindowCoversTranscript(t *testing.T) {
	t.Parallel()

	store := &fakeStore{rows: []db.Message{
		{ID: "u1", Role: "user", Content: "first goal"},
		{ID: "u2", Role: "user", Content: "second goal"},
		{ID: "u3", Role: "user", Content: "current ask"},
	}}
	opened := false
	outcome, err := (&ContextCompactor{Sessions: store, Config: &config.Config{}}).ForceCompact(
		context.Background(),
		sessionWithModel(),
		func(string, string) (cometsdk.Provider, error) {
			opened = true
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("ForceCompact() error = %v", err)
	}
	if outcome.Status != CompactStatusSkipped || outcome.Reason != "no_prefix" {
		t.Fatalf("outcome = %+v, want skipped no_prefix", outcome)
	}
	if opened {
		t.Fatal("provider opened when the transcript already fits the keep window")
	}
}

func TestForceCompactFoldsHistoryBeyondKeepWindow(t *testing.T) {
	t.Parallel()

	rows := []db.Message{
		{ID: "u1", Role: "user", Content: strings.Repeat("older context ", 8000)},
		{ID: "u2", Role: "user", Content: "current ask"},
	}
	store := &fakeStore{rows: rows}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "Kept the first goals."},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}
	outcome, err := (&ContextCompactor{Sessions: store, Config: &config.Config{}}).ForceCompact(
		context.Background(),
		sessionWithModel(),
		func(string, string) (cometsdk.Provider, error) { return provider, nil },
	)
	if err != nil {
		t.Fatalf("ForceCompact() error = %v", err)
	}
	if outcome.Status != CompactStatusCompacted {
		t.Fatalf("status = %s reason = %s, want compacted", outcome.Status, outcome.Reason)
	}
	if store.compactedUntil != "u1" {
		t.Fatalf("compacted until %q, want u1", store.compactedUntil)
	}
}

func TestForceCompactSummarizesOlderTurns(t *testing.T) {
	t.Parallel()

	rows := []db.Message{
		{ID: "old", Role: "user", Content: strings.Repeat("older context ", 8000)},
		{ID: "recent", Role: "user", Content: "keep this goal"},
	}
	store := &fakeStore{rows: rows}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "Goals: keep going."},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}
	outcome, err := (&ContextCompactor{
		Sessions: store,
		Config:   &config.Config{DefaultProviderID: "test", DefaultModelID: "test-model"},
	}).ForceCompact(context.Background(), sessionWithModel(), func(providerID, modelID string) (cometsdk.Provider, error) {
		if providerID != "prov" || modelID != "model" {
			t.Fatalf("open provider %s %s", providerID, modelID)
		}
		return provider, nil
	})
	if err != nil {
		t.Fatalf("ForceCompact() error = %v", err)
	}
	if outcome.Status != CompactStatusCompacted {
		t.Fatalf("status = %s reason = %s, want compacted", outcome.Status, outcome.Reason)
	}
	if store.compactCalls != 1 || store.contextSummary != "Goals: keep going." {
		t.Fatalf("summary calls = %d summary = %q", store.compactCalls, store.contextSummary)
	}
}

func sessionWithModel() session.Session {
	return session.Session{ID: "s1", ProviderID: "prov", ModelID: "model"}
}
