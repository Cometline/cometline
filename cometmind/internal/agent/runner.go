package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/subagent"
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

type providerStateStore interface {
	SaveAssistantProviderState(ctx context.Context, messageID string, states []cometsdk.ProviderState) error
}

type MemoryStore interface {
	Enabled() bool
	BaselinePreferences(ctx context.Context, limit int) ([]memory.ScoredMemory, error)
	RecentTaskOutcomes(ctx context.Context, limit int) ([]memory.ScoredMemory, error)
	RetrieveForTurn(ctx context.Context, query string) ([]memory.ScoredMemory, error)
	ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]memory.Change, error)
}

type sessionLoader interface {
	GetSession(ctx context.Context, sessionID string) (session.Session, error)
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
	SubagentOrchestrator   *subagent.Orchestrator

	// MemorySem is an optional semaphore that bounds the number of
	// extractMemoryAfterTurn calls that may run concurrently across all
	// sessions. When non-nil, each background goroutine acquires one slot
	// before starting and releases it on completion. A nil value means
	// unlimited (the previous behaviour).
	MemorySem          chan struct{}
	Compatibility      cometsdk.CapabilityResolver
	CompatibilityScope cometsdk.CapabilityScope

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
		// Extraction runs in the background so the SSE stream can close on done
		// and the next queued message can start without waiting on the extractor.
		go r.extractMemoryAfterTurn(context.WithoutCancel(ctx), turn, nil)
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
	jobCompletionGate := false
	jobTracker := newJobProgressTracker(ctx, r.Jobs, turn.ID)
	subagentWaitNudge := false
	pendingSubagentResults := ""
	// Injected memories belong to the first assistant message of the turn. They
	// are captured when retrieved (step 0) and attached to the first
	// AppendAssistantStep call so they persist and rebuild on reload.
	var pendingMemories []session.InjectedMemory
	var sess session.Session
	if svc, ok := r.Sessions.(sessionLoader); ok {
		if loaded, err := svc.GetSession(ctx, turn.ID); err == nil {
			sess = loaded
		}
	}
	if sess.ID == "" {
		sess.ID = turn.ID
	}
	emitStatus := func(phase event.TurnPhase) {
		ch <- event.TurnStatus(phase, "")
	}

	degradationsReported := false

	for steps < r.MaxSteps {
		if steps > 0 {
			emitStatus(event.PhaseContinuing)
		}

		baseSystem := r.buildSystemPrompt(sess.ContextSummary)
		if r.Compactor != nil && sess.ID != "" {
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
				baseSystem = r.buildSystemPrompt(sess.ContextSummary)
			}
		}

		msgs, err := r.Sessions.BuildSDKMessages(ctx, turn.ID)
		if err != nil {
			ch <- event.Errorf(err.Error(), "history")
			return err
		}

		// Normalize replay history and report any lossy degradations once per turn.
		normalized, degradations := NormalizeHistory(msgs)
		msgs = normalized
		// Continue nudges are in-memory user turns so providers that reject
		// trailing assistant prefills (Claude 4.6+) still accept the request.
		msgs = append(msgs, ContinueUserNudgeMessages(
			truncationContinue,
			jobProgressNudge,
			jobCompletionGate,
			jobTracker.JobID,
			subagentWaitNudge,
			pendingSubagentResults,
		)...)
		if !degradationsReported {
			for _, d := range degradations {
				logging.L().Info("history.normalized", "session", turn.ID, "kind", d.Kind, "count", d.Count)
			}
			degradationsReported = true
		}

		logging.L().Info("agent.step.start", "session", turn.ID, "step", steps+1, "model", turn.ModelID, "messages", len(msgs), "max_tokens", r.MaxTokens)

		system := baseSystem
		truncationContinue = false
		jobProgressNudge = false
		jobCompletionGate = false
		subagentWaitNudge = false
		pendingSubagentResults = ""
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
				outcomes, outcomeErr := r.Memory.RecentTaskOutcomes(ctx, 0)
				if outcomeErr != nil {
					logging.L().Error("memory.task_outcomes.failed", "session", turn.ID, "error", outcomeErr)
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
				if len(prefs) > 0 || len(outcomes) > 0 || len(mems) > 0 {
					logging.L().Info("memory.injected", "session", turn.ID, "preferences", len(prefs), "task_outcomes", len(outcomes), "relevant", len(mems))
					system += memory.FormatPromptMemories(memory.PromptMemories{Preferences: prefs, TaskOutcomes: outcomes, Relevant: mems})
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
		if r.Compatibility != nil {
			req.Compatibility = r.Compatibility.ResolveCapabilityPolicy(ctx, r.CompatibilityScope)
		}
		toolOutputBytes := toolResultBytes(req.Messages)
		var result *llm.GenerateMessageResult
		recoveryAttempt := 0
		for {
			streamStarted := time.Now()
			logging.L().Info("llm.stream.start", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "messages", len(req.Messages), "tools", len(req.Tools), "tool_output_bytes", toolOutputBytes, "recovery_attempt", recoveryAttempt, "system_bytes", len(req.System), "max_tokens", req.MaxTokens)
			stream := llm.StreamMessage(ctx, r.Provider, req)
			logging.L().Info("llm.stream.opened", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds())
			emitStatus(event.PhaseComposingResponse)

			firstEventLogged := false
			firstOutputLogged := false
			completeToolCall := false
			eventCount := 0
			for ev := range stream.Events() {
				eventCount++
				if !firstEventLogged {
					firstEventLogged = true
					logging.L().Info("llm.stream.first_event", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "recovery_attempt", recoveryAttempt, "event_type", fmt.Sprintf("%T", ev), "duration_ms", time.Since(streamStarted).Milliseconds())
				}
				switch e := ev.(type) {
				case cometsdk.TextDeltaEvent:
					firstOutputLogged = true
					ch <- event.TextDelta(e.Text)
				case cometsdk.ReasoningStartEvent:
					ch <- event.ReasoningStart()
				case cometsdk.ReasoningContentEvent:
					firstOutputLogged = true
					ch <- event.ReasoningDelta(e.Text)
				case cometsdk.ToolCallStartEvent:
					firstOutputLogged = true
					ch <- event.ToolCall(e.ID, e.Name, nil)
				case cometsdk.ToolCallDoneEvent:
					firstOutputLogged = true
					completeToolCall = true
					ch <- event.ToolCall(e.ID, e.Name, []byte(e.Input))
				case cometsdk.StepFinishEvent:
					ch <- event.StepFinish(e.Usage)
				}
			}
			result, err = stream.Result()
			failureCategory := classifyStreamFailure(err)
			logging.L().Info("llm.stream.events_closed", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "events", eventCount, "first_event", firstEventLogged, "first_output", firstOutputLogged, "complete_tool_call", completeToolCall, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds())
			if err == nil {
				break
			}
			// A cancelled runner context is the explicit /stop path (or a client
			// disconnect). It is normal control flow, not a provider failure:
			// retain any visible partial output, close with done, and do not add an
			// error transcript row or SSE error card.
			if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
				persistPartialStep(ctx, r.Sessions, turn.ID, turn.ProviderID, result, pendingMemories)
				logging.L().Info("agent.step.stopped", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "duration_ms", time.Since(streamStarted).Milliseconds())
				return nil
			}
			if recoveryAttempt == 0 && recoverableStreamFailure(err) && !completeToolCall {
				textChars, reasoningChars := partialRenderLengths(result)
				recoveryAttempt++
				logging.L().Warn("agent.step.recover", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "text_chars", textChars, "reasoning_chars", reasoningChars)
				ch <- event.TurnRecover(textChars, reasoningChars)
				continue
			}
			persistPartialStep(ctx, r.Sessions, turn.ID, turn.ProviderID, result, pendingMemories)
			logging.L().Error("agent.step.failed", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "events", eventCount, "first_event", firstEventLogged, "first_output", firstOutputLogged, "complete_tool_call", completeToolCall, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds(), "error", err)
			ch <- event.Errorf(userFacingAgentError(err), "llm")
			return err
		}
		logging.L().Info("agent.step.finish", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", steps+1, "finish_reason", string(result.FinishReason), "tool_calls", len(result.ToolCalls), "input_tokens", result.Usage.InputTokens, "output_tokens", result.Usage.OutputTokens, "recovery_attempt", recoveryAttempt)

		if err := r.Sessions.SaveTokenUsage(ctx, turn.ID, result.Usage); err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return err
		}

		// Whitespace-only content (common from some mini models on tool steps)
		// is treated as empty so we do not persist invisible assistant bubbles.
		text := strings.TrimSpace(assistantPlainText(result.Message))
		reasoningBlocks := result.Message.ReasoningContent
		persistedToolIDs := map[string]string{}
		if text != "" || len(reasoningBlocks) > 0 || len(result.Message.ProviderState) > 0 || len(result.ToolCalls) > 0 {
			assistant, toolIDs, err := r.Sessions.AppendAssistantStep(ctx, turn.ID, text, reasoningBlocks, result.ToolCalls, pendingMemories)
			if err != nil {
				ch <- event.Errorf(err.Error(), "db")
				return err
			}
			persistedToolIDs = toolIDs
			if states, ok := r.Sessions.(providerStateStore); ok {
				if err := states.SaveAssistantProviderState(ctx, assistant.ID, scopeProviderState(result.Message.ProviderState, turn.ProviderID)); err != nil {
					ch <- event.Errorf(err.Error(), "db")
					return err
				}
			}
		}
		// Guard against providers that terminate a step without yielding any
		// replayable assistant payload. Persisting an empty assistant row poisons
		// later provider switches because many APIs reject assistant history with
		// neither content nor tool calls.
		// Memories are attached to the first persisted assistant message only.
		pendingMemories = nil

		if result.FinishReason == cometsdk.FinishStop {
			if collected, waited, err := r.collectActiveSubagentResults(ctx, turn.ID); err != nil {
				ch <- event.Errorf(err.Error(), "subagents")
				return err
			} else if waited {
				pendingSubagentResults = collected
				subagentWaitNudge = false
				steps++
				continue
			}
			if jobTracker.TryConsumeCompletionGate() {
				jobCompletionGate = true
				logging.L().Info(
					"agent.job_completion_gate",
					"session", turn.ID,
					"job_id", jobTracker.JobID,
					"gate_used", jobTracker.completionGateUsed,
					"gate_budget", jobTracker.completionGateBudget,
				)
				steps++
				continue
			}
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
			if collected, waited, err := r.collectActiveSubagentResults(ctx, turn.ID); err != nil {
				ch <- event.Errorf(err.Error(), "subagents")
				return err
			} else if waited {
				pendingSubagentResults = collected
				subagentWaitNudge = false
				steps++
				continue
			}
			if jobTracker.TryConsumeCompletionGate() {
				jobCompletionGate = true
				logging.L().Info(
					"agent.job_completion_gate",
					"session", turn.ID,
					"job_id", jobTracker.JobID,
					"gate_used", jobTracker.completionGateUsed,
					"gate_budget", jobTracker.completionGateBudget,
				)
				steps++
				continue
			}
			return completeTurn()
		}

		emitStatus(event.PhaseRunningTools)
		for i, tc := range result.ToolCalls {
			persistedID := persistedToolIDs[tc.ID]
			if persistedID == "" {
				ch <- event.Errorf("missing persisted tool call id", "db")
				return fmt.Errorf("missing persisted tool call id for %s", tc.ID)
			}
			if ctx.Err() != nil {
				if err := persistCancelledToolResults(ctx, r.Sessions, turn.ID, result.ToolCalls[i:], persistedToolIDs); err != nil {
					ch <- event.Errorf(err.Error(), "db")
					return err
				}
				return nil
			}
			start := time.Now()
			logging.L().Info("tool.call.start", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "input_bytes", len(tc.Input))
			toolCtx := tools.WithToolSession(ctx, turn.ID)
			toolCtx = tools.WithProgress(toolCtx, backgroundProgressEmitter(ch))
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
			if err := persistToolResult(ctx, r.Sessions, turn.ID, persistedID, out, isErr, dur, exit); err != nil {
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
			if ctx.Err() != nil {
				if err := persistCancelledToolResults(ctx, r.Sessions, turn.ID, result.ToolCalls[i+1:], persistedToolIDs); err != nil {
					ch <- event.Errorf(err.Error(), "db")
					return err
				}
				return nil
			}
		}
		if r.hasActiveSubagents(turn.ID) {
			subagentWaitNudge = true
		} else {
			subagentWaitNudge = false
		}
		pendingSubagentResults = ""

		steps++
	}

	ch <- event.Errorf("max steps exceeded", "max_steps")
	return fmt.Errorf("max steps exceeded")
}

func (r *Runner) hasActiveSubagents(parentSessionID string) bool {
	return r.SubagentOrchestrator != nil && r.SubagentOrchestrator.ActiveCount(parentSessionID) > 0
}

func (r *Runner) collectActiveSubagentResults(ctx context.Context, parentSessionID string) (string, bool, error) {
	if !r.hasActiveSubagents(parentSessionID) {
		return "", false, nil
	}
	if r.SubagentOrchestrator == nil {
		return "", false, fmt.Errorf("subagent waiting is not configured")
	}

	results, err := r.SubagentOrchestrator.Wait(ctx, parentSessionID, nil)
	if err != nil {
		return "", false, err
	}
	var b strings.Builder
	for _, res := range results {
		writeCollectedSubagentResult(&b, res.ChildSessionID, string(res.Kind), res.Status, res.Summary)
	}
	return strings.TrimSpace(b.String()), true, nil
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
	if ch != nil {
		if wire := memoryChangesToWire(changes); len(wire) > 0 {
			ch <- event.MemoryUpdated(wire)
		}
	}
}

func (r *Runner) systemPrompt() string {
	base := strings.TrimSpace(r.SystemPrompt)
	if base == "" {
		base = DefaultSystemPrompt()
	}
	if strings.TrimSpace(r.SkillIndex) == "" && strings.TrimSpace(r.JobIndex) == "" {
		return base
	}
	return base + r.SkillIndex + r.JobIndex
}

func (r *Runner) buildSystemPrompt(contextSummary string) string {
	base := r.systemPrompt()
	var parts []string
	if block := FormatSummaryPromptBlock(contextSummary); block != "" {
		parts = append(parts, block)
	}
	if block := FormatOutputBudgetPromptBlock(r.MaxTokens); block != "" {
		parts = append(parts, block)
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

const cancelledToolResult = "Tool execution cancelled before completion."

func persistToolResult(ctx context.Context, store TurnStore, sessionID, toolCallID, output string, isErr bool, durationMS int64, exit *int64) error {
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer persistCancel()
	if err := store.UpdateToolCallResult(persistCtx, toolCallID, output, durationMS, exit); err != nil {
		return err
	}
	_, err := store.AppendToolResultMessage(persistCtx, sessionID, toolCallID, output, isErr)
	return err
}

func persistCancelledToolResults(ctx context.Context, store TurnStore, sessionID string, calls []cometsdk.ToolCallBlock, persistedToolIDs map[string]string) error {
	for _, tc := range calls {
		persistedID := persistedToolIDs[tc.ID]
		if persistedID == "" {
			return fmt.Errorf("missing persisted tool call id for %s", tc.ID)
		}
		if err := persistToolResult(ctx, store, sessionID, persistedID, cancelledToolResult, true, 0, nil); err != nil {
			return err
		}
	}
	return nil
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

func toolResultBytes(messages []cometsdk.Message) int {
	total := 0
	for _, m := range messages {
		for _, block := range m.Content {
			if result, ok := block.(cometsdk.ToolResultBlock); ok {
				total += len(result.Content)
			}
		}
	}
	return total
}

func partialRenderLengths(result *llm.GenerateMessageResult) (textChars, reasoningChars int) {
	if result == nil {
		return 0, 0
	}
	textChars = len(utf16.Encode([]rune(assistantPlainText(result.Message))))
	for _, block := range result.Message.ReasoningContent {
		switch value := block.(type) {
		case cometsdk.ReasoningBlock:
			reasoningChars += len(utf16.Encode([]rune(value.Text)))
		case cometsdk.TextBlock:
			reasoningChars += len(utf16.Encode([]rune(value.Text)))
		}
	}
	return textChars, reasoningChars
}

func persistPartialStep(ctx context.Context, store TurnStore, sessionID, providerID string, result *llm.GenerateMessageResult, memories []session.InjectedMemory) {
	if result == nil {
		return
	}
	partialText := strings.TrimSpace(assistantPlainText(result.Message))
	if partialText == "" && len(result.Message.ReasoningContent) == 0 && len(result.Message.ProviderState) == 0 {
		return
	}
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer persistCancel()
	assistant, _, err := store.AppendAssistantStep(persistCtx, sessionID, partialText, result.Message.ReasoningContent, nil, memories)
	if err != nil {
		logging.L().Warn("agent.partial_persist_failed", "session", sessionID, "error", err)
		return
	}
	if states, ok := store.(providerStateStore); ok {
		if err := states.SaveAssistantProviderState(persistCtx, assistant.ID, scopeProviderState(result.Message.ProviderState, providerID)); err != nil {
			logging.L().Warn("agent.partial_provider_state_persist_failed", "session", sessionID, "error", err)
		}
	}
}

func scopeProviderState(states []cometsdk.ProviderState, providerID string) []cometsdk.ProviderState {
	if len(states) == 0 {
		return nil
	}
	result := append([]cometsdk.ProviderState(nil), states...)
	for i := range result {
		result[i].ProviderID = providerID
	}
	return result
}

// userFacingAgentError maps provider/runtime errors into short messages safe
// to show in the chat transcript. Context cancel (client abort or disconnect)
// is the most common silent-empty-turn cause for mini models.
func userFacingAgentError(err error) string {
	if err == nil {
		return "The request failed."
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if errors.Is(err, context.DeadlineExceeded) {
			return "The model timed out before finishing. Send the message again to continue."
		}
		return "Response interrupted. Send the message again to continue."
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "The request failed."
	}
	// Some providers wrap cancel without satisfying errors.Is.
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "context canceled") || strings.Contains(lower, "context cancelled") {
		return "Response interrupted. Send the message again to continue."
	}
	if strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") {
		return "The model timed out before finishing. Send the message again to continue."
	}
	return msg
}

// backgroundProgressEmitter is used for tool callbacks that may outlive the
// current turn stream, such as background subagents. Once the caller has
// drained the turn and closed the channel, later progress is best-effort only.
func backgroundProgressEmitter(ch chan<- event.Event) tools.ProgressFn {
	return func(ev event.Event) {
		defer func() {
			_ = recover()
		}()
		ch <- ev
	}
}

func writeCollectedSubagentResult(b *strings.Builder, id, kind, status, summary string) {
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	fmt.Fprintf(b, "child_session_id: %s\nkind: %s\nstatus: %s\n\n%s", id, kind, status, summary)
}
