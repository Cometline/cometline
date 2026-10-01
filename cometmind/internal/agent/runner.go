package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
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
	jobTracker := s.jobTracker
	effectiveMaxTokens := s.maxTokens

	// MaxSteps limits work rounds. If they are exhausted, make one final
	// tool-free request so the user still receives a best-effort answer.
	for s.steps <= r.MaxSteps {
		p, err := r.prepareStep(ctx, s)
		if err != nil {
			return err
		}
		finalizing := p.finalizing
		recentTools := p.recentTools
		result, startedToolCalls, done, err := r.streamStep(ctx, s, p)
		if done {
			return err
		}

		if err := r.Sessions.SaveTokenUsage(ctx, turn.ID, result.Usage, turn.ProviderID, turn.ModelID); err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return err
		}

		// Whitespace-only content (common from some mini models on tool steps)
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
