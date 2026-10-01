package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

// Runner executes the persisted agent loop for one user turn (which may span many tool steps).
type Runner struct {
	Config   *config.Config
	Provider cometsdk.Provider
	Sessions TurnStore
	Memory   MemoryStore
	Registry *tools.Registry
	Jobs     OngoingJobLookup

	MaxSteps               int
	MemoryRetrievalTimeout time.Duration
	// StreamRecoveryBackoff is the first wait between recoverable model-stream
	// retries. Later attempts double it up to eight seconds. Zero uses 2s.
	StreamRecoveryBackoff time.Duration
	SystemPrompt          string
	AgentMode             session.AgentMode
	SkillIndex            string
	JobIndex              string
	SubagentOrchestrator  *subagent.Orchestrator

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
	s := &turnState{turn: turn, ch: ch}
	defer s.sendDone()

	if r.MaxSteps <= 0 {
		r.MaxSteps = 100
	}
	r.initTurnState(ctx, s)
	sess := &s.sess
	jobTracker := s.jobTracker
	sessionBudget := s.budget
	effectiveMaxTokens := s.maxTokens
	retrievalTimeout := s.retrievalTimeout

	// MaxSteps limits work rounds. If they are exhausted, make one final
	// tool-free request so the user still receives a best-effort answer.
	for s.steps <= r.MaxSteps {
		finalizing := s.steps == r.MaxSteps || s.doomLoopHalt
		requestTools := r.Registry.CometSDK()
		if finalizing {
			requestTools = nil
		}
		if s.steps > 0 {
			s.emitStatus(event.PhaseContinuing)
		}

		baseSystem := r.buildSystemPrompt(sess.ContextSummary, effectiveMaxTokens)
		if r.Compactor != nil && sess.ID != "" {
			tools := requestTools
			emitBudget := func(compacted bool) {
				budget, err := r.Compactor.EstimatePromptBudget(
					ctx, sess.ID, baseSystem, tools, turn.ProviderID, turn.ModelID,
				)
				if err != nil {
					logging.L().Warn("context.budget.estimate_failed", "session", sess.ID, "error", err)
					return
				}
				ch <- event.ContextBudget(budget.Estimated, budget.Available, budget.ContextWindow, compacted)
			}
			emitBudget(false)
			beforeSummary := sess.ContextSummary
			beforeUntil := sess.CompactedUntilMessageID
			updated, err := r.Compactor.MaybeCompact(
				ctx,
				*sess,
				baseSystem,
				tools,
				r.Provider,
				turn.ProviderID,
				turn.ModelID,
				false,
				func(ev event.Event) { ch <- ev },
			)
			if err == nil {
				*sess = updated
				baseSystem = r.buildSystemPrompt(sess.ContextSummary, effectiveMaxTokens)
				if sess.ContextSummary != beforeSummary || sess.CompactedUntilMessageID != beforeUntil {
					emitBudget(true)
				}
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
		recentTools := toolFingerprintsSinceLastUser(normalized)
		// Continue nudges are in-memory user turns so providers that reject
		// trailing assistant prefills (Claude 4.6+) still accept the request.
		msgs = append(msgs, s.nudges.messages(jobTracker.JobID)...)
		if s.doomLoopHalt {
			msgs = append(msgs, DoomLoopStopMessages()...)
		} else if finalizing {
			msgs = append(msgs, FinalAnswerNudgeMessages()...)
		}
		if !s.degradationsReported {
			for _, d := range degradations {
				logging.L().Info("history.normalized", "session", turn.ID, "kind", d.Kind, "count", d.Count)
			}
			s.degradationsReported = true
		}

		logging.L().Info("agent.step.start", "session", turn.ID, "step", s.steps+1, "model", turn.ModelID, "messages", len(msgs), "max_tokens", effectiveMaxTokens, "context_window", sessionBudget.Context, "limit_source", sessionBudget.LimitSource)

		system := baseSystem
		memoryPromptSuffix := ""
		s.nudges = continueNudges{}
		if r.Memory != nil && r.Memory.Enabled() && s.steps == 0 {
			decision := memory.DecideRetrieval(msgs)
			logging.L().Info("memory.retrieve.policy", "session", turn.ID, "retrieve", decision.Retrieve, "reason", decision.Reason, "score", decision.Score, "text_bytes", decision.TextBytes)
			if !decision.Retrieve {
				logging.L().Info("memory.retrieve.skipped", "session", turn.ID, "reason", decision.Reason, "score", decision.Score, "text_bytes", decision.TextBytes)
			} else {
				s.emitStatus(event.PhaseRetrievingMemories)
				query := memory.BuildRetrievalQuery(memory.RetrievalQueryInput{
					Messages: msgs,
				})
				allowance := memoryTokenAllowance(sessionBudget, baseSystem, msgs, r.Registry.CometSDK())
				retrieveCtx, cancel := context.WithTimeout(ctx, retrievalTimeout)
				promptMemories, memErr := r.Memory.RetrieveForTurn(retrieveCtx, turn.ID, query, allowance)
				cancel()
				if memErr != nil {
					if errors.Is(memErr, context.DeadlineExceeded) {
						logging.L().Warn("memory.retrieve.timeout", "session", turn.ID, "budget_ms", retrievalTimeout.Milliseconds())
					} else {
						logging.L().Error("memory.retrieve.failed", "session", turn.ID, "error", memErr)
						ch <- event.Errorf(memErr.Error(), "memory")
					}
				}
				if len(promptMemories.Records) > 0 {
					logging.L().Info("memory.injected", "session", turn.ID, "preferences", promptMemories.Count(memory.BucketPreference), "task_outcomes", promptMemories.Count(memory.BucketTaskOutcome), "semantic", promptMemories.Count(memory.BucketSemantic), "token_allowance", allowance)
					memoryPromptSuffix = memory.FormatPromptMemories(promptMemories)
					system += memoryPromptSuffix
					if len(promptMemories.Records) > 0 {
						wire := make([]event.MemoryWire, len(promptMemories.Records))
						s.pendingMemories = make([]session.InjectedMemory, len(promptMemories.Records))
						for i, m := range promptMemories.Records {
							wire[i] = event.MemoryWire{
								ID:              m.ID,
								Content:         m.Content,
								Kind:            m.Kind,
								Bucket:          event.MemoryBucket(m.Bucket),
								Similarity:      m.Similarity,
								EffectiveWeight: m.EffectiveWeight,
							}
							s.pendingMemories[i] = session.InjectedMemory{
								ID:              m.ID,
								Content:         m.Content,
								Kind:            m.Kind,
								Bucket:          session.MemoryBucket(m.Bucket),
								Similarity:      m.Similarity,
								EffectiveWeight: m.EffectiveWeight,
							}
						}
						ch <- event.MemoryInjected(wire)
					}
				}
			}
		}

		s.emitStatus(event.PhaseContactingModel)
		req := r.buildTurnRequest(ctx, s, system, msgs, requestTools)
		toolOutputBytes := toolResultBytes(req.Messages)
		var result *llm.GenerateMessageResult
		recoveryAttempt := 0
		overflowRecovered := false
		var startedToolCalls []cometsdk.ToolCallBlock
		for {
			streamStarted := time.Now()
			logging.L().Info("llm.stream.start", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "messages", len(req.Messages), "tools", len(req.Tools), "tool_output_bytes", toolOutputBytes, "recovery_attempt", recoveryAttempt, "overflow_recovered", overflowRecovered, "system_bytes", len(req.System), "max_tokens", req.MaxTokens)
			stream := llm.StreamMessage(ctx, r.Provider, req)
			logging.L().Info("llm.stream.opened", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds())
			s.emitStatus(event.PhaseComposingResponse)

			firstEventLogged := false
			firstOutputLogged := false
			completeToolCall := false
			eventCount := 0
			startedToolCalls = nil
			startedToolIndex := map[string]int{}
			for ev := range stream.Events() {
				eventCount++
				if !firstEventLogged {
					firstEventLogged = true
					logging.L().Info("llm.stream.first_event", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "recovery_attempt", recoveryAttempt, "event_type", fmt.Sprintf("%T", ev), "duration_ms", time.Since(streamStarted).Milliseconds())
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
					if finalizing {
						logging.L().Warn("agent.final_answer.unexpected_tool_call", "session", turn.ID, "tool_call_id", e.ID, "tool", e.Name)
						continue
					}
					firstOutputLogged = true
					if _, ok := startedToolIndex[e.ID]; !ok {
						startedToolIndex[e.ID] = len(startedToolCalls)
						startedToolCalls = append(startedToolCalls, cometsdk.ToolCallBlock{
							ID:    e.ID,
							Name:  e.Name,
							Input: json.RawMessage(`{}`),
						})
					} else {
						startedToolCalls[startedToolIndex[e.ID]].Name = e.Name
					}
					ch <- event.ToolCall(e.ID, e.Name, nil)
				case cometsdk.ToolCallDoneEvent:
					if finalizing {
						logging.L().Warn("agent.final_answer.unexpected_tool_call", "session", turn.ID, "tool_call_id", e.ID, "tool", e.Name)
						continue
					}
					firstOutputLogged = true
					completeToolCall = true
					if idx, ok := startedToolIndex[e.ID]; ok {
						startedToolCalls[idx].Name = e.Name
						startedToolCalls[idx].Input = json.RawMessage(e.Input)
					} else {
						startedToolIndex[e.ID] = len(startedToolCalls)
						startedToolCalls = append(startedToolCalls, cometsdk.ToolCallBlock{
							ID:    e.ID,
							Name:  e.Name,
							Input: json.RawMessage(e.Input),
						})
					}
					ch <- event.ToolCall(e.ID, e.Name, []byte(e.Input))
				case cometsdk.StepFinishEvent:
					ch <- event.StepFinish(e.Usage)
				}
			}
			result, err = stream.Result()
			failureCategory := classifyStreamFailure(err)
			logging.L().Info("llm.stream.events_closed", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "events", eventCount, "first_event", firstEventLogged, "first_output", firstOutputLogged, "complete_tool_call", completeToolCall, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds())
			if err == nil {
				break
			}
			// A cancelled runner context is the explicit /stop path (or a client
			// disconnect). It is normal control flow, not a provider failure:
			// retain any visible partial output, close with done, and do not add an
			// error transcript row or SSE error card.
			if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
				persistPartialStep(ctx, r.Sessions, turn.ID, turn.ProviderID, turn.ModelID, result, s.pendingMemories)
				logging.L().Info("agent.step.stopped", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "duration_ms", time.Since(streamStarted).Milliseconds())
				return nil
			}
			if !overflowRecovered && isContextOverflowError(err) && !completeToolCall && r.Compactor != nil && sess.ID != "" {
				overflowRecovered = true
				logging.L().Warn("agent.step.overflow_recover", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "error", err)
				tools := requestTools
				beforeSummary := sess.ContextSummary
				beforeUntil := sess.CompactedUntilMessageID
				updated, compactErr := r.Compactor.MaybeCompact(
					ctx,
					*sess,
					baseSystem,
					tools,
					r.Provider,
					turn.ProviderID,
					turn.ModelID,
					true,
					func(ev event.Event) { ch <- ev },
				)
				if compactErr == nil {
					*sess = updated
					baseSystem = r.buildSystemPrompt(sess.ContextSummary, effectiveMaxTokens)
					system = baseSystem + memoryPromptSuffix
					rebuildMsgs, rebuildErr := r.Sessions.BuildSDKMessages(ctx, turn.ID)
					if rebuildErr == nil {
						rebuildMsgs, _ = NormalizeHistory(rebuildMsgs)
						rebuildMsgs = append(rebuildMsgs, s.nudges.messages(jobTracker.JobID)...)
						if finalizing {
							rebuildMsgs = append(rebuildMsgs, FinalAnswerNudgeMessages()...)
						}
						msgs = rebuildMsgs
						req = r.buildTurnRequest(ctx, s, system, msgs, requestTools)
						toolOutputBytes = toolResultBytes(req.Messages)
						if sess.ContextSummary != beforeSummary || sess.CompactedUntilMessageID != beforeUntil {
							budget, budgetErr := r.Compactor.EstimatePromptBudget(
								ctx, sess.ID, baseSystem, tools, turn.ProviderID, turn.ModelID,
							)
							if budgetErr == nil {
								ch <- event.ContextBudget(budget.Estimated, budget.Available, budget.ContextWindow, true)
							}
						}
						continue
					}
					logging.L().Warn("agent.step.overflow_rebuild_failed", "session", turn.ID, "error", rebuildErr)
				} else {
					logging.L().Warn("agent.step.overflow_compact_failed", "session", turn.ID, "error", compactErr)
				}
			}
			if ctx.Err() == nil && recoveryAttempt < maxStreamRecoveryAttempts && recoverableStreamFailure(err) && !completeToolCall {
				textChars, reasoningChars := partialRenderLengths(result)
				recoveryAttempt++
				delay := recoveryDelay(r.StreamRecoveryBackoff, recoveryAttempt)
				if ra := retryAfterDelay(err); ra > delay {
					delay = ra
				}
				logging.L().Warn("agent.step.recover", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "delay_ms", delay.Milliseconds(), "text_chars", textChars, "reasoning_chars", reasoningChars)
				ch <- event.TurnRecover(textChars, reasoningChars)
				if waitErr := waitForRecovery(ctx, delay); waitErr != nil {
					persistPartialStep(ctx, r.Sessions, turn.ID, turn.ProviderID, turn.ModelID, result, s.pendingMemories)
					if errors.Is(waitErr, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
						logging.L().Info("agent.step.stopped", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "duration_ms", time.Since(streamStarted).Milliseconds())
						return nil
					}
					logging.L().Error("agent.step.failed", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "events", eventCount, "first_event", firstEventLogged, "first_output", firstOutputLogged, "complete_tool_call", completeToolCall, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds(), "error", waitErr)
					ch <- event.Errorf(userFacingAgentError(waitErr), "llm")
					return waitErr
				}
				continue
			}
			persistPartialStep(ctx, r.Sessions, turn.ID, turn.ProviderID, turn.ModelID, result, s.pendingMemories)
			logging.L().Error("agent.step.failed", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "events", eventCount, "first_event", firstEventLogged, "first_output", firstOutputLogged, "complete_tool_call", completeToolCall, "failure_category", failureCategory, "recovery_attempt", recoveryAttempt, "duration_ms", time.Since(streamStarted).Milliseconds(), "error", err)
			ch <- event.Errorf(userFacingAgentError(err), "llm")
			return err
		}
		logging.L().Info("agent.step.finish", "session", turn.ID, "provider", r.Provider.ID(), "model", turn.ModelID, "step", s.steps+1, "finish_reason", string(result.FinishReason), "tool_calls", len(result.ToolCalls), "input_tokens", result.Usage.InputTokens, "output_tokens", result.Usage.OutputTokens, "recovery_attempt", recoveryAttempt)

		if err := r.Sessions.SaveTokenUsage(ctx, turn.ID, result.Usage, turn.ProviderID, turn.ModelID); err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return err
		}

		// Whitespace-only content (common from some mini models on tool s.steps)
		// is treated as empty so we do not persist invisible assistant bubbles.
		text := strings.TrimSpace(assistantPlainText(result.Message))
		reasoningBlocks := result.Message.ReasoningContent
		incompleteToolCalls := []cometsdk.ToolCallBlock(nil)
		persistToolCalls := result.ToolCalls
		if finalizing {
			if len(result.ToolCalls) > 0 || len(startedToolCalls) > 0 {
				logging.L().Warn("agent.final_answer.unexpected_tool_call", "session", turn.ID, "tool_calls", len(result.ToolCalls))
			}
			persistToolCalls = nil
		} else {
			incompleteToolCalls = incompleteStartedToolCalls(startedToolCalls, result.ToolCalls)
			if len(incompleteToolCalls) > 0 {
				persistToolCalls = append(append([]cometsdk.ToolCallBlock{}, result.ToolCalls...), incompleteToolCalls...)
			}
		}
		persistedToolIDs := map[string]string{}
		if text != "" || len(reasoningBlocks) > 0 || len(result.Message.ProviderState) > 0 || len(persistToolCalls) > 0 {
			assistant, toolIDs, err := r.Sessions.AppendAssistantStep(ctx, turn.ID, text, reasoningBlocks, persistToolCalls, s.pendingMemories)
			if err != nil {
				ch <- event.Errorf(err.Error(), "db")
				return err
			}
			persistedToolIDs = toolIDs
			if states, ok := r.Sessions.(providerStateStore); ok {
				if err := states.SaveAssistantProviderState(ctx, assistant.ID, scopeProviderState(result.Message.ProviderState, turn.ProviderID, turn.ModelID)); err != nil {
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
		s.pendingMemories = nil
		if finalizing {
			return r.completeTurn(ctx, s)
		}

		if len(incompleteToolCalls) > 0 {
			if err := settleIncompleteToolCalls(ctx, r.Sessions, turn.ID, incompleteToolCalls, persistedToolIDs, ch); err != nil {
				ch <- event.Errorf(err.Error(), "db")
				return err
			}
		}

		if result.FinishReason == cometsdk.FinishStop {
			if collected, waited, err := r.collectActiveSubagentResults(ctx, turn.ID); err != nil {
				ch <- event.Errorf(err.Error(), "subagents")
				return err
			} else if waited {
				s.nudges.subagentResults = collected
				s.nudges.subagentWait = false
				s.steps++
				continue
			}
			if jobTracker.TryConsumeCompletionGate() {
				s.nudges.jobCompletionGate = true
				logging.L().Info(
					"agent.job_completion_gate",
					"session", turn.ID,
					"job_id", jobTracker.JobID,
					"gate_used", jobTracker.completionGateUsed,
					"gate_budget", jobTracker.completionGateBudget,
				)
				s.steps++
				continue
			}
			return r.completeTurn(ctx, s)
		}
		if len(result.ToolCalls) == 0 {
			if result.FinishReason == cometsdk.FinishMaxTokens && len(incompleteToolCalls) > 0 {
				if s.incompleteToolTruncationContinuations < maxIncompleteToolTruncationContinuations && s.steps < r.MaxSteps {
					s.incompleteToolTruncationContinuations++
					s.nudges.incompleteToolTruncation = true
					logging.L().Info(
						"agent.incomplete_tool_truncation.continue",
						"session", turn.ID,
						"step", s.steps+1,
						"continuation", s.incompleteToolTruncationContinuations,
						"incomplete_tools", len(incompleteToolCalls),
						"max_tokens", effectiveMaxTokens,
					)
					s.steps++
					continue
				}
				logging.L().Info(
					"agent.incomplete_tool_truncation.stop",
					"session", turn.ID,
					"step", s.steps+1,
					"incomplete_tools", len(incompleteToolCalls),
					"max_tokens", effectiveMaxTokens,
				)
				return r.completeTurn(ctx, s)
			}
			if result.FinishReason == cometsdk.FinishMaxTokens &&
				s.outputTruncationContinuations < maxOutputTruncationContinuations &&
				s.steps < r.MaxSteps {
				s.outputTruncationContinuations++
				s.nudges.truncation = true
				logging.L().Info(
					"agent.output_truncation.continue",
					"session", turn.ID,
					"step", s.steps+1,
					"continuation", s.outputTruncationContinuations,
					"max_tokens", effectiveMaxTokens,
				)
				s.steps++
				continue
			}
			if collected, waited, err := r.collectActiveSubagentResults(ctx, turn.ID); err != nil {
				ch <- event.Errorf(err.Error(), "subagents")
				return err
			} else if waited {
				s.nudges.subagentResults = collected
				s.nudges.subagentWait = false
				s.steps++
				continue
			}
			if jobTracker.TryConsumeCompletionGate() {
				s.nudges.jobCompletionGate = true
				logging.L().Info(
					"agent.job_completion_gate",
					"session", turn.ID,
					"job_id", jobTracker.JobID,
					"gate_used", jobTracker.completionGateUsed,
					"gate_budget", jobTracker.completionGateBudget,
				)
				s.steps++
				continue
			}
			return r.completeTurn(ctx, s)
		}

		s.emitStatus(event.PhaseRunningTools)
		schemaCircuitOpen := false
		doomLoopHit := false
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
			var res tools.Result
			var execErr error
			recentTools = append(recentTools, FingerprintTool(tc.Name, tc.Input))
			skipInvalidInput := schemaCircuitOpen && !tools.IsCompleteJSONObject(tc.Input)
			if IsDoomLoop(recentTools, DoomLoopThreshold) {
				res = tools.Result{OK: false, Output: doomLoopToolResult(tc.Name)}
				doomLoopHit = true
				logging.L().Warn("agent.doom_loop.blocked", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID)
			} else if skipInvalidInput {
				res = tools.Result{OK: false, Output: skippedInvalidToolInputResult(tc.Name)}
				logging.L().Warn("tool.call.schema_circuit_open", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "streak", s.invalidToolInputStreak)
			} else {
				toolCtx := tools.WithToolSession(ctx, turn.ID)
				toolCtx = tools.WithProgress(toolCtx, backgroundProgressEmitter(ch))
				res, execErr = r.Registry.Execute(toolCtx, tc.Name, tc.Input)
			}
			dur := time.Since(start).Milliseconds()
			logging.L().Info("tool.call.finish", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "ok", res.OK && execErr == nil, "duration_ms", dur, "output_bytes", len(res.Output))

			out := res.Output
			isErr := !res.OK
			if execErr != nil {
				isErr = true
				out = fmt.Sprintf("%s\n(execute error: %v)", out, execErr)
			}
			if !skipInvalidInput {
				if !res.OK && tools.IsInvalidToolInput(res, execErr) {
					s.invalidToolInputStreak++
					if s.invalidToolInputStreak >= maxConsecutiveInvalidToolInputs {
						schemaCircuitOpen = true
					}
				} else {
					s.invalidToolInputStreak = 0
				}
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
				s.nudges.jobProgress = true
			}
			if ctx.Err() != nil {
				if err := persistCancelledToolResults(ctx, r.Sessions, turn.ID, result.ToolCalls[i+1:], persistedToolIDs); err != nil {
					ch <- event.Errorf(err.Error(), "db")
					return err
				}
				return nil
			}
		}
		if doomLoopHit {
			s.doomLoopHalt = true
		}
		if r.hasActiveSubagents(turn.ID) {
			s.nudges.subagentWait = true
		} else {
			s.nudges.subagentWait = false
		}
		s.nudges.subagentResults = ""

		s.steps++
	}

	return r.completeTurn(ctx, s)
}
