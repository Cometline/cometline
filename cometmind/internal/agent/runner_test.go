package agent

import (
	"context"
	"testing"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

// fakeStore is an in-memory TurnStore. It records the persistence calls the
// runner makes so the agent loop can be exercised without a live database.
type fakeStore struct {
	history     []cometsdk.Message
	usageSaved  int
	appendCalls int
	toolUpdates int
	toolResults int
}

func (f *fakeStore) BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error) {
	return f.history, nil
}

func (f *fakeStore) SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage) error {
	f.usageSaved++
	return nil
}

func (f *fakeStore) AppendAssistantStep(ctx context.Context, sessionID, text string, reasoningBlocks []cometsdk.Block, toolCalls []cometsdk.ToolCallBlock) (session.Message, map[string]string, error) {
	f.appendCalls++
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
	return session.Message{}, nil
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

type fakeMemory struct {
	retrieveCalls int
	baselineCalls int
	waitForCancel bool
	preferences   []memory.ScoredMemory
}

func (m *fakeMemory) Enabled() bool { return true }

func (m *fakeMemory) BaselinePreferences(ctx context.Context, limit int) ([]memory.ScoredMemory, error) {
	m.baselineCalls++
	return m.preferences, nil
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
	return nil, nil
}

// drain collects events the runner emits until it sends its terminal done
// event. The runner sends done but does not close the channel, so we stop there.
func drain(ch <-chan event.Event) []event.Event {
	var out []event.Event
	for ev := range ch {
		out = append(out, ev)
		if ev.Kind == event.KindDone {
			break
		}
	}
	return out
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

	ch := make(chan event.Event, 16)
	var runErr error
	go func() {
		runErr = r.Run(context.Background(), session.AgentTurn{ID: "s1", ModelID: "m"}, ch)
	}()
	events := drain(ch)

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
		if ev.Kind == event.KindMemoryUpdated {
			t.Fatalf("memory_updated should not be streamed; got %+v", ev)
		}
	}
	if !sawText {
		t.Errorf("expected a text_delta 'hello' event, got %+v", events)
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
	ch := make(chan event.Event, 16)
	var runErr error
	go func() { runErr = r.Run(context.Background(), session.AgentTurn{ID: "s1", ModelID: "m"}, ch) }()
	_ = drain(ch)

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.retrieveCalls != 0 {
		t.Fatalf("RetrieveForTurn called %d times, want 0", mem.retrieveCalls)
	}
	if mem.baselineCalls != 0 {
		t.Fatalf("BaselinePreferences called %d times, want 0", mem.baselineCalls)
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
	ch := make(chan event.Event, 16)
	var runErr error
	go func() { runErr = r.Run(context.Background(), session.AgentTurn{ID: "s1", ModelID: "m"}, ch) }()
	_ = drain(ch)

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.retrieveCalls != 1 {
		t.Fatalf("RetrieveForTurn called %d times, want 1", mem.retrieveCalls)
	}
	if mem.baselineCalls != 1 {
		t.Fatalf("BaselinePreferences called %d times, want 1", mem.baselineCalls)
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
	ch := make(chan event.Event, 16)
	var runErr error
	go func() { runErr = r.Run(context.Background(), session.AgentTurn{ID: "s1", ModelID: "m"}, ch) }()
	events := drain(ch)

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.retrieveCalls != 1 {
		t.Fatalf("RetrieveForTurn called %d times, want 1", mem.retrieveCalls)
	}
	if mem.baselineCalls != 1 {
		t.Fatalf("BaselinePreferences called %d times, want 1", mem.baselineCalls)
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
	ch := make(chan event.Event, 16)
	var runErr error
	go func() { runErr = r.Run(context.Background(), session.AgentTurn{ID: "s1", ModelID: "m"}, ch) }()
	_ = drain(ch)

	if runErr != nil {
		t.Fatalf("Run returned error: %v", runErr)
	}
	if mem.baselineCalls != 1 || mem.retrieveCalls != 1 {
		t.Fatalf("baseline=%d retrieve=%d, want 1/1", mem.baselineCalls, mem.retrieveCalls)
	}
}
