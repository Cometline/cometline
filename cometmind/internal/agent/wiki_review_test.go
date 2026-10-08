package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

func TestDecideWikiReview(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	web := []skillCall{{Name: "web_fetch", OK: true}}
	tests := []struct {
		name    string
		in      wikiReviewInput
		start   bool
		logSkip string
	}{
		{name: "successful fetch", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, Calls: web}, start: true},
		{name: "successful search", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, Calls: []skillCall{{Name: "web_search", OK: true}}}, start: true},
		{name: "repo read", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, Calls: []skillCall{{Name: "read_file", Path: "README.md", OK: true}}}, start: true},
		{name: "failed fetch", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, Calls: []skillCall{{Name: "web_fetch", OK: false}}}},
		{name: "chat only", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now}},
		{name: "already wrote wiki", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, Calls: []skillCall{
			{Name: "read_file", Path: "README.md", OK: true},
			{Name: "write_file", Path: "@runtime/wiki/concepts/example.md", OK: true},
		}}},
		{name: "child session", in: wikiReviewInput{UserChat: false, HasModel: true, Now: now, Calls: web}},
		{name: "cooldown", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, LastStarted: now.Add(-14 * time.Minute), Calls: web}},
		{name: "cooldown expired", in: wikiReviewInput{UserChat: true, HasModel: true, Now: now, LastStarted: now.Add(-15 * time.Minute), Calls: web}, start: true},
		{name: "missing model", in: wikiReviewInput{UserChat: true, HasModel: false, Now: now, Calls: web}, logSkip: "wiki.review.skipped_no_model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideWikiReview(tt.in)
			if got.Start != tt.start || got.LogSkip != tt.logSkip {
				t.Fatalf("decision = %+v", got)
			}
		})
	}
}

func TestWikiReviewStartsBesideSkillReview(t *testing.T) {
	ctx := context.Background()
	svc := reviewSessionService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := svc.NewSession(ctx, ws.ID, "chat-model", "chat-provider")
	if err != nil {
		t.Fatal(err)
	}
	if err := seedMutatingTurn(ctx, svc, parent.ID, 8); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.AppendAssistantStep(ctx, parent.ID, "", nil, []cometsdk.ToolCallBlock{{
		ID: "web1", Name: "web_fetch", Input: json.RawMessage(`{"url":"https://example.com"}`),
	}}, nil); err != nil {
		t.Fatal(err)
	}
	msgs, err := svc.ListMessageRows(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	var assistantID string
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			assistantID = msgs[i].ID
			break
		}
	}
	calls, err := svc.ListToolCallsForSession(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call.MessageID == assistantID && call.ToolName == "web_fetch" {
			if _, err := svc.AppendToolResultMessage(ctx, parent.ID, call.ID, "page", false); err != nil {
				t.Fatal(err)
			}
		}
	}

	hub := event.NewHub()
	sub := hub.Subscribe()
	defer sub.Close()
	var wikiChild, skillChild string
	runner := &Runner{
		Config: &config.Config{Memory: config.MemoryConfig{
			ExtractionProvider: "extract-provider",
			ExtractionModel:    "extract-model",
		}},
		Sessions: svc,
		Events:   hub,
		ReviewChild: func(_ context.Context, child session.Session, registry *tools.Registry, _ int, _ string) error {
			skillChild = child.ID
			if registry.Has("web_fetch") {
				t.Fatal("skill review can fetch the web")
			}
			return nil
		},
		WikiChild: func(_ context.Context, child session.Session, registry *tools.Registry, maxSteps int, _ string) error {
			wikiChild = child.ID
			if maxSteps != wikiReviewMaxSteps {
				t.Fatalf("max steps = %d", maxSteps)
			}
			if registry.Has("web_search") || registry.Has("write_skill") || registry.Has("run_command") {
				t.Fatal("wiki registry is too wide")
			}
			if !registry.Has("web_fetch") || !registry.Has("write_file") {
				t.Fatal("wiki registry missing write or fetch")
			}
			raw := []byte(`{"path":"@runtime/wiki/index.md","content":"# Index\n"}`)
			_, ids, err := svc.AppendAssistantStep(ctx, child.ID, "", nil, []cometsdk.ToolCallBlock{{
				ID: "w1", Name: "write_file", Input: raw,
			}}, nil)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if _, err := svc.AppendToolResultMessage(ctx, child.ID, id, "ok", false); err != nil {
					return err
				}
			}
			return nil
		},
	}
	turn := session.AgentTurnFromSession(parent)
	runner.reviewSkillsAfterTurn(ctx, turn)
	runner.reviewWikiAfterTurn(ctx, turn)
	if skillChild == "" || wikiChild == "" {
		t.Fatalf("skill=%q wiki=%q", skillChild, wikiChild)
	}
	if _, err := svc.GetSession(ctx, wikiChild); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("wiki child remains: %v", err)
	}
	select {
	case ev := <-sub.Events:
		if ev.Kind != event.KindWikiReviewUpdated || len(ev.WikiPaths) != 1 || ev.WikiPaths[0] != "@runtime/wiki/index.md" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("missing wiki_review_updated")
	}
}
