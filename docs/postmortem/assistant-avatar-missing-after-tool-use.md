# Assistant avatar missing after tool use

**Date:** 2026-06-14  
**Components:** `reducers/chat.ts`, `ChatThread.svelte`

## Symptom

After the assistant called tools, the final response text bubble was missing its avatar. Earlier assistant bubbles showed the avatar correctly; only the post-tool response appeared without one.

## Root cause

1. **`reducers/chat.ts` reset `assistant.current` on every `tool_call` and `step_finish`** via `finishAssistantSegment()`. When text resumed after the tools, `ensureAssistantForText()` created a brand-new assistant item.

2. **`ChatThread.svelte` treated consecutive assistant items as a continuation**. The second assistant row got the `avatar-gutter` instead of `avatar-mini`, so the avatar disappeared.

3. **Tools were also rendered as standalone rows** in addition to being folded into the "Thinking" toggle, which made the split between text segments visually obvious.

## Fix

- In **`reducers/chat.ts`**, settle reasoning/text pending state on `tool_call` and `step_finish`, but keep `assistant.current` so later `text_delta` events append to the same assistant bubble.
- In **`ChatThread.svelte`**, attach all tools within a turn to the active assistant's "Thinking" toggle and stop rendering them as separate rows.
- Update reducer tests to expect merged text across tool calls.

## How to avoid regressions

Only clear `assistant.current` at true turn boundaries (`done`, `error`, or a new user message). If you add a new stream event that should split the assistant bubble, you must also ensure the UI renders an avatar for the new segment.
