package agent

import (
	"context"
	"strings"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
)

// stepOutcome tells Run how to proceed after one model step.
type stepOutcome int

const (
	// stepDone ends the turn. A nil error means it finished normally.
	stepDone stepOutcome = iota
	// stepAgain runs another model step. The step counter is already advanced.
	stepAgain
	// stepTools runs the tool calls from this step, then advances the counter.
	stepTools
)

// settleModelStep persists the model step and decides whether the turn stops,
// continues with another model call, or executes tool calls.
func (r *Runner) settleModelStep(
	ctx context.Context,
	s *turnState,
	p *stepRequest,
	result *llm.GenerateMessageResult,
	startedToolCalls []cometsdk.ToolCallBlock,
) (stepOutcome, map[string]string, error) {
	turn := s.turn
	if err := r.Sessions.SaveTokenUsage(ctx, turn.ID, result.Usage, turn.ProviderID, turn.ModelID); err != nil {
		s.ch <- event.Errorf(err.Error(), "db")
		return stepDone, nil, err
	}

	persistedToolIDs, incomplete, err := r.persistAssistantStep(ctx, s, p.finalizing, result, startedToolCalls)
	if err != nil {
		return stepDone, nil, err
	}
	// Memories are attached to the first persisted assistant message only.
	s.pendingMemories = nil
	if p.finalizing {
		return stepDone, nil, r.completeTurn(ctx, s)
	}
	if len(incomplete) > 0 {
		if err := settleIncompleteToolCalls(ctx, r.Sessions, turn.ID, incomplete, persistedToolIDs, s.ch); err != nil {
			s.ch <- event.Errorf(err.Error(), "db")
			return stepDone, nil, err
		}
	}
	if result.FinishReason == cometsdk.FinishStop {
		outcome, err := r.continueForFollowUp(ctx, s)
		return outcome, nil, err
	}
	if len(result.ToolCalls) == 0 {
		outcome, err := r.continueWithoutToolCalls(ctx, s, result, incomplete)
		return outcome, nil, err
	}
	return stepTools, persistedToolIDs, nil
}

func (r *Runner) persistAssistantStep(
	ctx context.Context,
	s *turnState,
	finalizing bool,
	result *llm.GenerateMessageResult,
	startedToolCalls []cometsdk.ToolCallBlock,
) (map[string]string, []cometsdk.ToolCallBlock, error) {
	turn := s.turn
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
			s.ch <- event.Errorf(err.Error(), "db")
			return nil, nil, err
		}
		persistedToolIDs = toolIDs
		if states, ok := r.Sessions.(providerStateStore); ok {
			if err := states.SaveAssistantProviderState(ctx, assistant.ID, scopeProviderState(result.Message.ProviderState, turn.ProviderID, turn.ModelID)); err != nil {
				s.ch <- event.Errorf(err.Error(), "db")
				return nil, nil, err
			}
		}
	}
	// Guard against providers that terminate a step without yielding any
	// replayable assistant payload. Persisting an empty assistant row poisons
	// later provider switches because many APIs reject assistant history with
	// neither content nor tool calls.
	return persistedToolIDs, incompleteToolCalls, nil
}

// continueForFollowUp injects subagent results or the job completion gate, or
// finishes the turn when neither applies.
func (r *Runner) continueForFollowUp(ctx context.Context, s *turnState) (stepOutcome, error) {
	if collected, waited, err := r.collectActiveSubagentResults(ctx, s.turn.ID); err != nil {
		s.ch <- event.Errorf(err.Error(), "subagents")
		return stepDone, err
	} else if waited {
		s.nudges.subagentResults = collected
		s.nudges.subagentWait = false
		s.steps++
		return stepAgain, nil
	}
	if s.jobTracker.TryConsumeCompletionGate() {
		s.nudges.jobCompletionGate = true
		logging.L().Info(
			"agent.job_completion_gate",
			"session", s.turn.ID,
			"job_id", s.jobTracker.JobID,
			"gate_used", s.jobTracker.completionGateUsed,
			"gate_budget", s.jobTracker.completionGateBudget,
		)
		s.steps++
		return stepAgain, nil
	}
	return stepDone, r.completeTurn(ctx, s)
}

func (r *Runner) continueWithoutToolCalls(
	ctx context.Context,
	s *turnState,
	result *llm.GenerateMessageResult,
	incompleteToolCalls []cometsdk.ToolCallBlock,
) (stepOutcome, error) {
	if result.FinishReason == cometsdk.FinishMaxTokens && len(incompleteToolCalls) > 0 {
		if s.incompleteToolTruncationContinuations < maxIncompleteToolTruncationContinuations && s.steps < r.MaxSteps {
			s.incompleteToolTruncationContinuations++
			s.nudges.incompleteToolTruncation = true
			logging.L().Info(
				"agent.incomplete_tool_truncation.continue",
				"session", s.turn.ID,
				"step", s.steps+1,
				"continuation", s.incompleteToolTruncationContinuations,
				"incomplete_tools", len(incompleteToolCalls),
				"max_tokens", s.maxTokens,
			)
			s.steps++
			return stepAgain, nil
		}
		logging.L().Info(
			"agent.incomplete_tool_truncation.stop",
			"session", s.turn.ID,
			"step", s.steps+1,
			"incomplete_tools", len(incompleteToolCalls),
			"max_tokens", s.maxTokens,
		)
		return stepDone, r.completeTurn(ctx, s)
	}
	if result.FinishReason == cometsdk.FinishMaxTokens &&
		s.outputTruncationContinuations < maxOutputTruncationContinuations &&
		s.steps < r.MaxSteps {
		s.outputTruncationContinuations++
		s.nudges.truncation = true
		logging.L().Info(
			"agent.output_truncation.continue",
			"session", s.turn.ID,
			"step", s.steps+1,
			"continuation", s.outputTruncationContinuations,
			"max_tokens", s.maxTokens,
		)
		s.steps++
		return stepAgain, nil
	}
	return r.continueForFollowUp(ctx, s)
}
