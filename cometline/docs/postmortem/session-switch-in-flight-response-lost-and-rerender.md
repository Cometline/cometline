# Session switch: in-flight response lost, stuck, or replayed from scratch

**Date:** 2026-06-17  
**Components:** `conversation-controller.ts`, `ChatView.svelte`, `ChatThread.svelte`, `AssistantMarkdown.svelte`, `chat.svelte.ts`, `chat.svelte.test.ts`, `conversation-controller.test.ts`

## Symptom

While Session A was waiting on the LLM (reasoning or partial response streaming over SSE), switching to Session B and sending there, then switching back to Session A caused several failures:

1. **Response vanished** — only the user bubble remained (e.g. `"write a 1000 words joke"`), with no assistant/reasoning content. The session felt **stuck** until the user sent a **second** message on A.
2. **Early switch before SSE** — sending on A and switching away before reasoning started had the same stuck/orphan behavior.
3. **Response replayed on return** — when content did appear after switching back during an in-flight stream, the assistant text **re-rendered from empty** (typewriter catch-up) instead of showing the accumulated text immediately; only **new** tokens should animate.
4. **Cmd+T aborted or stranded the previous new chat** — quickly doing `Cmd+T`, sending a first message, then `Cmd+T` and sending another first message created two sessions, but the first session's response never appeared. One path aborted Session A's active SSE. Another path left Session A's first message as a pending route handoff that never started if `/session/A` did not activate before the user returned to `/`.
5. **`/change` forked session stuck on spinner** — using `/change` to fork into another workspace, then sending a message in the forked session, could leave the assistant row spinning forever. The fork succeeded and the user bubble appeared, but a stale transcript load from the pre-fork session could race with the forked route activation/send path.

Desired behavior: sessions stream **concurrently in the background**; switching away and back should show the **current** partial or completed transcript without re-fetch clobbering cache or replaying already-streamed text.

## Root cause

Four independent bugs compounded; two affected **data**, two affected **presentation**.

### 1. Turn bound to live `sessionId` (orphaned send)

`ChatView` passed the **live** `sessionId` prop into `chatStore.send` / `refreshConversationSession`:

```ts
send: (payload, opts) => chatStore.send(sessionId, payload, opts),
```

`runTurn` is async (user-bubble flight → `await send`). If the user switched to Session B before the flight finished, `send()` ran against **B's id** while the queued turn belonged to **A**. A's SSE never started correctly.

### 2. `loadTranscript` clobbered in-flight cache on return

`onMount()` always called `loadTranscript`, while `$effect.pre` used `shouldSkipTranscriptLoad()` — **two inconsistent paths**.

When returning to A:

- Background stream might be finished or `isStreamingFor(A)` false while server had not persisted the assistant turn yet.
- `loadTranscript` fetched a **user-only** transcript and `writeSessionItems` **overwrote** the cache that still held partial reasoning/response.

That matches the screenshot symptom and explains why a **second send** “fixed” A (new stream + `preAssistant`, not reliance on clobbered cache).

### 3. `bindSession` saved stale visible `items`

On leave, `bindSession` did `sessionCache.set(sessionID, items)` using the **visible** `items` ref, not `getCachedItems(sessionID)`. Background stream updates wrote to `sessionCache`; visible `items` could lag, producing snapshots missing assistant/reasoning rows.

### 4. `AssistantMarkdown` remount replayed typewriter from empty

`ChatThread` uses `{#each threadItems as item (item.id)}`. Switching sessions destroys A's DOM nodes and recreates them on return. `AssistantMarkdown` remounted with `displaySource = ''` while `streaming={true}`, so the reveal animation **replayed from scratch** even though `source` already held kilobytes of text.

Separately, `isInitialTranscriptPaint` could run hydration (opacity 0 → settle) when returning to a session that already had cached items, adding a “whole transcript fading in again” feel.

### 5. New-chat reset aborted every stream

`Cmd+T` and the sidebar New Chat action used `startNewChat()`, which selected no session and called `chatStore.clear()`. `clear()` is a destructive global reset: it calls `abortAllStreams()`, clears every session cache, and drops stream handles. That is correct for teardown, but wrong for ordinary navigation to the hero new-chat screen.

The result was a race where Session A's first turn could be in progress, the user pressed `Cmd+T` to create Session B, and Session A was silently aborted even though the product expectation is that sessions continue in the background.

### 6. Pending first-turn handoff was a single global slot

The home composer creates a session, stores the first message in `sessionStore.queuePendingMessage(session.id, ...)`, then navigates to `/session/:id`. `ChatView` consumes that pending handoff on mount and starts the turn so the first-turn animation can run.

Originally `sessionStore` had only one `pendingMessage`. Rapidly creating Session A and then Session B could overwrite A's pending first turn with B's, or leave A's pending turn unconsumed if navigation to A was superseded before `ChatView.onMount()` ran.

### 7. `/change` fork path loaded the wrong transcript after navigation

`Composer.applyWorkspaceChange()` handled both pure workspace changes and session forks. For forks it did:

```ts
await goto(`/session/${forkedId}`);
await onWorkspaceChanged?.();
```

In `ChatView`, `onWorkspaceChanged` was wired to `chatStore.loadTranscript(sessionId)`. After `goto`, Svelte route props and component effects can interleave: the callback may still close over the old `sessionId`, or it may run while the new forked session activation is already binding/loading/sending. The forked `ChatView` already loads its transcript on activation, so this callback was redundant and could perturb the active chat store at exactly the wrong time.

## Fix

### Data layer (`chat.svelte.ts`, `conversation-controller.ts`, `ChatView.svelte`)

1. **Capture turn session at enqueue** — `ensureQueue()` stores `queueForSessionId`; `runTurn` calls `deps.send(turnSessionId, …)` and `deps.refreshSession(turnSessionId)`. ChatView deps take explicit `sid`.
2. **`hasInFlightTurn(sessionId)`** — true when streaming, a stream handle exists, or cache has a pending assistant/reasoning item. Used by `shouldSkipTranscriptLoad`, `loadTranscript` early/post-fetch guards, and `onMount`.
3. **`bindSession` leave snapshot** — flush batch, then `sessionCache.set(sessionID, getCachedItems(sessionID))`.
4. **`send()` cleanup** — throw when `isStreamingFor` blocks duplicate send; abort handle on run mismatch; `finally` only finishes when `streamHandles.get(sessionId) === handle`.
5. **UI state on switch** — `awaitingFirstAssistant` restored via `chatStore.isAwaitingFirstAssistant(sessionId)` instead of unconditional reset.
6. **Non-destructive new chat** — `startNewChat()` uses `chatStore.detachActiveSession()` instead of `chatStore.clear()`. Detach flushes the active session batch, preserves cache and stream handles, unbinds the visible session, and shows the hero composer without aborting background sessions.
7. **Per-session pending handoff** — `sessionStore` stores pending first messages in a `Map` keyed by session id, so rapid new chats do not overwrite each other.
8. **Drain pending before leaving** — `startNewChat()` checks the current session for an unconsumed pending first message and starts it via `chatStore.send(..., { skipUser: false })` before returning to the hero screen. This sacrifices the first-turn animation only when the user leaves before activation, but preserves the core invariant: the submitted message is processed.
9. **No post-fork workspace callback** — after `/change` forks and navigates to `/session/{forkedId}`, skip `onWorkspaceChanged`. The new `ChatView` activation is responsible for loading the forked transcript. Keep `onWorkspaceChanged` only for pure workspace changes where no route/session fork happens.

### Presentation layer (`AssistantMarkdown.svelte`, `ChatThread.svelte`)

1. **Mount snap** — on first mount with non-empty `source`, set `displaySource = source` once (`snappedExistingSource`). Remount mid-stream shows accumulated text immediately; only **new** deltas use the typewriter reveal.
2. **Skip transcript hydration when cache exists** — `sessionHasCachedTranscript()` uses `getCachedItemCount`, `isStreamingFor`, and `hasInFlightTurn` so returning to A does not run `isInitialTranscriptPaint` settle/fade on already-hydrated content.

## How to avoid regressions

- **Never close over live route/prop session id inside async turn pipelines.** Capture session id when the turn is enqueued or when the per-session queue is created.
- **One gate for transcript load.** `onMount`, `$effect.pre`, and `loadTranscript` must share the same skip rules; prefer `hasInFlightTurn` over `isStreamingFor && items.length` alone.
- **Cache is authoritative for background streams.** On `bindSession` leave, persist `getCachedItems`, not visible `items`.
- **Server transcript lags in-flight SSE.** Do not overwrite local cache with fetch results while a turn is in flight or pending assistant rows exist locally.
- **Keyed `{#each}` remounts children on session switch.** Any streaming UI with local animation state (`displaySource`, typewriter) must **snap to current `source` on mount** when remounting mid-stream.
- **Fresh mount ≠ empty session.** If `getCachedItemCount(sessionId) > 0` or the session is streaming, skip `isInitialTranscriptPaint` hydration.
- **New Chat is navigation, not teardown.** `Cmd+T` / sidebar New Chat must not call destructive global cleanup. Use a detach/unbind operation so existing sessions can keep streaming in the background.
- **Pending first turns are session-scoped work, not global UI state.** A route handoff must be keyed by session id, and navigation away should either consume it into a background send or leave it safely associated with that session.
- **Fork navigation owns forked transcript loading.** Do not run extra workspace-change transcript loads after `goto(/session/{forkedId})`; they can close over stale route props and race the new session activation.

## Verification

1. `cd cometline && pnpm run test` — includes session-switch, in-flight clobber, turn-session capture, and duplicate-send throw cases.
2. Manual:
   - Session A: send a long prompt; wait for reasoning or partial text.
   - Switch to B; send another question; wait for B to respond.
   - Switch back to A → A's reasoning/response visible **immediately** at current length, continuing to update if still streaming; **no** user-only stuck state; **no** full typewriter replay from empty.
   - Rapid `Cmd+T` → send in A → `Cmd+T` → send in B → both sessions should stream or complete independently; returning to A should show A's accumulated response.
   - `/change /some/other/workspace` from an existing session → forked session opens with the fork note → send a message → assistant should stream or show a real error, not remain on an infinite spinner.

## Related postmortems

- [session-switch-slow-and-stuck-load.md](./session-switch-slow-and-stuck-load.md) — superseded load / empty snapshot on rapid switch (load promise cleanup, `snapshotLoading`).
- [composer-phase-and-positioning.md](./composer-phase-and-positioning.md) — composer phase vs transcript load timing.
- [streaming-ui-not-live-updating.md](./streaming-ui-not-live-updating.md) — `$state.raw` + immutable item updates for live SSE rendering.
