# Streaming avatar disappears and spinner jumps between turns

**Date:** 2026-06-17  
**Components:** `ChatThread.svelte`, `chat.svelte.ts`

## Symptom

On the second (and subsequent) messages, the assistant avatar disappeared briefly before the thinking block appeared. The spinner also jumped: it first appeared below the previous response, then moved to the new assistant row once it rendered. On the first turn, two avatars appeared simultaneously.

## Root cause

Three interrelated issues in the streaming render pipeline:

1. **No assistant item existed between `markStreaming` and the first SSE event.** `send()` called `markStreaming` before any assistant item was in the items array. The orphan pending row (a standalone avatar + spinner snippet) filled this gap, but it had no transition and was removed instantly when the first batched `reasoning_start` event created the real assistant item. The assistant row then appeared with a `fly` intro transition (opacity 0 → 1, y offset), leaving a visible gap where no avatar existed.

2. **`markStreaming` ran before `addUserToSession`.** This caused a render frame where `sessionStreaming` was true but the last item was still the old assistant. `streamingAssistantId` pointed to the old assistant, briefly showing the activity spinner on the previous response.

3. **First-turn double avatar.** The pre-created empty assistant (added to fix issue 1) was not found by `firstAssistantItem` (which requires text or reasoning content), so `firstAssistantId` was null. `firstAssistantInNormalList` then returned true, rendering the assistant in the normal list alongside the first-turn avatar slot.

## Fix

- **Pre-create an empty assistant item in `send()`** immediately after `markStreaming`, and set it as `ctx.assistant.current`. This eliminates the orphan phase entirely — `streamingAssistantId` points to the real assistant from the start, and the spinner renders in the correct position.

- **Reorder `addUserToSession` before `markStreaming`** so no render frame shows streaming state with stale items.

- **Guard `firstAssistantInNormalList`** with `if (showFirstTurnAvatarSlot()) return false` to prevent the pre-created assistant from rendering in the normal list when the first-turn slot is active.

- **Remove the orphan pending row entirely** (`assistantPendingRow` snippet, `showOrphanPendingAssistant`, `isAwaitingVisibleStreamingReply`, `isAssistantPendingRenderedInThread`, `pendingAssistantRenderedInFirstTurnSlot`). The pre-created assistant makes the orphan unnecessary.

- **Remove all `in:fly={rowIntro(...)}` transitions** from assistant, tool, subagent, memory, status, and error rows. These transitions caused visual discontinuities during streaming and were unnecessary.

## How to avoid regressions

Always ensure an assistant item exists in the items array before `markStreaming` is called. The `done` event's `clearEmptyAssistant` handles cleanup if the stream fails before any content arrives. Do not reintroduce an orphan/pending row pattern — the pre-created assistant with `showAssistantActivitySpinner` is the single source of truth for the streaming spinner.
