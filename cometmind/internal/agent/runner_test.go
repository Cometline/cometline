package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/subagent"
	"github.com/cometline/cometmind/internal/tools"
)

// fakeStore is an in-memory TurnStore. It records the persistence calls the
// runner makes so the agent loop can be exercised without a live database.
type fakeStore struct {
	history          []cometsdk.Message
	allHistory       []cometsdk.Message
	rows             []db.Message
	contextSummary   string
	compactedUntil   string
	usageSaved       int
	appendCalls      int
	appendTexts      []string
	appendReasoning  [][]cometsdk.Block
	toolUpdates      int
	toolResults      int
	compactCalls     int
	inflateAfterTool bool
}

func (f *fakeStore) BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	return f.history, nil
}

func (f *fakeStore) BuildSDKMessagesAll(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	if f.allHistory != nil {
		return f.allHistory, nil
	}
	return f.history, nil
}

func (f *fakeStore) SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage) error {
	f.usageSaved++
	return nil
}

func (f *fakeStore) AppendAssistantStep(ctx context.Context, sessionID, text string, reasoningBlocks []cometsdk.Block, toolCalls []cometsdk.ToolCallBlock, injectedMemories []session.InjectedMemory) (session.Message, map[string]string, error) {
	f.appendCalls++
	f.appendTexts = append(f.appendTexts, text)
	f.appendReasoning = append(f.appendReasoning, reasoningBlocks)
	ids := make(map[string]string, len(toolCalls))
	for _, tc := range toolCalls {
		ids[tc.ID] = "persisted-" + tc.ID
	}
	return session.Message{}, ids, nil
}

func (f *fakeStore) UpdateToolCallResult(ctx context.Context, toolCallID, result string, durMs int64, exit *int64) error {
	f.toolUpdates++
	return nil
}

func (f *fakeStore) AppendToolResultMessage(ctx context.Context, sessionID, toolCallID, output string, isErr bool) (session.Message, error) {
	f.toolResults++
	if f.inflateAfterTool {
		inflated := strings.Repeat("tool output ", 70000)
		f.allHistory = append(f.allHistory, cometsdk.Message{
			Role: cometsdk.RoleUser,
			Content: []cometsdk.Block{
				cometsdk.TextBlock{Text: inflated},
			},
		})
		f.rows = append(f.rows, db.Message{ID: "tool-result-big", Role: "tool_result", Content: output})
	}
	return session.Message{}, nil
}

func (f *fakeStore) GetSession(ctx context.Context, sessionID string) (session.Session, error) {
	return session.Session{ID: sessionID, ContextSummary: f.contextSummary, CompactedUntilMessageID: f.compactedUntil}, nil
}

func (f *fakeStore) ListMessageRows(ctx context.Context, sessionID string) ([]db.Message, error) {
	return f.rows, nil
}

func (f *fakeStore) UpdateContextSummary(ctx context.Context, sessionID, summary, untilMessageID string) error {
	f.compactCalls++
	f.contextSummary = summary
	f.compactedUntil = untilMessageID
	return nil
}

func (f *fakeStore) NewChildSession(ctx context.Context, parent session.Session, purpose, subagentKind string) (session.Session, error) {
	return session.Session{}, nil
}

func (f *fakeStore) UpdateSessionModel(ctx context.Context, sessionID, modelID, providerID string) (session.Session, error) {
	return session.Session{}, nil
}

func (f *fakeStore) AppendUserMessage(ctx context.Context, sessionID, text string) (session.Message, error) {
	return session.Message{}, nil
}

func (f *fakeStore) UpdateDelegationState(ctx context.Context, sessionID string, status session.DelegationStatus, summary, pendingQuestion string) error {
	return nil
}

func (f *fakeStore) UpdateACPSessionID(ctx context.Context, sessionID, acpSessionID string) error {
	return nil
}

func (f *fakeStore) CompactChildSession(ctx context.Context, childID string) error {
	return nil
}

func (f *fakeStore) LastAssistantText(ctx context.Context, sessionID string) (string, error) {
	return "", nil
}

func (f *fakeStore) ListToolCallsForSession(ctx context.Context, sessionID string) ([]db.ToolCall, error) {
	return nil, nil
}

// fakeProvider streams a fixed sequence of SDK events for one Stream call.
type fakeProvider struct {
	events []cometsdk.Event
}

func (p *fakeProvider) ID() string { return "fake" }

func (p *fakeProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	ch := make(chan cometsdk.Event, len(p.events))
	for _, ev := range p.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

// sequentialFakeProvider returns a different event sequence on each Stream call.
type sequentialFakeProvider struct {
	sequences [][]cometsdk.Event
	calls     int
}

func (p *sequentialFakeProvider) ID() string { return "fake-seq" }

func (p *sequentialFakeProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	idx := p.calls
	p.calls++
	if idx >= len(p.sequences) {
		idx = len(p.sequences) - 1
	}
	events := p.sequences[idx]
	ch := make(chan cometsdk.Event, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

type fakeMemory struct {
	retrieveCalls  int
	baselineCalls  int
	outcomeCalls   int
	extractCalls   int
	waitForCancel  bool
	preferences    []memory.ScoredMemory
	outcomes       []memory.ScoredMemory
	extractChanges []memory.Change
}

func (m *fakeMemory) Enabled() bool { return true }

func (m *fakeMemory) BaselinePreferences(ctx context.Context, limit int) ([]memory.ScoredMemory, error) {
	m.baselineCalls++
	return m.preferences, nil
}

func (m *fakeMemory) RecentTaskOutcomes(ctx context.Context, limit int) ([]memory.ScoredMemory, error) {
	m.outcomeCalls++
	return m.outcomes, nil
}

func (m *fakeMemory) RetrieveForTurn(ctx context.Context, query string) ([]memory.ScoredMemory, error) {
	m.retrieveCalls++
	if m.waitForCancel {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return nil, nil
}

func (m *fakeMemory) ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]memory.Change, error) {
	m.extractCalls++
	return m.extractChanges, nil
}

// drain collects events the runner emits until the channel closes.
func drain(ch <-chan event.Event) []event.Event {
	var out []event.Event
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

func runAndDrainWithContext(t *testing.T, ctx context.Context, r *Runner, turn session.AgentTurn) ([]event.Event, error) {
	t.Helper()
	ch := make(chan event.Event, 64)
	var runErr error
	go func() {
		runErr = r.Run(ctx, turn, ch)
		close(ch)
	}()
	return drain(ch), runErr
}

func runAndDrain(t *testing.T, r *Runner, turn session.AgentTurn) ([]event.Event, error) {
	return runAndDrainWithContext(t, context.Background(), r, turn)
}

func TestRunner_TextOnlyTurnPersistsAndStops(t *testing.T) {
	store := &fakeStore{}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "hello"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: cometsdk.TokenUsage{InputTokens: 3, OutputTokens: 1}},
		cometsdk.DoneEvent{},
	}}

	r := &Runner{
		Provider: provider,
		Sessions: store,
		Registry: tools.NewRegistry(t.TempDir()),
	}

	events, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if store.usageSaved != 1 {
		t.Errorf("SaveTokenUsage called %d times, want 1", store.usageSaved)
	}
	if store.appendCalls != 1 {
		t.Errorf("AppendAssistantStep called %d times, want 1", store.appendCalls)
	}
	if store.toolResults != 0 {
		t.Errorf("AppendToolResultMessage called %d times, want 0", store.toolResults)
	}

	// The runner forwards a text delta and always closes with a done event.
	if len(events) == 0 || events[len(events)-1].Kind != event.KindDone {
		t.Fatalf("expected final event to be done, got %+v", events)
	}
	var sawText bool
	for _, ev := range events {
		if ev.Kind == event.KindTextDelta && ev.Delta == "hello" {
			sawText = true
		}
	}
	if !sawText {
		t.Errorf("expected a text_delta 'hello' event, got %+v", events)
	}
}

func TestRunner_PersistsPartialResponseBeforeStreamError(t *testing.T) {
	store := &fakeStore{}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.ReasoningStartEvent{},
		cometsdk.ReasoningContentEvent{Text: "thinking"},
		cometsdk.TextDeltaEvent{Text: "partial reply"},
		cometsdk.ErrorEvent{Err: fmt.Errorf("provider disconnected")},
	}}

	events, runErr := runAndDrain(t, &Runner{
		Provider: provider,
		Sessions: store,
		Registry: tools.NewRegistry(t.TempDir()),
	}, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr == nil {
		t.Fatal("Run returned nil error, want stream failure")
	}
	if store.appendCalls != 1 {
		t.Fatalf("AppendAssistantStep called %d times, want 1", store.appendCalls)
	}
	if got := store.appendTexts[0]; got != "partial reply" {
		t.Fatalf("persisted partial text = %q, want %q", got, "partial reply")
	}
	if len(store.appendReasoning[0]) != 1 {
		t.Fatalf("persisted reasoning blocks = %d, want 1", len(store.appendReasoning[0]))
	}
	if len(events) < 2 || events[len(events)-1].Kind != event.KindDone {
		t.Fatalf("expected error and terminal done events, got %+v", events)
	}
	if events[len(events)-2].Kind != event.KindError {
		t.Fatalf("expected error before done, got %+v", events)
	}
}

func TestRunner_ExtractsMemoryInBackgroundAfterDone(t *testing.T) {
	store := &fakeStore{}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "noted"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}
	mem := &fakeMemory{
		extractChanges: []memory.Change{{
			Action:  "create",
			Kind:    "preference",
			Content: "loves zhajiangmian",
			ID:      "mem-1",
		}},
	}

	r := &Runner{
		Provider: provider,
		Sessions: store,
		Memory:   mem,
		Registry: tools.NewRegistry(t.TempDir()),
	}

	events, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if len(events) == 0 || events[len(events)-1].Kind != event.KindDone {
		t.Fatalf("expected stream to end with done, got %+v", events)
	}
	for _, ev := range events {
		if ev.Kind == event.KindMemoryUpdated {
			t.Fatalf("memory_updated should not be emitted on the turn stream, got %+v", events)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for mem.extractCalls == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if mem.extractCalls != 1 {
		t.Fatalf("ExtractAfterTurn called %d times, want 1", mem.extractCalls)
	}
}

func TestBackgroundProgressEmitterIgnoresClosedChannel(t *testing.T) {
	ch := make(chan event.Event, 1)
	emit := backgroundProgressEmitter(ch)
	close(ch)

	emit(event.SubagentProgress("child-1", "tool", "grep"))
}

func TestBackgroundProgressEmitterForwardsWhenChannelOpen(t *testing.T) {
	ch := make(chan event.Event, 1)
	emit := backgroundProgressEmitter(ch)

	emit(event.SubagentProgress("child-1", "tool", "grep"))
	ev := <-ch
	if ev.Kind != event.KindSubagentProgress {
		t.Fatalf("event kind = %q, want %q", ev.Kind, event.KindSubagentProgress)
	}
	if ev.ProgressKind != "tool" || ev.ProgressText != "grep" {
		t.Fatalf("progress = (%q, %q), want (%q, %q)", ev.ProgressKind, ev.ProgressText, "tool", "grep")
	}
}

func TestRunner_MaxTokensWithoutToolsContinuesThenStops(t *testing.T) {
	store := &fakeStore{}
	provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{
		{
			cometsdk.TextDeltaEvent{Text: "part one "},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishMaxTokens, Usage: cometsdk.TokenUsage{InputTokens: 3, OutputTokens: 4096}},
			cometsdk.DoneEvent{},
		},
		{
			cometsdk.TextDeltaEvent{Text: "part two"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop, Usage: cometsdk.TokenUsage{InputTokens: 4, OutputTokens: 2}},
			cometsdk.DoneEvent{},
		},
	}}

	r := &Runner{
		Provider:  provider,
		Sessions:  store,
		Registry:  tools.NewRegistry(t.TempDir()),
		MaxTokens: 4096,
	}

	events, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if provider.calls != 2 {
		t.Fatalf("Stream called %d times, want 2", provider.calls)
	}
	if store.appendCalls != 2 {
		t.Fatalf("AppendAssistantStep called %d times, want 2", store.appendCalls)
	}

	var text strings.Builder
	for _, ev := range events {
		if ev.Kind == event.KindTextDelta {
			text.WriteString(ev.Delta)
		}
	}
	if got := text.String(); got != "part one part two" {
		t.Fatalf("text deltas = %q, want %q", got, "part one part two")
	}
}

func TestRunner_MaxTokensWithoutToolsStopsAfterContinuationCap(t *testing.T) {
	store := &fakeStore{}
	provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{
		{
			cometsdk.TextDeltaEvent{Text: "a"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishMaxTokens},
			cometsdk.DoneEvent{},
		},
		{
			cometsdk.TextDeltaEvent{Text: "b"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishMaxTokens},
			cometsdk.DoneEvent{},
		},
		{
			cometsdk.TextDeltaEvent{Text: "c"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishMaxTokens},
			cometsdk.DoneEvent{},
		},
	}}

	r := &Runner{
		Provider: provider,
		Sessions: store,
		Registry: tools.NewRegistry(t.TempDir()),
	}

	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	// Initial truncated step + 2 continuation attempts, then stop.
	if provider.calls != 3 {
		t.Fatalf("Stream called %d times, want 3", provider.calls)
	}
	if store.appendCalls != 3 {
		t.Fatalf("AppendAssistantStep called %d times, want 3", store.appendCalls)
	}
}

func TestRunner_CompactsAgainAfterToolResultsInflatePrompt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "small.txt"), []byte("small result"), 0o644); err != nil {
		t.Fatalf("write small.txt: %v", err)
	}

	rows := make([]db.Message, 0, 11)
	allHistory := make([]cometsdk.Message, 0, 11)
	for i := 0; i < 11; i++ {
		id := fmt.Sprintf("u%d", i+1)
		text := fmt.Sprintf("small prior user turn %d", i+1)
		rows = append(rows, db.Message{ID: id, Role: "user", Content: text})
		allHistory = append(allHistory, cometsdk.Message{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: text}},
		})
	}
	store := &fakeStore{
		history:          allHistory,
		allHistory:       allHistory,
		rows:             rows,
		inflateAfterTool: true,
	}
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{
		toolStep("tc1", "read_file", `{"path":"small.txt"}`),
		{
			cometsdk.TextDeltaEvent{Text: "summary after tool"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
		{
			cometsdk.TextDeltaEvent{Text: "final answer"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
	}}

	r := &Runner{
		Provider:  provider,
		Sessions:  store,
		Registry:  tools.NewRegistry(dir),
		Compactor: &ContextCompactor{Sessions: store},
	}

	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if store.compactCalls != 1 {
		t.Fatalf("UpdateContextSummary called %d times, want 1", store.compactCalls)
	}
	if provider.calls != 3 {
		t.Fatalf("Stream called %d times, want 3", provider.calls)
	}
	if len(provider.requests) != 3 {
		t.Fatalf("captured %d requests, want 3", len(provider.requests))
	}
	if strings.Contains(provider.requests[0].System, "summary after tool") {
		t.Fatalf("step 1 unexpectedly included post-tool summary:\n%s", provider.requests[0].System)
	}
	if !strings.Contains(provider.requests[2].System, "Earlier conversation summary") ||
		!strings.Contains(provider.requests[2].System, "summary after tool") {
		t.Fatalf("final synthesis request missing compacted summary:\n%s", provider.requests[2].System)
	}
}

func TestRunner_SkipsMemoryRetrievalForLowValueTurn(t *testing.T) {
	store := &fakeStore{history: []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "hihi"}},
	}}}
	mem := &fakeMemory{}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "hi"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}

	r := &Runner{Provider: provider, Sessions: store, Memory: mem, Registry: tools.NewRegistry(t.TempDir())}
	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.retrieveCalls != 0 {
		t.Fatalf("RetrieveForTurn called %d times, want 0", mem.retrieveCalls)
	}
	if mem.baselineCalls != 0 {
		t.Fatalf("BaselinePreferences called %d times, want 0", mem.baselineCalls)
	}
	if mem.outcomeCalls != 0 {
		t.Fatalf("RecentTaskOutcomes called %d times, want 0", mem.outcomeCalls)
	}
}

func TestRunner_RetrievesMemoryForSubstantiveTurn(t *testing.T) {
	store := &fakeStore{history: []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "remember my preferred model"}},
	}}}
	mem := &fakeMemory{}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "ok"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}

	r := &Runner{Provider: provider, Sessions: store, Memory: mem, Registry: tools.NewRegistry(t.TempDir())}
	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.retrieveCalls != 1 {
		t.Fatalf("RetrieveForTurn called %d times, want 1", mem.retrieveCalls)
	}
	if mem.baselineCalls != 1 {
		t.Fatalf("BaselinePreferences called %d times, want 1", mem.baselineCalls)
	}
	if mem.outcomeCalls != 1 {
		t.Fatalf("RecentTaskOutcomes called %d times, want 1", mem.outcomeCalls)
	}
}

func TestRunner_MemoryRetrievalTimeoutDoesNotEmitError(t *testing.T) {
	store := &fakeStore{history: []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "remember my preferred model"}},
	}}}
	mem := &fakeMemory{waitForCancel: true}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "ok"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}

	r := &Runner{
		Provider:               provider,
		Sessions:               store,
		Memory:                 mem,
		Registry:               tools.NewRegistry(t.TempDir()),
		MemoryRetrievalTimeout: 10 * time.Millisecond,
	}
	events, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.retrieveCalls != 1 {
		t.Fatalf("RetrieveForTurn called %d times, want 1", mem.retrieveCalls)
	}
	if mem.baselineCalls != 1 {
		t.Fatalf("BaselinePreferences called %d times, want 1", mem.baselineCalls)
	}
	if mem.outcomeCalls != 1 {
		t.Fatalf("RecentTaskOutcomes called %d times, want 1", mem.outcomeCalls)
	}
	for _, ev := range events {
		if ev.Kind == event.KindError {
			t.Fatalf("timeout should not emit error event: %+v", ev)
		}
	}
}

func TestRunner_InjectsPreferencesWhenSemanticRetrievalTimesOut(t *testing.T) {
	store := &fakeStore{history: []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "help me implement this"}},
	}}}
	mem := &fakeMemory{
		waitForCancel: true,
		preferences: []memory.ScoredMemory{{Record: memory.Record{
			ID: "pref1", Kind: "preference", Content: "User prefers Traditional Chinese replies.",
		}}},
	}
	provider := &fakeProvider{events: []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "ok"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}

	r := &Runner{
		Provider:               provider,
		Sessions:               store,
		Memory:                 mem,
		Registry:               tools.NewRegistry(t.TempDir()),
		MemoryRetrievalTimeout: 10 * time.Millisecond,
	}
	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.baselineCalls != 1 || mem.outcomeCalls != 1 || mem.retrieveCalls != 1 {
		t.Fatalf("baseline=%d outcomes=%d retrieve=%d, want 1/1/1", mem.baselineCalls, mem.outcomeCalls, mem.retrieveCalls)
	}
}

func TestRunner_InjectsRecentTaskOutcomes(t *testing.T) {
	store := &fakeStore{history: []cometsdk.Message{{
		Role:    cometsdk.RoleUser,
		Content: []cometsdk.Block{cometsdk.TextBlock{Text: "what have you worked on recently?"}},
	}}}
	mem := &fakeMemory{outcomes: []memory.ScoredMemory{{Record: memory.Record{
		ID: "task-1", Kind: "task_outcome", Content: "Completed the autonomous jobs retry policy.",
	}}}}
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{{
		cometsdk.TextDeltaEvent{Text: "ok"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}}}

	r := &Runner{Provider: provider, Sessions: store, Memory: mem, Registry: tools.NewRegistry(t.TempDir())}
	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.outcomeCalls != 1 {
		t.Fatalf("RecentTaskOutcomes called %d times, want 1", mem.outcomeCalls)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("captured %d requests, want 1", len(provider.requests))
	}
	system := provider.requests[0].System
	if !strings.Contains(system, "## Recent task outcomes") || !strings.Contains(system, "Completed the autonomous jobs retry policy.") {
		t.Fatalf("system prompt missing task outcome recall: %q", system)
	}
}

type fakeOngoingJobLookup struct {
	job jobs.Job
	ok  bool
}

func (f *fakeOngoingJobLookup) JobForSession(ctx context.Context, sessionID string) (jobs.Job, bool, error) {
	return f.job, f.ok, nil
}

// capturingSequentialFakeProvider records each outbound LLM request.
type capturingSequentialFakeProvider struct {
	sequences [][]cometsdk.Event
	requests  []*cometsdk.Request
	calls     int
}

func (p *capturingSequentialFakeProvider) ID() string { return "fake-capture" }

func (p *capturingSequentialFakeProvider) Stream(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	p.requests = append(p.requests, req)
	idx := p.calls
	p.calls++
	if idx >= len(p.sequences) {
		idx = len(p.sequences) - 1
	}
	events := p.sequences[idx]
	ch := make(chan cometsdk.Event, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func toolStep(toolID, name, input string) []cometsdk.Event {
	return []cometsdk.Event{
		cometsdk.ToolCallDoneEvent{ID: toolID, Name: name, Input: json.RawMessage(input)},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishToolUse},
		cometsdk.DoneEvent{},
	}
}

func TestRunner_JobProgressNudgeInjectedAfterTools(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("world"), 0o644); err != nil {
		t.Fatalf("write hello.txt: %v", err)
	}

	// After the progress nudge step, the job completion gate refuses two more
	// text-only stops before allowing the turn to end.
	textStop := []cometsdk.Event{
		cometsdk.TextDeltaEvent{Text: "done"},
		cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
		cometsdk.DoneEvent{},
	}
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{
		toolStep("tc1", "read_file", `{"path":"hello.txt"}`),
		toolStep("tc2", "read_file", `{"path":"hello.txt"}`),
		toolStep("tc3", "read_file", `{"path":"hello.txt"}`),
		textStop,
		textStop,
		textStop,
	}}

	r := &Runner{
		Provider: provider,
		Sessions: &fakeStore{},
		Registry: tools.NewRegistry(dir),
		Jobs: &fakeOngoingJobLookup{
			ok:  true,
			job: jobs.Job{ID: "job-1", Status: jobs.StatusOngoing},
		},
	}

	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	// 3 tool steps + 1 progress-nudged stop + 2 completion-gate steps
	if provider.calls != 6 {
		t.Fatalf("Stream called %d times, want 6", provider.calls)
	}
	if len(provider.requests) < 6 {
		t.Fatalf("captured %d requests, want 6", len(provider.requests))
	}

	nudge := FormatJobProgressNudgeBlock("job-1")
	if !strings.Contains(provider.requests[3].System, nudge) {
		t.Fatalf("step 4 system missing nudge block:\n%s", provider.requests[3].System)
	}
	for i := 0; i < 3; i++ {
		if strings.Contains(provider.requests[i].System, nudge) {
			t.Fatalf("step %d system should not include nudge yet", i+1)
		}
	}
	gate := FormatJobCompletionGateBlock("job-1")
	if !strings.Contains(provider.requests[4].System, gate) {
		t.Fatalf("step 5 system missing completion gate:\n%s", provider.requests[4].System)
	}
	if !strings.Contains(provider.requests[5].System, gate) {
		t.Fatalf("step 6 system missing completion gate:\n%s", provider.requests[5].System)
	}
}

func TestRunner_JobCompletionGateForcesTerminalToolPath(t *testing.T) {
	dir := t.TempDir()
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{
		{
			cometsdk.TextDeltaEvent{Text: "I finished the work."},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
		{
			cometsdk.TextDeltaEvent{Text: "still done"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
		{
			cometsdk.TextDeltaEvent{Text: "really done"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
	}}

	r := &Runner{
		Provider: provider,
		Sessions: &fakeStore{},
		Registry: tools.NewRegistry(dir),
		Jobs: &fakeOngoingJobLookup{
			ok:  true,
			job: jobs.Job{ID: "job-gate", Status: jobs.StatusOngoing},
		},
		MaxSteps: 10,
	}

	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s-gate", ModelID: "m"})
	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	// First stop is gated twice, third stop ends the turn.
	if provider.calls != 3 {
		t.Fatalf("Stream called %d times, want 3", provider.calls)
	}
	gate := FormatJobCompletionGateBlock("job-gate")
	if strings.Contains(provider.requests[0].System, gate) {
		t.Fatal("first step should not include completion gate")
	}
	if !strings.Contains(provider.requests[1].System, gate) {
		t.Fatalf("second step missing gate:\n%s", provider.requests[1].System)
	}
	if !strings.Contains(provider.requests[2].System, gate) {
		t.Fatalf("third step missing gate:\n%s", provider.requests[2].System)
	}
}

func TestRunner_JobCompletionGateSkippedWithoutOngoingJob(t *testing.T) {
	dir := t.TempDir()
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{
		{
			cometsdk.TextDeltaEvent{Text: "hello"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
	}}

	r := &Runner{
		Provider: provider,
		Sessions: &fakeStore{},
		Registry: tools.NewRegistry(dir),
		Jobs:     &fakeOngoingJobLookup{ok: false},
	}

	_, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s-plain", ModelID: "m"})
	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if provider.calls != 1 {
		t.Fatalf("Stream called %d times, want 1 (no gate without job)", provider.calls)
	}
}

func TestRunner_SubagentWaitNudgeInjectedWhileChildrenActive(t *testing.T) {
	dir := t.TempDir()
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{
		toolStep("tc1", "spawn_general_agent", `{"task":"say hello"}`),
		{
			cometsdk.TextDeltaEvent{Text: "done"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
	}}

	orch := subagent.NewOrchestrator(5)
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := orch.Register("s1", "child-1", subagent.KindGeneral, cancel); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r := &Runner{
		Provider:             provider,
		Sessions:             &fakeStore{},
		Registry:             tools.NewRegistry(dir),
		SubagentOrchestrator: orch,
		MaxSteps:             2,
	}

	_, runErr := runAndDrainWithContext(t, ctx, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr == nil {
		t.Fatal("expected run to stop when wait context times out while subagent is still active")
	}
	if provider.calls != 2 {
		t.Fatalf("Stream called %d times, want 2", provider.calls)
	}
	if len(provider.requests) < 2 {
		t.Fatalf("captured %d requests, want 2", len(provider.requests))
	}

	waitBlock := FormatWaitForSubagentsBlock()
	if strings.Contains(provider.requests[0].System, waitBlock) {
		t.Fatalf("step 1 system should not include wait block:\n%s", provider.requests[0].System)
	}
	if !strings.Contains(provider.requests[1].System, waitBlock) {
		t.Fatalf("step 2 system missing wait block:\n%s", provider.requests[1].System)
	}
	if !strings.Contains(runErr.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("runErr = %v, want %v", runErr, context.DeadlineExceeded)
	}
	if got := orch.ActiveCount("s1"); got != 1 {
		t.Fatalf("ActiveCount() = %d, want 1", got)
	}
}

func TestRunner_AutoCollectsActiveSubagentResultsBeforeFinishing(t *testing.T) {
	provider := &capturingSequentialFakeProvider{sequences: [][]cometsdk.Event{
		toolStep("tc1", "spawn_general_agent", `{"task":"say hello"}`),
		{
			cometsdk.TextDeltaEvent{Text: "final synthesis"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
	}}

	orch := subagent.NewOrchestrator(5)
	ctx := context.Background()
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := orch.Register("s1", "child-1", subagent.KindGeneral, cancel); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		orch.Complete("child-1", subagent.Result{
			ChildSessionID: "child-1",
			Kind:           subagent.KindGeneral,
			Status:         "completed",
			Summary:        "said hello",
		})
		_ = childCtx
	}()

	r := &Runner{
		Provider:             provider,
		Sessions:             &fakeStore{},
		Registry:             tools.NewRegistry(t.TempDir()),
		SubagentOrchestrator: orch,
		MaxSteps:             3,
	}

	events, runErr := runAndDrain(t, r, session.AgentTurn{ID: "s1", ModelID: "m"})

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if provider.calls != 3 {
		t.Fatalf("Stream called %d times, want 3", provider.calls)
	}
	if len(provider.requests) < 3 {
		t.Fatalf("captured %d requests, want 3", len(provider.requests))
	}

	collectedBlock := FormatCollectedSubagentResultsBlock("child_session_id: child-1\nkind: general\nstatus: completed\n\nsaid hello")
	if !strings.Contains(provider.requests[2].System, collectedBlock) {
		t.Fatalf("step 3 system missing collected subagent results:\n%s", provider.requests[2].System)
	}
	if got := orch.ActiveCount("s1"); got != 0 {
		t.Fatalf("ActiveCount() = %d, want 0", got)
	}
	if len(events) == 0 || events[len(events)-1].Kind != event.KindDone {
		t.Fatalf("expected final event to be done, got %+v", events)
	}
}

func TestUserFacingAgentError(t *testing.T) {
	t.Parallel()
	if got := userFacingAgentError(context.Canceled); !strings.Contains(got, "interrupted") {
		t.Fatalf("canceled = %q", got)
	}
	if got := userFacingAgentError(fmt.Errorf("openai: context canceled")); !strings.Contains(got, "interrupted") {
		t.Fatalf("wrapped canceled = %q", got)
	}
	if got := userFacingAgentError(context.DeadlineExceeded); !strings.Contains(got, "timed out") {
		t.Fatalf("deadline = %q", got)
	}
	if got := userFacingAgentError(fmt.Errorf("provider exploded")); got != "provider exploded" {
		t.Fatalf("passthrough = %q", got)
	}
}

func TestRunner_RecoversPartialStreamOnceWithoutPersistingFirstAttempt(t *testing.T) {
	provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{
		{
			cometsdk.TextDeltaEvent{Text: "partial"},
			cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: "fake", Cause: io.ErrUnexpectedEOF}},
		},
		{
			cometsdk.TextDeltaEvent{Text: "complete"},
			cometsdk.StepFinishEvent{FinishReason: cometsdk.FinishStop},
			cometsdk.DoneEvent{},
		},
	}}
	store := &fakeStore{}
	r := &Runner{Provider: provider, Sessions: store, Registry: tools.NewRegistry(t.TempDir())}

	events, err := runAndDrain(t, r, session.AgentTurn{ID: "s-recover", ModelID: "m"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("Stream called %d times, want 2", provider.calls)
	}
	if store.appendCalls != 1 || len(store.appendTexts) != 1 || store.appendTexts[0] != "complete" {
		t.Fatalf("assistant persistence = calls=%d texts=%q, want one complete result", store.appendCalls, store.appendTexts)
	}
	var recoverEvent *event.Event
	for i := range events {
		if events[i].Kind == event.KindTurnRecover {
			recoverEvent = &events[i]
		}
	}
	if recoverEvent == nil || recoverEvent.TextChars != len("partial") || recoverEvent.ReasoningChars != 0 {
		t.Fatalf("turn_recover = %+v, want partial text reset", recoverEvent)
	}
}

func TestRunner_StopsAfterSecondRecoverableStreamFailureAndPersistsPartial(t *testing.T) {
	provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{
		{
			cometsdk.TextDeltaEvent{Text: "first"},
			cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: "fake", Cause: io.ErrUnexpectedEOF}},
		},
		{
			cometsdk.TextDeltaEvent{Text: "second"},
			cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: "fake", Cause: io.ErrUnexpectedEOF}},
		},
	}}
	store := &fakeStore{}
	r := &Runner{Provider: provider, Sessions: store, Registry: tools.NewRegistry(t.TempDir())}

	_, err := runAndDrain(t, r, session.AgentTurn{ID: "s-double-fail", ModelID: "m"})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if provider.calls != 2 {
		t.Fatalf("Stream called %d times, want 2", provider.calls)
	}
	if store.appendCalls != 1 || store.appendTexts[0] != "second" {
		t.Fatalf("partial persistence = calls=%d texts=%q, want second attempt only", store.appendCalls, store.appendTexts)
	}
}

func TestRunner_DoesNotRecoverAfterCompleteToolCall(t *testing.T) {
	provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{{
		cometsdk.ToolCallDoneEvent{ID: "tc-1", Name: "list_dir", Input: json.RawMessage(`{"path":"."}`)},
		cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: "fake", Cause: io.ErrUnexpectedEOF}},
	}}}
	r := &Runner{Provider: provider, Sessions: &fakeStore{}, Registry: tools.NewRegistry(t.TempDir())}

	_, err := runAndDrain(t, r, session.AgentTurn{ID: "s-tool-fail", ModelID: "m"})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if provider.calls != 1 {
		t.Fatalf("Stream called %d times, want 1 after complete tool call", provider.calls)
	}
}

func TestRunner_DoesNotRecoverTerminalFailures(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"cancelled", &cometsdk.StreamError{ProviderID: "fake", Cause: context.Canceled}},
		{"auth", &cometsdk.AuthError{ProviderID: "fake", StatusCode: 401}},
		{"bad request", &cometsdk.ServerError{ProviderID: "fake", StatusCode: 400}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{{
				cometsdk.ErrorEvent{Err: tc.err},
			}}}
			r := &Runner{Provider: provider, Sessions: &fakeStore{}, Registry: tools.NewRegistry(t.TempDir())}
			if _, err := runAndDrain(t, r, session.AgentTurn{ID: "s-terminal", ModelID: "m"}); err == nil {
				t.Fatal("Run returned nil error")
			}
			if provider.calls != 1 {
				t.Fatalf("Stream called %d times, want 1", provider.calls)
			}
		})
	}
}

func TestRunner_UserCancelledContextFinishesWithoutError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &sequentialFakeProvider{sequences: [][]cometsdk.Event{{
		cometsdk.TextDeltaEvent{Text: "partial"},
		cometsdk.ErrorEvent{Err: &cometsdk.StreamError{ProviderID: "fake", Cause: context.Canceled}},
	}}}
	store := &fakeStore{}
	r := &Runner{Provider: provider, Sessions: store, Registry: tools.NewRegistry(t.TempDir())}

	events, err := runAndDrainWithContext(t, ctx, r, session.AgentTurn{ID: "s-stopped", ModelID: "m"})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, ev := range events {
		if ev.Kind == event.KindError {
			t.Fatalf("unexpected error event: %+v", ev)
		}
	}
}
