# Composer stuck in hero layout after session switch

**Date:** 2026-06-14  
**Components:** `ChatView.svelte`, `ChatThread.svelte`, `chat.svelte.ts`, `session.svelte.ts`, `Composer.svelte`

## Symptom

After switching between chat sessions (especially from home or another session), the composer looked wrong:

- Input placeholder stayed **"Type something. Anything."** (hero copy) instead of dock copy **"Type something…"**
- Composer sat **bottom-left**, narrower than the thread column above it
- Thread messages were centered; composer was not aligned with them
- Bottom of the composer could clip against the chat panel edge

The thread fade-in work for session switches was fine; only composer phase/placement regressed.

## Root cause

Session-switch fade-in added early transcript loading in `$effect.pre` plus a dedupe guard in `loadTranscript()`. That combined badly with the existing `onMount` composer dock/center logic.

### 1. `loadTranscript()` returned before the fetch finished

```javascript
// broken guard
if (sessionID === nextSessionID && isLoading) return;
```

`$effect.pre` started the load and set `isLoading = true`. `onMount` called `loadTranscript()` again, hit the guard, and **returned immediately** (resolving `undefined`, not a pending promise).

### 2. `onMount` dock/center ran on an empty transcript

```javascript
// broken pattern
void chatStore.loadTranscript(sessionId).then(() => {
	if (chatStore.items.length > 0) shellStore.dockComposer();
	else shellStore.centerComposer();
});
```

The `.then()` ran while `items` were still `[]`, so it called **`centerComposer()`** even though the session had messages on the way.

`shellStore.composerPhase` stayed `'centered'` → `Composer` kept `variant="hero"` while the thread showed a loaded conversation.

### 3. Hero positioning on a session thread surface

With `composerPhase === 'centered'` on `ChatView`:

- `.composer-wrapper.centered` used `bottom: var(--composer-hero-bottom)` and `transform: translateY(50%)`
- Thread used docked layout (`thread-shell.docked`)
- Hero and dock placement models diverged → visible horizontal/vertical misalignment

Placeholder `"Type something. Anything."` is a reliable signal: it only renders when `variant === 'hero'`.

### 4. `ChatView` hero grid lacked the home-page composer override

`+page.svelte` resets composer wrapper placement under `.chat-home.hero-layout`:

```css
.chat-home.hero-layout .composer-wrapper {
	position: relative;
	bottom: auto;
	transform: none;
	/* ... */
}
```

`ChatView.svelte` did not have this override. During brief hero-layout windows (first paint before session bind, or centered phase during load), absolute + `translateY(50%)` on the wrapper fought the grid and made alignment worse.

## Fix (applied)

### 1. In-flight `loadTranscript` promise

`chat.svelte.ts` tracks `loadPromise` / `loadPromiseSession`. If the same session is already loading, callers await the existing promise instead of returning early.

### 2. Session-scoped composer phase `$effect` in `ChatView`

Replaced `onMount` `.then()` dock/center with:

```javascript
$effect(() => {
	if (chatStore.sessionID !== sessionId) return;
	if (firstTurnActive) return;

	if (hasVisibleConversation) {
		shellStore.dockComposer();
	} else if (!chatStore.isLoading) {
		shellStore.centerComposer();
	}
});
```

On the **session route**, `hasVisibleConversation` includes `isLoading`, but the UI shows `thread-shell` + loading — not the empty hero. Docking during load is correct here and prevents hero phase from sticking while messages fetch.

`firstTurnActive` is excluded so first-turn flight still starts from centered hero; `onPrepareFlight` docks in sync with the flight.

### 3. `bindSession()` in `$effect.pre`

Before first paint, `chatStore.bindSession(sessionId)` clears stale items from the previous session so the thread never flashes the wrong transcript.

`hasVisibleConversation` also requires `chatStore.sessionID === sessionId`.

### 4. Hero-layout composer override in `ChatView`

Copied the `+page.svelte` `.chat-home.hero-layout .composer-wrapper` rules into `ChatView.svelte` so first-turn hero grid and composer placement agree.

### 5. Session thread fade-in (same change set)

`ChatThread.svelte` wraps loaded messages in a one-shot `in:fade` and suppresses per-row `in:fly` on the initial transcript paint. That is independent of composer phase but shipped in the same session-switch pass.

## Relation to hero-composer-dock postmortem

[hero-composer-dock-transition-jank.md](./hero-composer-dock-transition-jank.md) says:

> **Never dock from `hasVisibleConversation` alone** — it includes `isLoading`.

That rule applies to **empty hero surfaces** (home route, empty session before first send). On **`ChatView` session route**, loading shows the thread shell, not the empty hero — docking when `hasVisibleConversation` is true is intentional and prevents this regression.

| Surface | Loading UI | Dock when `isLoading`? |
| ------- | ---------- | ---------------------- |
| `+page.svelte` (home) | Empty hero | No |
| `ChatView` (session, empty, no items yet) | Thread loading | Yes (brief; then center if transcript empty) |
| `ChatView` (session, items or streaming) | Thread | Yes |

## How to avoid regressions

- **Do not gate `loadTranscript` with a bare `isLoading` return** unless you return the in-flight promise. Otherwise any `await loadTranscript()` or `.then()` after a duplicate call resolves instantly with stale empty `items`.

- **Do not dock/center from `onMount().then(loadTranscript)`** when `$effect.pre` also starts the load. Use one owner for composer phase on the session route (the `$effect` above) plus `onPrepareFlight` for first turn.

- **Placeholder string is a debug signal** — `"Type something. Anything."` means `composerPhase` is still `'centered'` on a session that should be docked.

- **Keep `+page.svelte` and `ChatView.svelte` hero composer CSS in sync** when changing `.composer-wrapper` placement under `.chat-home.hero-layout`.

- **Session switch checklist:** wrong transcript flash → `bindSession` + `sessionID` match; message pop-in → thread fade; composer bottom-left + hero placeholder → composer phase regression (this doc); **blank white thread with docked composer after first send** → [first-turn-transcript-invisible.md](./first-turn-transcript-invisible.md) (`isLoading` stuck / hydrating opacity).

## Verification

1. Home → open session with existing messages → composer docked, centered with thread, placeholder **"Type something…"**.
2. Switch between two populated sessions → no hero placeholder; composer stays aligned after each switch.
3. Empty session → composer centered until first send; no dock during load after transcript resolves empty.
4. New session with pending first message → first turn still flies from hero; `onPrepareFlight` docks with flight.
5. Collapse sidebar → composer and thread remain centered together in `main`.
