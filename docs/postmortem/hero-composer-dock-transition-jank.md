# Hero → chat composer transition jank

**Date:** 2026-06-14  
**Components:** `ChatView.svelte`, `FirstTurnFlight.svelte`, `Composer.svelte`, `+page.svelte`, `app.css`

## Symptom

The first-message transition from hero to docked chat felt wrong:

- Composer **dropped immediately** to the bottom, then **transitioned again** seconds later.
- Hero → dock motion did not feel like one continuous 560ms animation with the first-turn flight.
- Composer width/position on hero did not match the in-chat dock composer, so the handoff looked like a jump even when CSS `transition` was enabled.

## Root cause

Several issues stacked; fixing only duration or width was not enough.

### 1. Reactive `$effect` docked too early

A `$effect` keyed off `hasVisibleConversation`, which includes `chatStore.isLoading`:

```javascript
// broken pattern
if (hasVisibleConversation && !firstTurnActive) {
	shellStore.dockComposer();
}
```

On an empty session, `loadTranscript()` sets `isLoading = true` before any messages exist. The composer docked **while the empty hero was still visible**. Later, first-turn flight or stream completion triggered another layout change — felt like a second transition “a few seconds later”.

### 2. Composer dock was sequenced after flight, not with it

`dockComposer()` originally ran in `onFlightDoneChange` — after the 560ms flight. User bubble and avatar flew first; composer moved in a **separate** 560ms pass afterward (~1.1s total, two beats).

### 3. Hero and dock used different positioning models

| Surface                   | Composer placement                                                   |
| ------------------------- | -------------------------------------------------------------------- |
| Home (`+page.svelte`)     | Grid item, `position: relative` under empty state                    |
| Session hero (`ChatView`) | `position: absolute`, `bottom: calc(50% - 11rem)`, `translateY(50%)` |
| Session dock              | `position: absolute`, `bottom: var(--composer-dock-bottom)`          |

CSS `transition` on `bottom` / `transform` animates between **computed token values**, not the element’s **painted** position. When hero layout (grid vs absolute) or empty-state reflow shifts the box, the browser may snap before interpolating.

First-turn flight captures the textarea with `getBoundingClientRect()` (real pixels). Composer dock still used CSS properties only — message and composer did not share the same motion origin.

### 4. Layout mode flip at flight start

When `firstTurnActive` became true, empty state unmounted and thread mounted in the same turn as dock. Thread clip (`thread-shell.docked`) and composer position changed together without a shared pixel-space animation plan.

## Fix (applied)

1. **Remove reactive composer dock/center `$effect`.** Dock or center only on explicit events:
    - After `loadTranscript()` resolves: dock if `items.length > 0`, else `centerComposer()`.
    - First turn: `onPrepareFlight` in `FirstTurnFlight` (see below).

2. **Dock in realtime with flight** — `FirstTurnFlight` calls `onPrepareFlight()` after capturing hero rects, in the same frame as `setActive(true)` and before particle animation. Composer, thread clip, user bubble, and avatar all run over `--duration-flight` (560ms).

3. **Capture flight origins before layout flip** — In `FirstTurnFlight.animate()`, read empty avatar + textarea rects **before** `setActive(true)` / `stageUser()`.

4. **Unified composer width** — Hero and dock both use `--chat-composer-width` (same as `--chat-thread-width`) with a shared `--chat-gutter`.

5. **Thread clip transition** — `thread-shell` transitions `bottom` over `--duration-flight` when `.docked` applies.

6. **Composer styling transition** — `Composer.svelte` uses `--duration-flight` for width, padding, border-radius, etc., so hero → dock variant changes track the move.

### `onPrepareFlight` hook

```typescript
// FirstTurnFlight.svelte — after rect capture, before/at flight start
onPrepareFlight?.();
setActive(true);
stageUser(text);
```

```typescript
// ChatView.svelte
onPrepareFlight={() => shellStore.dockComposer()}
```

Do **not** call `dockComposer()` again in `onFlightDoneChange`.

## Recommended follow-up (not yet implemented)

For pixel-perfect hero → dock when grid and absolute layouts diverge, use **FLIP** on the composer wrapper:

1. `getBoundingClientRect()` on `.composer-wrapper` while still hero/centered.
2. Apply dock layout (remove `.centered`, set dock `bottom`).
3. Measure again; set `transform` to the delta; animate `transform` to `none` over `--duration-flight`.

Alternatively, use **one hero positioning system everywhere** (home + session both grid, or both absolute with a tuned `--composer-hero-bottom` that matches the grid layout exactly).

Until FLIP or unified placement lands, small viewport / padding changes can reintroduce a visible snap.

## How to avoid regressions

- **Never dock from `hasVisibleConversation` alone** — it includes `isLoading`. Gate on `chatStore.items.length > 0` or an explicit first-turn flag.

- **First-turn composer:** dock via `onPrepareFlight`, not `$effect`, not `onFlightDoneChange`.

- **One timing token:** flight particles, composer move, thread clip, and composer chrome should all use `--duration-flight`. Keep `FLIGHT_MS` in `first-turn-flight.ts` in sync (currently 560).

- **Separate first-turn flags:**
    - `firstTurnFlightDone` — flight overlay (~560ms)
    - `awaitingFirstAssistant` — first stream until `onFirstTurnComplete`
    - Do not use `composerPhase` alone to infer hero layout during first turn after `onPrepareFlight` docks.

- **Before changing hero layout**, check both `+page.svelte` and `ChatView.svelte` — they must agree on composer placement or FLIP is required.

## Verification

1. Empty session → composer stays centered until first send; no dock during `loadTranscript`.
2. First send → user bubble, avatar, composer, and thread clip animate together (~560ms); no second dock when the stream ends.
3. Open session with existing messages → composer docked once after transcript load, no hero flash.
4. Home → new session with pending message → first turn matches (2) after navigation.
5. Composer width identical in hero and dock on the same breakpoint.
