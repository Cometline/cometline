# First-turn transcript invisible after user bubble flight

**Date:** 2026-06-15  
**Components:** `ChatView.svelte`, `ChatThread.svelte`, `chat.svelte.ts`, `session.svelte.ts`

## Symptom

After sending the **first message** from home (pending message → navigate to `/session/[id]`):

1. The user bubble flew from the hero composer to its thread position as expected.
2. Immediately afterward the **entire thread went blank** — composer stayed docked at the bottom, but no user message, no assistant reply, no empty-state copy.
3. Switching to another session and back **restored** the conversation (server transcript loaded normally).

The failure looked like a render bug in the flight animation, but the DOM was present with `opacity: 0`.

## Root cause

Two bugs stacked; either one could produce a blank thread on first turn.

### 1. `bindSession()` left `isLoading = true` with no loader to finish it

While fixing session-switch composer jank, `bindSession()` was changed to set `isLoading = true` when switching sessions so `hasVisibleConversation` stayed true and the composer did not flash to hero layout.

On the **pending first-message path**:

1. `$effect.pre` → `bindSession(sessionId)` → `isLoading = true`, `items = []`
2. `hasPendingMessage(sessionId)` → **return early** (no `loadTranscript()`)
3. `onMount` consumes pending message and starts first-turn flight
4. `$effect.pre` re-runs after pending is consumed; `items.length > 0` guard skips `loadTranscript()` again
5. **`isLoading` never returns to `false`** — nothing runs the `loadTranscript()` `finally` block

`ChatThread` treats `chatStore.isLoading` as “initial transcript paint”:

```javascript
if (!isSessionSynced || chatStore.isLoading) {
  isInitialTranscriptPaint = true;
  return;
}
```

```css
.thread-messages.hydrating {
  opacity: 0;
  pointer-events: none;
}
```

So messages were mounted but **fully transparent**. Session switch fixed it because a new `loadTranscript()` completed and cleared `isLoading`.

### 2. In-flight `loadTranscript()` could wipe staged first-turn items

When pending was consumed, `$effect.pre` re-ran **before** `stageUser()`:

1. `hasPendingMessage` became false → `loadTranscript()` started with `items = []`
2. First-turn flight called `stageUser()` → local user row appeared
3. Empty transcript fetch resolved while `isStreaming` was still false (flight runs before `send()`)
4. Async handler applied `items = []` from the server, **deleting the staged user**

`send()` uses `skipUser: true` after flight, so the user row was not re-added locally. Switching sessions reloaded the persisted user turn from SQLite.

## Fix

### 1. `bindSession()` does not own loading state

```javascript
function bindSession(nextSessionID: string) {
  // ...
  isLoading = false; // only loadTranscript() sets true/false
}
```

Session-switch composer stability stays in `ChatView` `$effect.pre` (`dockComposer()` when already docked or when `loadTranscript()` sets loading) and in the crossfade snapshot logic — not by forcing `isLoading` in `bindSession()`.

### 2. Do not overwrite local items when a fetch completes late

In `loadTranscript()` async success/error handlers:

```javascript
if (sessionID === nextSessionID && items.length > 0) return;
```

Also skip starting a load in `ChatView` `$effect.pre` when the session already has local items:

```javascript
if (chatStore.sessionID === sessionId && chatStore.items.length > 0) return;
```

### 3. Hydration only hides an empty loading shell

`ChatThread` hydration effect:

```javascript
if (chatStore.isLoading && threadItems.length === 0) {
  isInitialTranscriptPaint = true;
  return;
}
```

If local items exist (first-turn staging, streaming), keep them visible even when `isLoading` is stale.

### 4. First-turn layout flags in `hasVisibleConversation`

Include `awaitingFirstAssistant || firstTurnActive` so the thread shell mounts as soon as the first turn starts, before `stageUser()` populates `items`.

## Relation to other postmortems

- [composer-phase-and-positioning.md](./composer-phase-and-positioning.md) — session route composer phase; do **not** fix composer flash by setting `isLoading` in `bindSession()`. Empty session should stay centered until first send; loading UI on the session route is the thread shell, not the home hero.
- [first-turn-fly-transition-bugs.md](./first-turn-fly-transition-bugs.md) — different mechanism (`reveal: false` / `transition:fly`); same symptom class (user bubble vanishes).

## How to avoid regressions

- **`bindSession()` clears items and session id only.** Loading flags belong to `loadTranscript()`, not session binding.
- **Pending-message path never assumes a loader will run.** If `$effect.pre` skips `loadTranscript()` for pending, nothing clears a loading flag set elsewhere.
- **Any async transcript apply must re-check `items.length` and `isStreaming`**, not only at call time. First-turn flight can add rows while a fetch started earlier is still in flight.
- **`.thread-messages.hydrating` is a debug signal for invisible transcripts.** If the composer is docked and the pane is blank white, check `chatStore.isLoading` and `isInitialTranscriptPaint` before blaming flight or SSE.
- **Do not clear `awaitingFirstAssistant` from layout reset effects** when `hasVisibleConversation` flickers; only `onFirstTurnComplete` should end the first-turn layout phase.

## Verification

1. Home → type first message → navigate → user bubble flies → **stays visible**; assistant stream appears in the same session without switching away.
2. Empty session in sidebar → send first message (no pending) → same behavior.
3. Switch between two populated sessions → no hero composer flash; transcript visible after switch.
4. Slow network: start first message before an empty-session `loadTranscript()` completes → staged user is not wiped when the fetch returns.
