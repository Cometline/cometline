package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
	_ "modernc.org/sqlite"
)

func TestTurnReviewSkillPromptActsOnClassLessons(t *testing.T) {
	got := turnReviewSystemPrompt(true, false)
	for _, want := range []string{
		"Be active",
		"overwrite=true",
		"not self-improvement",
		"never found a working method",
		"class of task",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	for _, banned := range []string{
		"only when it was demonstrated and verified",
		"If there is no complete verified workflow, do not call write_skill",
	} {
		if strings.Contains(got, banned) {
			t.Errorf("prompt still defaults to skipping: %q", banned)
		}
	}
	if strings.Contains(got, "@runtime/wiki/") {
		t.Fatal("skill-only prompt includes the wiki job")
	}
}

func TestSkillReviewForkUsesNarrowSurfaceAndDeletesChild(t *testing.T) {
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

	hub := event.NewHub()
	sub := hub.Subscribe()
	defer sub.Close()

	var seen []string
	var child session.Session
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	runner := &Runner{
		Config: &config.Config{Memory: config.MemoryConfig{
			ExtractionProvider: "extract-provider",
			ExtractionModel:    "extract-model",
		}},
		Sessions: svc,
		Events:   hub,
		ReviewNow: func() time.Time {
			return now
		},
		ReviewChild: func(_ context.Context, got session.Session, registry *tools.Registry, maxSteps int, systemPrompt string) error {
			child = got
			if maxSteps != reviewMaxSteps {
				t.Errorf("max steps = %d", maxSteps)
			}
			if systemPrompt == "" {
				t.Error("empty system prompt")
			}
			if got.ParentSessionID != parent.ID || got.Origin != "user" || got.SubagentKind != reviewKind {
				t.Errorf("child = %+v", got)
			}
			if got.ModelID != "extract-model" || got.ProviderID != "extract-provider" {
				t.Errorf("child model = %s/%s", got.ProviderID, got.ModelID)
			}
			for _, tool := range registry.CometSDK() {
				seen = append(seen, tool.Name)
			}
			for _, required := range []string{"write_skill", "write_file", "web_fetch"} {
				if !registry.Has(required) {
					t.Errorf("registry missing %s", required)
				}
			}
			for _, forbidden := range []string{"run_command", "write_skill_draft", "promote_skill_draft", "web_search", "spawn_general_agent"} {
				if registry.Has(forbidden) {
					t.Errorf("registry has %s", forbidden)
				}
			}
			content := "---\nname: ship-checklist\ndescription: Ship a reviewed change\n---\n\n# Steps\n"
			raw, err := json.Marshal(map[string]any{"name": "ship-checklist", "content": content})
			if err != nil {
				return err
			}
			_, ids, err := svc.AppendAssistantStep(ctx, got.ID, "", nil, []cometsdk.ToolCallBlock{{
				ID: "w1", Name: "write_skill", Input: raw,
			}}, nil)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if _, err := svc.AppendToolResultMessage(ctx, got.ID, id, "ok", false); err != nil {
					return err
				}
			}
			return nil
		},
	}

	runner.reviewSkillsAfterTurn(ctx, session.AgentTurnFromSession(parent))

	if !hasTools(seen, "read_file", "load_skill", "write_skill") {
		t.Fatalf("surface = %#v", seen)
	}
	if _, err := svc.GetSession(ctx, child.ID); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("child still exists: %v", err)
	}
	updated, err := svc.GetSession(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SkillReviewStartedAt != now.UnixMilli() {
		t.Fatalf("cooldown = %d", updated.SkillReviewStartedAt)
	}
	select {
	case ev := <-sub.Events:
		if ev.Kind != event.KindSkillReviewUpdated || len(ev.SkillReviews) != 1 || ev.SkillReviews[0].Name != "ship-checklist" {
			t.Fatalf("event = %+v", ev)
		}
		if ev.Kind == event.KindSubagentStarted || ev.Kind == event.KindSubagentFinished {
			t.Fatal("delegation event published")
		}
	case <-time.After(time.Second):
		t.Fatal("missing skill_review_updated")
	}

	starts := 0
	runner.ReviewChild = func(context.Context, session.Session, *tools.Registry, int, string) error {
		starts++
		return nil
	}
	reset, err := svc.GetSession(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reset.SkillReviewMutatingCount != 0 {
		t.Fatalf("counter = %d, want 0", reset.SkillReviewMutatingCount)
	}
}

func TestSkillReviewUsesDefaultModelWhenExtractionUnpinned(t *testing.T) {
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
	var gotModel, gotProvider string
	runner := &Runner{
		Config:   &config.Config{DefaultProviderID: "chat-provider", DefaultModelID: "chat-model"},
		Sessions: svc,
		ReviewChild: func(_ context.Context, child session.Session, _ *tools.Registry, _ int, _ string) error {
			gotModel = child.ModelID
			gotProvider = child.ProviderID
			return nil
		},
	}
	runner.reviewSkillsAfterTurn(ctx, session.AgentTurnFromSession(parent))
	if gotProvider != "chat-provider" || gotModel != "chat-model" {
		t.Fatalf("review model = %s/%s, want default chat-provider/chat-model", gotProvider, gotModel)
	}
}

func TestSkillReviewSkipsWithoutAnyModel(t *testing.T) {
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
	starts := 0
	runner := &Runner{
		Config:   &config.Config{},
		Sessions: svc,
		ReviewChild: func(context.Context, session.Session, *tools.Registry, int, string) error {
			starts++
			return nil
		},
	}
	runner.reviewSkillsAfterTurn(ctx, session.AgentTurnFromSession(parent))
	if starts != 0 {
		t.Fatalf("review started without a model, starts = %d", starts)
	}
	updated, err := svc.GetSession(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SkillReviewStartedAt != 0 {
		t.Fatalf("cooldown consumed without a start: %d", updated.SkillReviewStartedAt)
	}
	if updated.SkillReviewMutatingCount != 10 {
		t.Fatalf("counter = %d, want 10", updated.SkillReviewMutatingCount)
	}
}

func TestSkillReviewCounterStartsOnceAcrossTurns(t *testing.T) {
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
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	var prompts []string
	starts := 0
	runner := &Runner{
		Config: &config.Config{Memory: config.MemoryConfig{
			ExtractionProvider: "extract-provider",
			ExtractionModel:    "extract-model",
		}},
		Sessions: svc,
		ReviewNow: func() time.Time {
			return now
		},
		ReviewChild: func(_ context.Context, child session.Session, _ *tools.Registry, _ int, _ string) error {
			starts++
			rows, err := svc.ListMessageRows(ctx, child.ID)
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.Role == "user" {
					prompts = append(prompts, row.Content)
				}
			}
			return nil
		},
	}
	for i := 1; i <= 10; i++ {
		if err := seedNamedTurn(ctx, svc, parent.ID, fmt.Sprintf("turn-%d", i), 1); err != nil {
			t.Fatal(err)
		}
		runner.reviewSkillsAfterTurn(ctx, session.AgentTurnFromSession(parent))
	}
	if starts != 1 {
		t.Fatalf("starts = %d, want 1", starts)
	}
	reset, err := svc.GetSession(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reset.SkillReviewMutatingCount != 0 {
		t.Fatalf("counter = %d, want 0", reset.SkillReviewMutatingCount)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "turn-1") || !strings.Contains(prompts[0], "turn-10") {
		t.Fatalf("window = %#v", prompts)
	}
	updated, err := svc.GetSession(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SkillReviewMutatingCount != 0 {
		t.Fatalf("counter = %d, want 0", updated.SkillReviewMutatingCount)
	}
}

func TestForkSessionStartsSkillReviewCounterAtZero(t *testing.T) {
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
	if err := svc.SetSkillReviewCounter(ctx, parent.ID, 9, 1); err != nil {
		t.Fatal(err)
	}
	forked, err := svc.ForkSession(ctx, parent.ID, ws.Path)
	if err != nil {
		t.Fatal(err)
	}
	if forked.SkillReviewMutatingCount != 0 {
		t.Fatalf("forked counter = %d", forked.SkillReviewMutatingCount)
	}
	if forked.SkillReviewCountResetAt == 0 {
		t.Fatal("forked window was not reset")
	}
}

func reviewSessionService(t *testing.T) *session.Service {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return session.New(conn)
}

func seedMutatingTurn(ctx context.Context, svc *session.Service, sessionID string, n int) error {
	return seedNamedTurn(ctx, svc, sessionID, "ship the change", n)
}

func seedNamedTurn(ctx context.Context, svc *session.Service, sessionID, text string, n int) error {
	if _, err := svc.AppendUserMessage(ctx, sessionID, text); err != nil {
		return err
	}
	calls := make([]cometsdk.ToolCallBlock, n)
	for i := range calls {
		calls[i] = cometsdk.ToolCallBlock{
			ID:    fmt.Sprintf("c%d", i),
			Name:  "edit_file",
			Input: json.RawMessage(`{"path":"src/main.go"}`),
		}
	}
	_, ids, err := svc.AppendAssistantStep(ctx, sessionID, "edited", nil, calls, nil)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := svc.AppendToolResultMessage(ctx, sessionID, id, "ok", false); err != nil {
			return err
		}
	}
	return nil
}

func hasTools(got []string, want ...string) bool {
	set := map[string]bool{}
	for _, name := range got {
		set[name] = true
	}
	for _, name := range want {
		if !set[name] {
			return false
		}
	}
	return true
}
