package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

// After this many consecutive schema/JSON argument failures in one turn,
// remaining incomplete JSON calls in the current step are skipped.
// Calls that already have a complete JSON object still run.
const maxConsecutiveInvalidToolInputs = 2

func int64PtrFromIntPtr(v *int) *int64 {
	if v == nil {
		return nil
	}
	x := int64(*v)
	return &x
}

const cancelledToolResult = "Tool execution cancelled before completion."
const truncatedIncompleteToolResult = "Tool call was cut off at the output token limit before it finished. It was not executed."

func skippedInvalidToolInputResult(name string) string {
	return fmt.Sprintf(
		"Skipped %s: too many consecutive invalid tool argument payloads in this turn. Do not retry with similar arguments. Emit one smaller, complete JSON object, or continue without this tool.",
		name,
	)
}

func incompleteStartedToolCalls(started, completed []cometsdk.ToolCallBlock) []cometsdk.ToolCallBlock {
	if len(started) == 0 {
		return nil
	}
	done := make(map[string]struct{}, len(completed))
	for _, tc := range completed {
		done[tc.ID] = struct{}{}
	}
	var out []cometsdk.ToolCallBlock
	for _, tc := range started {
		if _, ok := done[tc.ID]; ok {
			continue
		}
		input := tc.Input
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		out = append(out, cometsdk.ToolCallBlock{ID: tc.ID, Name: tc.Name, Input: input})
	}
	return out
}

func settleIncompleteToolCalls(
	ctx context.Context,
	store TurnStore,
	sessionID string,
	calls []cometsdk.ToolCallBlock,
	persistedToolIDs map[string]string,
	ch chan<- event.Event,
) error {
	for _, tc := range calls {
		persistedID := persistedToolIDs[tc.ID]
		if persistedID == "" {
			return fmt.Errorf("missing persisted tool call id for %s", tc.ID)
		}
		if err := persistToolResult(ctx, store, sessionID, persistedID, truncatedIncompleteToolResult, true, 0, nil); err != nil {
			return err
		}
		if ch != nil {
			ch <- event.ToolResult(tc.ID, tc.Name, truncatedIncompleteToolResult, truncatedIncompleteToolResult)
		}
	}
	return nil
}

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
