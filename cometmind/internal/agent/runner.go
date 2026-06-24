package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

const memoryRetrievalTimeout = 3 * time.Second

// TurnStore is the narrow persistence seam the agent loop drives. It is the
// subset of session.Service the Runner actually needs, declared here on the
// consumer side so the loop can be unit-tested with an in-memory fake instead
// of a live SQLite database. *session.Service satisfies it.
type TurnStore interface {
	BuildSDKMessages(ctx context.Context, sessionID string) ([]cometsdk.Message, error)
	SaveTokenUsage(ctx context.Context, sessionID string, u cometsdk.TokenUsage) error
	AppendAssistantStep(ctx context.Context, sessionID, text string, reasoningBlocks []cometsdk.Block, toolCalls []cometsdk.ToolCallBlock, injectedMemories []session.InjectedMemory) (session.Message, map[string]string, error)
	UpdateToolCallResult(ctx context.Context, toolCallID, result string, durMs int64, exit *int64) error
	AppendToolResultMessage(ctx context.Context, sessionID, toolCallID, output string, isErr bool) (session.Message, error)
}

type MemoryStore interface {
	Enabled() bool
	BaselinePreferences(ctx context.Context, limit int) ([]memory.ScoredMemory, error)
	RetrieveForTurn(ctx context.Context, query string) ([]memory.ScoredMemory, error)
	ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]memory.Change, error)
}

// Runner executes the persisted agent loop for one user turn (which may span many tool steps).
type Runner struct {
	Config   *config.Config
	Provider cometsdk.Provider
	Sessions TurnStore
	Memory   MemoryStore
	Registry *tools.Registry
	Jobs     OngoingJobLookup

	MaxSteps               int
	MaxTokens              int
	MemoryRetrievalTimeout time.Duration
	SystemPrompt           string
	SkillIndex             string
	JobIndex               string

	// MemorySem is an optional semaphore that bounds the number of
	// extractMemoryAfterTurn calls that may run concurrently across all
	// sessions. When non-nil, each background goroutine acquires one slot
	// before starting and releases it on completion. A nil value means
	// unlimited (the previous behaviour).
	MemorySem chan struct{}

	// Compactor performs rolling context compaction on long sessions. Nil disables it.
	Compactor *ContextCompactor
}

// Run streams CometMind-native events on ch until the turn completes or ctx is cancelled.
// The caller must receive until the channel closes.
func (r *Runner) Run(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) error {
	doneSent := false
	sendDone := func() {
		if doneSent {
			return
		}
		ch <- event.Done()
		doneSent = true
	}
	defer sendDone()

	completeTurn := func() error {
		sendDone()
		// Extraction runs after done so the UI can settle the turn immediately,
		// then memory_updated arrives on the same SSE connection before it closes.
		r.extractMemoryAfterTurn(context.WithoutCancel(ctx), turn, ch)
		return nil
	}

	if r.MaxSteps <= 0 {
		r.MaxSteps = 50
	}
	if r.MaxTokens <= 0 {
		r.MaxTokens = 2048
	}
	retrievalTimeout := r.MemoryRetrievalTimeout
	if retrievalTimeout <= 0 {
		retrievalTimeout = memoryRetrievalTimeout
	}

	steps := 0
	outputTruncationContinuations := 0
	truncationContinue := false
	jobProgressNudge := false
	jobTracker := newJobProgressTracker(ctx, r.Jobs, turn.ID)
	// Injected memories belong to the first assistant message of the turn. They
	// are captured when retrieved (step 0) and attached to the first
	// AppendAssistantStep call so they persist and rebuild on reload.
	var pendingMemories []session.InjectedMemory
	var sess session.Session
	if svc, ok := r.Sessions.(*session.Service); ok {
		if loaded, err := svc.GetSession(ctx, turn.ID); err == nil {
			sess = loaded
		}
	}
	emitStatus := func(phase event.TurnPhase) {
		ch <- event.TurnStatus(phase, "")
	}

	// Resolve the target provider family once so session history (which may
	// have been produced by a different provider) can be normalized before
	// replay. Switching, say, an Anthropic session to Codex must not feed raw
	// chain-of-thought the Codex adapter would otherwise drop.
	providerFamily := ""
	if r.Config != nil {
		providerID := turn.ProviderID
		if providerID == "" {
			providerID = r.Config.Provider
		}
		providerFamily = provider.SDKFamily(r.Config, providerID)
	}
	degradationsReported := false

	for steps < r.MaxSteps {
		if steps > 0 {
			emitStatus(event.PhaseContinuing)
		}

		baseSystem := r.systemPrompt()
		if steps == 0 && r.Compactor != nil && sess.ID != "" {
			updated, err := r.Compactor.MaybeCompact(
				ctx,
				sess,
				baseSystem,
				r.Registry.CometSDK(),
				r.Provider,
				r.MaxTokens,
				func(ev event.Event) { ch <- ev },
			)
			if err == nil {
				sess = updated
			}
		}

		msgs, err := r.Sessions.BuildSDKMessages(ctx, turn.ID)
		if err != nil {
			ch <- event.Errorf(err.Error(), "history")
			return err
		}

		// Adapt cross-provider history (e.g. reasoning that the target provider
		// cannot replay) and report any lossy degradations once per turn.
		if providerFamily != "" {
			normalized, degradations := NormalizeHistoryForProvider(providerFamily, msgs)
			msgs = normalized
			if !degradationsReported {
				for _, d := range degradations {
					logging.L().Info("history.normalized", "session", turn.ID, "provider", providerFamily, "kind", d.Kind, "count", d.Count)
				}
				degradationsReported = true
			}
		}

		logging.L().Info("agent.step.start", "session", turn.ID, "step", steps+1, "model", turn.ModelID, "messages", len(msgs), "max_tokens", r.MaxTokens)

		system := r.buildSystemPrompt(sess.ContextSummary, truncationContinue, jobProgressNudge, jobTracker.JobID)
		truncationContinue = false
		jobProgressNudge = false
		if r.Memory != nil && r.Memory.Enabled() && steps == 0 {
			decision := memory.DecideRetrieval(msgs)
			logging.L().Info("memory.retrieve.policy", "session", turn.ID, "retrieve", decision.Retrieve, "reason", decision.Reason, "score", decision.Score, "text_bytes", decision.TextBytes)
			if !decision.Retrieve {
				logging.L().Info("memory.retrieve.skipped", "session", turn.ID, "reason", decision.Reason, "score", decision.Score, "text_bytes", decision.TextBytes)
			} else {
				emitStatus(event.PhaseRetrievingMemories)
				prefs, prefErr := r.Memory.BaselinePreferences(ctx, 3)
				if prefErr != nil {
					logging.L().Error("memory.preferences.failed", "session", turn.ID, "error", prefErr)
				}
				query := memory.BuildRetrievalQuery(memory.RetrievalQueryInput{
					Messages: msgs,
				})
				retrieveCtx, cancel := context.WithTimeout(ctx, retrievalTimeout)
				mems, memErr := r.Memory.RetrieveForTurn(retrieveCtx, query)
				cancel()
				if memErr != nil {
					if errors.Is(memErr, context.DeadlineExceeded) {
						logging.L().Warn("memory.retrieve.timeout", "session", turn.ID, "budget_ms", retrievalTimeout.Milliseconds(), "using_preferences", len(prefs) > 0)
					} else {
						logging.L().Error("memory.retrieve.failed", "session", turn.ID, "error", memErr)
						ch <- event.Errorf(memErr.Error(), "memory")
					}
				}
				if len(prefs) > 0 || len(mems) > 0 {
					logging.L().Info("memory.injected", "session", turn.ID, "preferences", len(prefs), "relevant", len(mems))
					system += memory.FormatPromptMemories(memory.PromptMemories{Preferences: prefs, Relevant: mems})
					// Only relevant (semantic) memories are surfaced to the UI as a
					// memory card. Preferences are injected into the prompt silently,
					// so skip the wire event when there is nothing relevant to show.
					if len(mems) > 0 {
						wire := make([]event.MemoryWire, len(mems))
						pendingMemories = make([]session.InjectedMemory, len(mems))
						for i, m := range mems {
							wire[i] = event.MemoryWire{
								ID:              m.ID,
								Content:         m.Content,
								Kind:            m.Kind,
								Similarity:      m.Similarity,
								EffectiveWeight: m.EffectiveWeight,
							}
							pendingMemories[i] = session.InjectedMemory{
								ID:              m.ID,
								Content:         m.Content,
								Kind:            m.Kind,
								Similarity:      m.Similarity,
								EffectiveWeight: m.EffectiveWeight,
							}
						}
						ch <- event.MemoryInjected(wire)
					}
				}
			}
		}

		emitStatus(event.PhaseContactingModel)
		req := BuildRequest(turn.ModelID, system, msgs, r.Registry.CometSDK(), r.MaxTokens)
		streamStarted := time.Now()
		logging.L().Info("llm.stream.start", "session", turn.ID, "step", steps+1, "model", turn.ModelID, "messages", len(req.Messages), "tools", len(req.Tools), "system_bytes", len(req.System), "max_tokens", req.MaxTokens)
		stream := llm.StreamMessage(ctx, r.Provider, req)
		logging.L().Info("llm.stream.opened", "session", turn.ID, "step", steps+1, "duration_ms", time.Since(streamStarted).Milliseconds())
		emitStatus(event.PhaseComposingResponse)

		firstEventLogged := false
		firstOutputLogged := false
		eventCount := 0
		for ev := range stream.Events() {
			eventCount++
			if !firstEventLogged {
				firstEventLogged = true
				logging.L().Info("llm.stream.first_event", "session", turn.ID, "step", steps+1, "event_type", fmt.Sprintf("%T", ev), "duration_ms", time.Since(streamStarted).Milliseconds())
			}
			switch e := ev.(type) {
			case cometsdk.TextDeltaEvent:
				if !firstOutputLogged {
					firstOutputLogged = true
					logging.L().Info("llm.stream.first_output", "session", turn.ID, "step", steps+1, "event_type", fmt.Sprintf("%T", ev), "duration_ms", time.Since(streamStarted).Milliseconds())
				}
				ch <- event.TextDelta(e.Text)
			case cometsdk.ReasoningStartEvent:
				ch <- event.ReasoningStart()
			case cometsdk.ReasoningContentEvent:
				if !firstOutputLogged {
					firstOutputLogged = true
					logging.L().Info("llm.stream.first_output", "session", turn.ID, "step", steps+1, "event_type", fmt.Sprintf("%T", ev), "duration_ms", time.Since(streamStarted).Milliseconds())
				}
				ch <- event.ReasoningDelta(e.Text)
			case cometsdk.ToolCallDoneEvent:
				if !firstOutputLogged {
					firstOutputLogged = true
					logging.L().Info("llm.stream.first_output", "session", turn.ID, "step", steps+1, "event_type", fmt.Sprintf("%T", ev), "duration_ms", time.Since(streamStarted).Milliseconds())
				}
				ch <- event.ToolCall(e.ID, e.Name, []byte(e.Input))
			case cometsdk.StepFinishEvent:
				ch <- event.StepFinish(e.Usage)
			}
		}
		logging.L().Info("llm.stream.events_closed", "session", turn.ID, "step", steps+1, "events", eventCount, "saw_first_output", firstOutputLogged, "duration_ms", time.Since(streamStarted).Milliseconds())

		result, err := stream.Result()
		if err != nil {
			logging.L().Error("agent.step.failed", "session", turn.ID, "step", steps+1, "events", eventCount, "saw_first_event", firstEventLogged, "saw_first_output", firstOutputLogged, "duration_ms", time.Since(streamStarted).Milliseconds(), "error", err)
			ch <- event.Errorf(err.Error(), "llm")
			return err
		}
		logging.L().Info("agent.step.finish", "session", turn.ID, "step", steps+1, "finish_reason", string(result.FinishReason), "tool_calls", len(result.ToolCalls), "input_tokens", result.Usage.InputTokens, "output_tokens", result.Usage.OutputTokens, "events", eventCount, "duration_ms", time.Since(streamStarted).Milliseconds())

		if err := r.Sessions.SaveTokenUsage(ctx, turn.ID, result.Usage); err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return err
		}

		text := assistantPlainText(result.Message)
		reasoningBlocks := result.Message.ReasoningContent
		_, persistedToolIDs, err := r.Sessions.AppendAssistantStep(ctx, turn.ID, text, reasoningBlocks, result.ToolCalls, pendingMemories)
		if err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return err
		}
		// Memories are attached to the first persisted assistant message only.
		pendingMemories = nil

		if result.FinishReason == cometsdk.FinishStop {
			return completeTurn()
		}
		if len(result.ToolCalls) == 0 {
			if result.FinishReason == cometsdk.FinishMaxTokens &&
				outputTruncationContinuations < maxOutputTruncationContinuations {
				outputTruncationContinuations++
				truncationContinue = true
				logging.L().Info(
					"agent.output_truncated.continue",
					"session", turn.ID,
					"step", steps+1,
					"continuation", outputTruncationContinuations,
					"max_tokens", r.MaxTokens,
				)
				steps++
				continue
			}
			return completeTurn()
		}

		emitStatus(event.PhaseRunningTools)
		for _, tc := range result.ToolCalls {
			persistedID := persistedToolIDs[tc.ID]
			if persistedID == "" {
				ch <- event.Errorf("missing persisted tool call id", "db")
				return fmt.Errorf("missing persisted tool call id for %s", tc.ID)
			}
			start := time.Now()
			logging.L().Info("tool.call.start", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "input_bytes", len(tc.Input))
			toolCtx := tools.WithToolSession(ctx, turn.ID)
			toolCtx = tools.WithProgress(toolCtx, func(ev event.Event) {
				ch <- ev
			})
			res, execErr := r.Registry.Execute(toolCtx, tc.Name, tc.Input)
			dur := time.Since(start).Milliseconds()
			logging.L().Info("tool.call.finish", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "ok", res.OK && execErr == nil, "duration_ms", dur, "output_bytes", len(res.Output))

			out := res.Output
			isErr := !res.OK
			if execErr != nil {
				isErr = true
				out = fmt.Sprintf("%s\n(execute error: %v)", out, execErr)
			}

			exit := int64PtrFromIntPtr(res.ExitCode)
			if err := r.Sessions.UpdateToolCallResult(ctx, persistedID, out, dur, exit); err != nil {
				ch <- event.Errorf(err.Error(), "db")
				return err
			}
			if _, err := r.Sessions.AppendToolResultMessage(ctx, turn.ID, persistedID, out, isErr); err != nil {
				ch <- event.Errorf(err.Error(), "db")
				return err
			}

			toolErr := ""
			if isErr {
				toolErr = out
			}
			ch <- event.ToolResult(tc.ID, tc.Name, out, toolErr)

			if jobTracker.ObserveTool(tc.Name, tc.Input) {
				jobProgressNudge = true
			}
		}

		steps++
	}

	ch <- event.Errorf("max steps exceeded", "max_steps")
	return fmt.Errorf("max steps exceeded")
}

func memoryChangesToWire(changes []memory.Change) []event.MemoryChangeWire {
	if len(changes) == 0 {
		return nil
	}
	wire := make([]event.MemoryChangeWire, 0, len(changes))
	for _, change := range changes {
		wire = append(wire, event.MemoryChangeWire{
			Action:  change.Action,
			Kind:    change.Kind,
			Content: change.Content,
			ID:      change.ID,
		})
	}
	return wire
}

func (r *Runner) extractMemoryAfterTurn(ctx context.Context, turn session.AgentTurn, ch chan<- event.Event) {
	if r.Memory == nil || !r.Memory.Enabled() {
		return
	}
	// Honour the optional concurrency cap: acquire a slot before doing any
	// work and release it when done. This prevents N simultaneous session
	// completions from spawning N unbounded LLM API calls and SQLite writes.
	if r.MemorySem != nil {
		select {
		case r.MemorySem <- struct{}{}:
			defer func() { <-r.MemorySem }()
		case <-ctx.Done():
			return
		}
	}
	providerID, model := turn.ProviderID, turn.ModelID
	llmProvider := r.Provider
	if r.Config != nil {
		providerID, model = r.Config.ExtractionLLMForSession(turn.ProviderID, turn.ModelID)
		if providerID != turn.ProviderID {
			if p, err := provider.NewFor(r.Config, providerID); err == nil {
				llmProvider = p
			} else {
				logging.L().Warn("memory.extract.provider_failed", "session", turn.ID, "provider", providerID, "error", err)
			}
		}
	}
	changes, err := r.Memory.ExtractAfterTurn(ctx, turn.ID, model, llmProvider)
	if err != nil {
		logging.L().Warn("memory.extract.after_turn_failed", "session", turn.ID, "provider", providerID, "error", err)
		return
	}
	if wire := memoryChangesToWire(changes); len(wire) > 0 {
		ch <- event.MemoryUpdated(wire)
	}
}

func (r *Runner) systemPrompt() string {
	base := strings.TrimSpace(r.SystemPrompt)
	if base == "" {
		base = DefaultSystemPrompt
	}
	if strings.TrimSpace(r.SkillIndex) == "" && strings.TrimSpace(r.JobIndex) == "" {
		return base
	}
	return base + r.SkillIndex + r.JobIndex
}

func (r *Runner) buildSystemPrompt(contextSummary string, truncationContinue, jobProgressNudge bool, jobID string) string {
	base := r.systemPrompt()
	var parts []string
	if block := FormatSummaryPromptBlock(contextSummary); block != "" {
		parts = append(parts, block)
	}
	if block := FormatOutputBudgetPromptBlock(r.MaxTokens); block != "" {
		parts = append(parts, block)
	}
	if truncationContinue {
		parts = append(parts, FormatOutputTruncationContinueBlock())
	}
	if jobProgressNudge {
		if block := FormatJobProgressNudgeBlock(jobID); block != "" {
			parts = append(parts, block)
		}
	}
	if len(parts) == 0 {
		return base
	}
	return base + "\n\n" + strings.Join(parts, "\n\n")
}

func int64PtrFromIntPtr(v *int) *int64 {
	if v == nil {
		return nil
	}
	x := int64(*v)
	return &x
}

func assistantPlainText(m cometsdk.Message) string {
	var b strings.Builder
	for _, bl := range m.Content {
		if tb, ok := bl.(cometsdk.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return b.String()
}
