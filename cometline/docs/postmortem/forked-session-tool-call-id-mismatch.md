# Forked session HTTP 400 from stale tool_call IDs

**Date:** 2026-06-17  
**Components:** `cometmind/internal/session/service.go` (`ForkSession`), `Composer.svelte` (`/change` → fork)

## Symptom

After using `/change` to fork a session into a different workspace directory, the
new forked session could not send messages. Any request returned:

```
cometsdk: openai: server error (HTTP 400): Error from provider: Provider returned error
```

This only happened on forked sessions whose original transcript contained at
least one tool call (e.g. `run_command`, `read_file`). Forked sessions with a
text-only history worked fine.

## Root cause

`ForkSession` copies the source session's full transcript — messages and their
`tool_calls` rows — into the new session. Each copied row is given a fresh ULID
(`id.New()`), which is correct for primary keys. The bug was that **tool-call
IDs are referenced in two places that must stay in sync**:

1. The assistant message's tool-call block. `BuildSDKMessages` reads the
   tool-call's own row ID (`tc.ID`) into `cometsdk.ToolCallBlock.ID`.
2. The matching `tool_result` message. The referenced ID is stored **inside the
   message's JSON content** as `toolResultPayload.ToolCallID`, not as a foreign
   key column.

The original fork loop only remapped (1) — it gave each copied `tool_calls` row
a new ID — but copied the `tool_result` message content verbatim, so (2) still
pointed at the **old** tool-call ID.

After forking, `BuildSDKMessages` therefore emitted:

- an assistant `ToolCallBlock` with the **new** ID, and
- a `ToolResultBlock` whose `ToolCallID` was the **old** ID.

OpenAI (and OpenAI-compatible providers) require every tool result's
`tool_call_id` to match an `id` in the preceding assistant message's
`tool_calls`. The mismatch is rejected as a malformed request → HTTP 400.

Because the live system prompt is rebuilt by the agent and stored `system` rows
are skipped in `BuildSDKMessages`, the forked "Forked from a session in …"
notice was *not* the cause — it is dropped before the provider call.

## Fix

In `ForkSession`, remap tool-call IDs consistently across both references:

- Maintain an `oldToolCallID → newToolCallID` map while iterating messages.
  Messages are ordered by `created_at ASC`, so an assistant's `tool_calls` are
  always copied before the `tool_result` that references them.
- When copying a `tool_calls` row, generate the new ID, record it in the map,
  and insert with the new ID.
- When copying a `tool_result` message, rewrite its content's `tool_call_id`
  through the map before persisting (new helper `remapToolResultContent`).
  Unknown IDs are passed through unchanged.

## How to avoid regressions

- **Tool-call IDs live in two coupled places.** The `tool_calls.id` column and
  the `tool_call_id` embedded in `tool_result` message JSON must always agree.
  Any operation that clones or rewrites messages (fork, future "duplicate
  session", export/import) must remap both together.
- **Prefer round-tripping through `BuildSDKMessages` in tests.** Asserting that
  the forked `ToolCallBlock.ID` equals the forked `ToolResultBlock.ToolCallID`
  (and differs from the original) catches the exact shape the provider rejects.
- **System rows are not sent to providers.** Don't blame the fork system notice
  for provider errors; it is skipped in `BuildSDKMessages`.

## Verification

1. `cd cometmind && go test -run TestForkSessionRemapsToolCallIDs ./internal/session/...`
   — asserts the forked tool_call ID is remapped and matches the tool_result.
2. In-app: run a tool in a session (e.g. "run pwd"), `/change` to fork it into
   another directory, then send a message in the fork → streams a normal reply
   instead of HTTP 400.
3. `cd cometmind && go test ./...` — full suite green (136 tests).
