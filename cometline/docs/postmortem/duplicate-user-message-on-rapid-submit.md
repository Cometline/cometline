# Duplicate user message on rapid submit

**Date:** 2026-06-14  
**Components:** `chat-turn-queue.ts`, `ChatView.svelte`, `Composer.svelte`, `start-chat.ts`

## Symptom

Sending a user message too quickly (double Enter, double click, or submit during the first-turn flight before streaming started) could deliver the **same text twice** to the backend: once as the active turn and again when the turn queue drained.

## Root cause

Two gaps combined:

### 1. Turn queue accepted duplicates while a turn was in flight

`createChatTurnQueue` serializes turns with a `processing` flag:

- First submit while idle → `runLoop(text)` runs immediately.
- Second submit while `processing` → same `text` pushed onto the pending queue.

When the first turn finished, the queue runner sent the duplicate text again. There was no dedupe against the **currently active** turn text.

This became more noticeable after every user message went through `onUserMessageFlight` (stage + flight + `send({ skipUser: true })`), because the turn stays `processing` for the full flight + stream + refresh window.

### 2. Composer allowed submit during flight but before `isStreaming`

The send button switches to Stop only when `chatStore.isStreaming` is true. During the first-turn flight (~560ms) and the brief gap before `send()` sets `isStreaming`, the send button stayed enabled.

`waitingForReply` was passed to `Composer` but only changed the placeholder — `submit()` did not check it. A second submit during flight enqueued a duplicate of the message already being staged/flown.

### 3. (Related) Unsafe `runLoop` re-entry

If `runLoop(initialText)` were ever invoked while `processing` was already true, the old guard only returned early when `initialText === undefined`. A concurrent `runLoop(text)` could have started a parallel turn. `enqueue` did not call that path, but the guard was brittle.

## Fix

1. **`chat-turn-queue.ts`**
   - Track `activeTurnText` for the in-flight turn.
   - On enqueue while busy: skip if `text === activeTurnText` or matches the last queued item.
   - If `runLoop` is called while `processing`, queue (or dedupe) instead of running in parallel.

2. **`ChatView.svelte`**
   - ~~Expose `turnQueue.processing` as `turnProcessing` via `syncQueueState`.~~ Removed 2026-06-16; queue state stays internal.

3. **`Composer.svelte`**
   - ~~`sendLocked = turnProcessing && !streaming`~~ **Removed (2026-06-16):** Composer no longer reads `turnProcessing`. Duplicate protection is queue-only (`activeTurnText` dedupe). Send unlock aligns with `!isStreaming` so post-stream `refreshSession` does not block the composer.

Intentional queueing during an active **stream** is unchanged: different messages can still be queued with Enter while the assistant reply is streaming.

## How to avoid regressions

- **Dedupe on enqueue**, not only in the UI — double events can bypass button disabled state; the queue is the source of truth.
- **`turnQueue.processing` is internal** — covers flight + send + `refreshSession` for serialization; do not expose it to Composer as a send lock.
- **`waitingForReply` is UX copy** — tracks `isStreaming || firstTurnActive`; it does not gate submit.
- **While streaming**, keep allowing queue submits.

## Verification

1. New chat → double-click Send or double Enter on the first message → only one user bubble and one API turn.
2. During assistant streaming → queue two **different** messages → both send in order after the current turn.
3. During streaming → submit the same queued text twice → second duplicate is ignored.
4. After a turn completes → normal single send still works.
