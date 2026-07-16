# Postmortem: Cancelled Tool Calls Poisoned Session Replay

Date: 2026-07-16

## Summary

Cancelling an agent turn after the model had emitted a tool call could leave a persisted assistant tool call without a matching tool result. The next message replayed that incomplete protocol state to the selected provider. Codex rejected it with `No tool output found for function call ...`; OpenAI-compatible providers failed upstream for the same reason.

## Impact

- A user could cancel an otherwise healthy turn and make the affected session unable to continue.
- Retrying the message did not help because every retry replayed the same orphaned tool call.
- Switching providers did not help because the invalid history was provider-neutral.
- The tool was not necessarily executed, but the persisted transcript represented it as an outstanding model request.

## Timeline

- A user cancelled a turn while the agent was processing tools.
- The following continuation request failed with Codex HTTP 400: `No tool output found for function call ...`.
- The same session then failed through an OpenAI-compatible provider, confirming that the failure was caused by the replayed history rather than a Codex-specific request format.
- Database inspection showed a persisted assistant tool call with an empty result and no matching `tool_result` message.

## Root Cause

The agent loop persists an assistant step, including its tool calls, before it executes those tools. A cancellation can arrive after that persistence point and before the corresponding tool result has been written.

The result-persistence calls used the turn context. Once cancellation propagated through that context, `UpdateToolCallResult` or `AppendToolResultMessage` could be skipped or fail. The transcript then contained only one side of a required tool-call/result pair.

During the next turn, `BuildSDKMessages` replayed every persisted tool call without checking whether a matching tool result existed. Providers correctly rejected this invalid conversation state.

## Fixes

- Persist every completed tool result through a short context detached from turn cancellation, so cancellation cannot interrupt the database update between the result row and the transcript row.
- When cancellation is detected before a tool starts, do not execute the tool. Instead, persist an error tool result: `Tool execution cancelled before completion.`
- When cancellation is detected after one tool returns, persist cancellation results for every remaining unstarted tool call before ending the turn.
- Filter historical assistant tool calls during replay when no matching `tool_result` exists. This repairs already-affected sessions without running tools again or exposing the rest of `~/.cometmind`.
- Keep the existing empty-assistant normalization so an assistant message whose only calls were filtered is not sent to providers.

## Verification

- Added an agent test that cancels immediately after an assistant tool call has been persisted and verifies that a cancellation result is stored.
- Added a session-history test that confirms orphaned tool calls are excluded while completed calls remain replayable.
- `go test ./...` passed in `cometmind`.

## Recovery

After deploying the fixed sidecar, the next message in an already-affected session skips orphaned historical tool calls and can continue normally. No manual database edit or provider switch is required. The original unexecuted tool is intentionally not retried automatically because it may have external side effects.

## Prevention

- Treat a model tool call and its result as an atomic replay contract, even when persistence spans multiple database writes.
- Use cancellation-detached persistence for the terminal record of work that has already started.
- Add cancellation coverage at each boundary: before tool execution, during tool execution, and after one tool in a multi-tool response.
- Validate history before replaying it to any provider, not only in provider-specific adapters.
