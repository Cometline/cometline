package agent

import (
	"context"
	"fmt"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/comet-sdk/llm"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

func (r *Runner) executeToolBatch(
	ctx context.Context,
	s *turnState,
	p *stepRequest,
	result *llm.GenerateMessageResult,
	persistedToolIDs map[string]string,
) error {
	s.emitStatus(event.PhaseRunningTools)
	schemaCircuitOpen := false
	doomLoopHit := false
	recentTools := p.recentTools
	for i := range result.ToolCalls {
		stop, err := r.executeOneTool(ctx, s, result, persistedToolIDs, i, &recentTools, &schemaCircuitOpen, &doomLoopHit)
		if err != nil || stop {
			return err
		}
	}
	if doomLoopHit {
		s.doomLoopHalt = true
	}
	s.nudges.subagentWait = r.hasActiveSubagents(s.turn.ID)
	s.nudges.subagentResults = ""
	return nil
}

func (r *Runner) executeOneTool(
	ctx context.Context,
	s *turnState,
	result *llm.GenerateMessageResult,
	persistedToolIDs map[string]string,
	i int,
	recentTools *[]ToolFingerprint,
	schemaCircuitOpen *bool,
	doomLoopHit *bool,
) (bool, error) {
	turn := s.turn
	ch := s.ch
	tc := result.ToolCalls[i]
	persistedID := persistedToolIDs[tc.ID]
	if persistedID == "" {
		ch <- event.Errorf("missing persisted tool call id", "db")
		return false, fmt.Errorf("missing persisted tool call id for %s", tc.ID)
	}
	if ctx.Err() != nil {
		if err := persistCancelledToolResults(ctx, r.Sessions, turn.ID, result.ToolCalls[i:], persistedToolIDs); err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return false, err
		}
		return true, nil
	}

	res, skipInvalidInput, dur, execErr := r.runToolCall(ctx, s, tc, recentTools, schemaCircuitOpen, doomLoopHit)
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
				*schemaCircuitOpen = true
			}
		} else {
			s.invalidToolInputStreak = 0
		}
	}
	if err := persistToolResult(ctx, r.Sessions, turn.ID, persistedID, out, isErr, dur, int64PtrFromIntPtr(res.ExitCode)); err != nil {
		ch <- event.Errorf(err.Error(), "db")
		return false, err
	}
	toolErr := ""
	if isErr {
		toolErr = out
	}
	ch <- event.ToolResult(tc.ID, tc.Name, out, toolErr)
	if s.jobTracker.ObserveTool(tc.Name, tc.Input) {
		s.nudges.jobProgress = true
	}
	if ctx.Err() != nil {
		if err := persistCancelledToolResults(ctx, r.Sessions, turn.ID, result.ToolCalls[i+1:], persistedToolIDs); err != nil {
			ch <- event.Errorf(err.Error(), "db")
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *Runner) runToolCall(
	ctx context.Context,
	s *turnState,
	tc cometsdk.ToolCallBlock,
	recentTools *[]ToolFingerprint,
	schemaCircuitOpen *bool,
	doomLoopHit *bool,
) (tools.Result, bool, int64, error) {
	turn := s.turn
	start := time.Now()
	logging.L().Info("tool.call.start", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "input_bytes", len(tc.Input))
	var res tools.Result
	var execErr error
	*recentTools = append(*recentTools, FingerprintTool(tc.Name, tc.Input))
	skipInvalidInput := *schemaCircuitOpen && !tools.IsCompleteJSONObject(tc.Input)
	if IsDoomLoop(*recentTools, DoomLoopThreshold) {
		res = tools.Result{OK: false, Output: doomLoopToolResult(tc.Name)}
		*doomLoopHit = true
		logging.L().Warn("agent.doom_loop.blocked", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID)
	} else if skipInvalidInput {
		res = tools.Result{OK: false, Output: skippedInvalidToolInputResult(tc.Name)}
		logging.L().Warn("tool.call.schema_circuit_open", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "streak", s.invalidToolInputStreak)
	} else {
		toolCtx := tools.WithToolSession(ctx, turn.ID)
		toolCtx = tools.WithProgress(toolCtx, backgroundProgressEmitter(s.ch))
		res, execErr = r.Registry.Execute(toolCtx, tc.Name, tc.Input)
	}
	dur := time.Since(start).Milliseconds()
	logging.L().Info("tool.call.finish", "session", turn.ID, "tool", tc.Name, "tool_call_id", tc.ID, "ok", res.OK && execErr == nil, "duration_ms", dur, "output_bytes", len(res.Output))
	return res, skipInvalidInput, dur, execErr
}
