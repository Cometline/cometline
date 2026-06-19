# Composer phase and positioning bugs (hero dock jank + stuck hero after session switch)

**Date:** 2026-06-14  
**Components:** `ChatView.svelte`, `ChatThread.svelte`, `FirstTurnFlight.svelte`, `Composer.svelte`, `+page.svelte`, `chat.svelte.ts`, `session.svelte.ts`, `app.css`

## Symptom A: Hero → dock transition jank on first message

The first-message transition from hero to docked chat felt wrong:

- Composer **dropped immediately** to the bottom, then **transitioned again** seconds later.
- Hero → dock motion did not feel like one continuous 560ms animation with the first-turn flight.
- Composer width/position on hero did not match the in-chat dock composer, so the handoff looked like a jump even when CSS `transition` was enabled.

## Symptom B: Composer stuck in hero layout after session switch

After switching between chat sessions (especially from home or another session), the composer looked wrong:

- Input placeholder stayed **"Type something. Anything."** (hero copy) instead of dock copy **"Type something…"**
- Composer sat **bottom-left**, narrower than the thread column above it
- Thread messages were centered; composer was not aligned with them

## Root causes

### A1. Reactive `$effect` docked too early

A `$effect` keyed off `hasVisibleConversation`, which includes `chatStore.isLoading`. On an empty session, `loadTranscript()` sets `isLoading = true` before any messages exist. The composer docked **while the empty hero was still visible**.

### A2. Composer dock sequenced after flight, not with it

`dockComposer()` originally ran in `onFlightDoneChange` — after the 560ms flight. User bubble and avatar flew first; composer moved in a **separate** 560ms pass afterward (~1.1s total, two beats).

### A3. Hero and dock used different positioning models

| Surface                   | Composer placement                                                   |
| ------------------------- | -------------------------------------------------------------------- |
| Home (`+page.svelte`)     | Grid item, `position: relative` under empty state                    |
| Session hero (`ChatView`) | `position: absolute`, `bottom: calc(50% - 11rem)`, `translateY(50%)` |
| Session dock              | `position: absolute`, `bottom: var(--composer-dock-bottom)`          |

CSS `transition` on `bottom` / `transform` animates between **computed token values**, not the element's **painted** position.

### B1. `loadTranscript()` returned before the fetch finished

```javascript
// broken guard
if (sessionID === nextSessionID && isLoading) return;
```

`$effect.pre` started the load and set `isLoading = true`. `onMount` called `loadTranscript()` again, hit the guard, and **returned immediately** (resolving `undefined`, not a pending promise).

### B2. `onMount` dock/center ran on an empty transcript

The `.then()` ran while `items` were still `[]`, so it called **`centerComposer()`** even though the session had messages on the way.

### B3. Hero positioning on a session thread surface

With `composerPhase === 'centered'` on `ChatView`, hero and dock placement models diverged → visible horizontal/vertical misalignment.

### B4. `ChatView` hero grid lacked the home-page composer override

`+page.svelte` resets composer wrapper placement under `.chat-home.hero-layout`; `ChatView.svelte` did not have this override.

## Fix

### A: First-turn hero → dock

1. **Remove reactive composer dock/center `$effect`.** Dock or center only on explicit events.
2. **Dock in realtime with flight** — `FirstTurnFlight` calls `onPrepareFlight()` after capturing hero rects, in the same frame as `setActive(true)`.
3. **Capture flight origins before layout flip** — Read empty avatar + textarea rects **before** `setActive(true)` / `stageUser()`.
4. **Unified composer width** — Hero and dock both use `--chat-composer-width`.
5. **Thread clip transition** — `thread-shell` transitions `bottom` over `--duration-flight` when `.docked` applies.

### B: Session switch stuck hero

1. **In-flight `loadTranscript` promise** — `chat.svelte.ts` tracks `loadPromise` / `loadPromiseSession`. If the same session is already loading, callers await the existing promise.
2. **Session-scoped composer phase `$effect` in `ChatView`** — Replaced `onMount` `.then()` dock/center with reactive `$effect` that docks when `hasVisibleConversation` is true.
3. **`bindSession()` in `$effect.pre`** — Clears stale items before first paint.
4. **Hero-layout composer override in `ChatView`** — Copied `+page.svelte` rules into `ChatView.svelte`.

### Dock rules by surface

| Surface | Loading UI | Dock when `isLoading`? |
| ------- | ---------- | ---------------------- |
| `+page.svelte` (home) | Empty hero | No |
| `ChatView` (session, empty, no items yet) | Thread loading | Yes (brief; then center if transcript empty) |
| `ChatView` (session, items or streaming) | Thread | Yes |

## How to avoid regressions

- **Never dock from `hasVisibleConversation` alone on the home surface** — it includes `isLoading`. Gate on `chatStore.items.length > 0` or an explicit first-turn flag.

- **First-turn composer:** dock via `onPrepareFlight`, not `$effect`, not `onFlightDoneChange`.

- **One timing token:** flight particles, composer move, thread clip, and composer chrome should all use `--duration-flight`. Keep `FLIGHT_MS` in `first-turn-flight.ts` in sync (currently 560).

- **Separate first-turn flags:**
    - `firstTurnFlightDone` — flight overlay (~560ms)
    - `awaitingFirstAssistant` — first stream until `onFirstTurnComplete`

- **Do not gate `loadTranscript` with a bare `isLoading` return** unless you return the in-flight promise.

- **Do not dock/center from `onMount().then(loadTranscript)`** when `$effect.pre` also starts the load. Use one owner for composer phase on the session route.

- **Placeholder string is a debug signal** — `"Type something. Anything."` means `composerPhase` is still `'centered'` on a session that should be docked.

- **Keep `+page.svelte` and `ChatView.svelte` hero composer CSS in sync** when changing `.composer-wrapper` placement under `.chat-home.hero-layout`.

- **Before changing hero layout**, check both `+page.svelte` and `ChatView.svelte` — they must agree on composer placement or FLIP is required.

## Recommended follow-up (not yet implemented)

For pixel-perfect hero → dock when grid and absolute layouts diverge, use **FLIP** on the composer wrapper:

1. `getBoundingClientRect()` on `.composer-wrapper` while still hero/centered.
2. Apply dock layout (remove `.centered`, set dock `bottom`).
3. Measure again; set `transform` to the delta; animate `transform` to `none` over `--duration-flight`.

## Verification

1. Empty session → composer stays centered until first send; no dock during `loadTranscript`.
2. First send → user bubble, avatar, composer, and thread clip animate together (~560ms); no second dock when the stream ends.
3. Open session with existing messages → composer docked once after transcript load, no hero flash.
4. Home → new session with pending message → first turn matches (2) after navigation.
5. Switch between two populated sessions → no hero placeholder; composer stays aligned after each switch.
6. Composer width identical in hero and dock on the same breakpoint.
