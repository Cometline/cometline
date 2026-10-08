package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

func TestDecideWikiReview(t *testing.T) {
	tests := []struct {
		name    string
		in      wikiReviewInput
		start   bool
		logSkip string
	}{
		{name: "ninth turn does not start", in: wikiReviewInput{UserChat: true, HasModel: true, Turns: 9}},
		{name: "tenth turn", in: wikiReviewInput{UserChat: true, HasModel: true, Turns: 10}, start: true},
		{name: "tool result without the turn count", in: wikiReviewInput{UserChat: true, HasModel: true}},
		{name: "chat only", in: wikiReviewInput{UserChat: true, HasModel: true}},
		{name: "child session", in: wikiReviewInput{UserChat: false, HasModel: true, Turns: 10}},
		{name: "missing model", in: wikiReviewInput{UserChat: true, HasModel: false, Turns: 10}, logSkip: "wiki.review.skipped_no_model"},
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

func TestTurnReviewCoalescesWhileRunning(t *testing.T) {
	ctx := context.Background()
	svc := reviewSessionService(t)
	ws, err := svc.EnsureWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mk := func(title string) session.Session {
		t.Helper()
		parent, err := svc.NewSession(ctx, ws.ID, "chat-model", "chat-provider")
		if err != nil {
			t.Fatal(err)
		}
		if err := seedNamedTurn(ctx, svc, parent.ID, title, 1); err != nil {
			t.Fatal(err)
		}
		if err := svc.SetSkillReviewCounter(ctx, parent.ID, 9, 1); err != nil {
			t.Fatal(err)
		}
		return parent
	}
	first := mk("first review")
	second := mk("second review")
	third := mk("third review")
	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	var prompts []string
	var running int
	var maxRunning int
	runner := &Runner{
		Config: &config.Config{Memory: config.MemoryConfig{
			ExtractionProvider: "extract-provider",
			ExtractionModel:    "extract-model",
		}},
		Sessions: svc,
		ReviewChild: func(_ context.Context, child session.Session, _ *tools.Registry, _ int, _ string) error {
			mu.Lock()
			running++
			if running > maxRunning {
				maxRunning = running
			}
			mu.Unlock()
			rows, err := svc.ListMessageRows(ctx, child.ID)
			if err != nil {
				return err
			}
			mu.Lock()
			for _, row := range rows {
				if row.Role == "user" {
					prompts = append(prompts, row.Content)
				}
			}
			mu.Unlock()
			if len(prompts) == 1 {
				close(started)
				<-release
			}
			mu.Lock()
			running--
			mu.Unlock()
			return nil
		},
	}
	go runner.reviewAfterTurn(ctx, session.AgentTurnFromSession(first))
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first review did not start")
	}
	runner.reviewAfterTurn(ctx, session.AgentTurnFromSession(second))
	runner.reviewAfterTurn(ctx, session.AgentTurnFromSession(third))
	close(release)
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		done := len(prompts) == 2 && running == 0
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("prompts = %#v running = %d", prompts, running)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if maxRunning != 1 {
		t.Fatalf("max running = %d, want 1", maxRunning)
	}
	if !strings.Contains(prompts[0], "first review") || !strings.Contains(prompts[1], "third review") || strings.Contains(prompts[1], "second review") {
		t.Fatalf("prompts = %#v", prompts)
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
	if err := seedMutatingTurn(ctx, svc, parent.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetSkillReviewCounter(ctx, parent.ID, 9, 1); err != nil {
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
	starts := 0
	var childID string
	runner := &Runner{
		Config: &config.Config{Memory: config.MemoryConfig{
			ExtractionProvider: "extract-provider",
			ExtractionModel:    "extract-model",
		}},
		Sessions: svc,
		Events:   hub,
		ReviewChild: func(_ context.Context, child session.Session, registry *tools.Registry, maxSteps int, prompt string) error {
			starts++
			childID = child.ID
			if maxSteps != reviewMaxSteps {
				t.Fatalf("max steps = %d", maxSteps)
			}
			if !strings.Contains(prompt, "write_skill") || !strings.Contains(prompt, "@runtime/wiki/") {
				t.Fatal("combined review prompt does not cover both jobs")
			}
			if !registry.Has("write_skill") || !registry.Has("write_file") || !registry.Has("web_fetch") {
				t.Fatal("combined registry missing a review tool")
			}
			if registry.Has("web_search") || registry.Has("run_command") {
				t.Fatal("combined registry is too wide")
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
	runner.reviewAfterTurn(ctx, turn)
	if starts != 1 || childID == "" {
		t.Fatalf("starts = %d child = %q", starts, childID)
	}
	if _, err := svc.GetSession(ctx, childID); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("review child remains: %v", err)
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
