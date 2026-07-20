# Thread turn scroll pinning (user message positioning)

**Date:** 2026-07-01  
**Components:** `ChatThread.svelte`, `thread-scroll.ts`, `thread-scroll.svelte.ts`, `thread-turns.ts`, `first-turn-flight.ts`, `UserMessageRow.svelte`

## Background and goals

On follow-up turns (from the second user message onward), we wanted ChatGPT / Claude-style behavior:

1. After the user sends a message, the bubble stays pinned near the **upper fifth** of the viewport.
2. Enough space remains below so the start of the AI reply is visible **without scrolling down first**.
3. **No layout shift** when streaming ends.
4. “Jump to latest” and manual scrolling should not land in an **empty dead zone** with no content.

## Why the existing implementation “did not work”

The codebase already had `userMessageScrollTop()` and `createThreadScroll()`, but in practice pinning almost never behaved as intended. Several issues stacked on top of each other:

### 1. User bubble flight fought the pin scroll

On every follow-up message, `ChatView` runs `UserBubbleFlight` → `flyUserBubble()`, which calls `scrollThreadToBottom()` and scrolls the thread **to the bottom**.

At the same time, `thread-scroll.svelte.ts` reacts to `lastUserId` changes and calls `scrollUserMessageToTop()` to **pin the user message upward**.

Both schedule work via `tick().then(...)` — **whichever runs last wins**. In most cases, “scroll to bottom” overwrote the pin, so the message looked like it simply followed the previous assistant reply.

**Fix (Phase 1):** Skip `scrollThreadToBottom()` when `origin === 'above-composer'` (follow-up turns).

### 2. The bottom spacer almost never activated

`ChatThread` used to append `<div class="thread-bottom-spacer">` when `last item === 'user'`.

But `chatStore.send()` **immediately** pushes an empty assistant placeholder after the user item. By the time the DOM renders, the last item is already `assistant`, so spacer height was 0.

Without extra scroll range, the pin target `scrollTop` often exceeded `scrollHeight - clientHeight`, the browser clamped it, and pinning **failed silently**.

### 3. Pin offset was too small and inconsistent

Follow-ups used 15% of viewport height; an older version used a fixed 128px. The first turn used 24px. In practice, messages still felt glued to the previous turn’s content.

### 4. Spacer removal on stream end → layout shift

We later kept the spacer through `sessionStreaming`, but when streaming stopped the spacer still disappeared. `scrollTop` stayed the same while content height shrank → **visible jump**.

### 5. Spacer caused “Jump to latest” and manual scroll to enter a dead zone

The spacer was real DOM height but not a message. Users could scroll into empty space; “Jump to latest” targeted `scrollHeight` and showed a blank area.

We patched this with `maxContentScrollTop` + clamp, but that conflicted with the scroll range pinning needed — **band-aids on the wrong model**.

## Final solution (Phase 2): Turn wrapper + min-height

Aligned with common industry practice (most ChatGPT-style UIs): **no trailing spacer div** — use a **turn container** to own the empty space.

### Architecture

```
thread-messages
└── .thread-turn                    ← one turn per user message
    ├── UserMessageRow
    ├── FirstTurnAssistantSlot (first turn only)
    └── assistant / tool / memory / …
└── .thread-turn.thread-turn-active ← latest follow-up turn only
    └── min-height: viewport - clearance
```

### Behavior

| When | What happens |
|------|----------------|
| User sends follow-up (turn ≥ 2) | `activeTurnCanvas = true`; that turn gets `min-height` |
| Pin positioning | `scrollIntoView({ block: 'start' })` + `.user-row { scroll-margin-top }` |
| During / after AI stream | `min-height` persists until the **next user message** (not until `sessionStreaming` ends) |
| Session switch / load history | `activeTurnCanvas = false`; old chats get no phantom blank area |
| Jump to latest | Normal `scrollTo(scrollHeight)`; bottom is the natural end of turn content, not a dead zone |

### New / main files

| File | Role |
|------|------|
| [`thread-turns.ts`](../src/lib/conversation/thread-turns.ts) | `groupThreadItemsIntoTurns()`, `activeTurnMinHeight()` |
| [`thread-turns.test.ts`](../src/lib/conversation/thread-turns.test.ts) | Turn grouping and min-height unit tests |
| [`thread-scroll.ts`](../src/lib/conversation/thread-scroll.ts) | Slimmed down: near-bottom, scroll key, `followUpPinScrollMargin()` |
| [`thread-scroll.svelte.ts`](../src/lib/conversation/thread-scroll.svelte.ts) | `scrollIntoView` pin, `activeTurnCanvas`, `activeTurnMinHeight` |
| [`ChatThread.svelte`](../src/lib/components/chat/ChatThread.svelte) | `{#each threadTurns}`, `.thread-turn-active`, spacer removed |
| [`first-turn-flight.ts`](../src/lib/first-turn-flight.ts) | Follow-up no longer calls `scrollThreadToBottom` |
| [`app.css`](../src/app.css) | `--thread-user-pin-*`, `--thread-turn-bottom-clearance` tokens |

### Removed concepts

- `thread-bottom-spacer` DOM node
- `reserveActiveTurnSpace` / `activeTurnBottomSpacerHeight`
- `userMessageScrollTop` / `offsetTopRelativeTo` pin math
- `maxContentScrollTop` / `clampScrollAboveBottomInset` (spacer inset only)

## Design tokens (tunable)

```css
--thread-user-pin-offset-first: 24px;        /* first turn (flight-driven; pin rarely used) */
--thread-user-pin-ratio-followup: 0.18;      /* follow-up upper fifth */
--thread-turn-bottom-clearance: 96px;        /* min-height = viewport - clearance */
```

At runtime, `.thread` sets `--thread-user-pin-offset-followup` (px) for the active turn’s `scroll-margin-top`.

## Takeaways

### 1. Empty space should belong to the turn, not a ghost div at the list tail

Most ChatGPT / Claude-style implementations reserve reply space with **`min-height` on the turn / last message**, not an unrelated spacer after `{#each}`.

That means:

- The semantic “bottom” of `scrollHeight` is still the end of the conversation structure
- Jump to latest and near-bottom checks need no special cases
- Users are less likely to scroll into a region with nothing on screen

**Old model (trailing spacer):**

```
{#each items}  messages…  {/each}
<div class="spacer" />   ← ghost: height without content
```

**New model (turn min-height):**

```
{#each turns}
  <div class="thread-turn" style="min-height: …">
    user + assistant rows
    (empty area = min-height minus content — inside the turn)
  </div>
{/each}
```

### 2. Prefer platform-native pinning

`scrollIntoView({ block: 'start' })` + `scroll-margin-top` is more stable than hand-computed `offsetTop` and custom `scrollTop`, and it conflicts less with layout changes (assistant placeholder, growing stream).

### 3. Only one scroll owner per moment

On follow-up, two systems competed:

- flight’s `scrollThreadToBottom`
- scroll controller’s pin

Clarify **who scrolls when**. First turn: `FirstTurnFlight`. Follow-up pin: `createThreadScroll`; flight must not scroll to the bottom.

### 4. State lifetime must match UX

| State | Should last until |
|-------|-------------------|
| Active turn `min-height` | Next user message (not stream end) |
| `activeTurnCanvas` | Same; cleared on session switch / hydration |
| Pin scroll | Follow-up only (`userMessageCount > 1`), not initial transcript paint |

### 5. Spacer on a flat list is a stepping stone, not the destination

Phase 1 (spacer + inset + clamp) validated the requirement but kept causing:

- layout shift (spacer appear / disappear)
- jump / clamp / pin three-way conflicts
- manual scroll into dead zones

For a durable ChatGPT-level experience, **turn grouping** is effectively required.

### 6. `ChatThread` stays a thin orchestrator; logic lives in pure modules

Per [`FRONTEND_PATTERNS.md`](../FRONTEND_PATTERNS.md):

- `thread-turns.ts` — testable turn grouping
- `thread-scroll.svelte.ts` — scroll controller
- `ChatThread.svelte` — thin `{#each}` + styles

## Verification checklist

1. **Turn 2+:** After send, user bubble near upper fifth; thinking / first tokens visible without scrolling down.
2. **Stream end:** No jump; `min-height` remains until the next user message.
3. **Jump to latest:** Shows the last AI content, not blank space.
4. **Manual scroll:** Should not rest in a message-free dead zone (blank area is inside the turn, semantically “reply canvas”).
5. **Session switch / open old chat:** No extra bottom gap; hydration still scrolls to bottom.
6. **First turn:** Hero → dock flight unchanged.

## Related docs

- [`FRONTEND_PATTERNS.md`](../FRONTEND_PATTERNS.md) — Chat thread controller pattern
- [`composer-phase-and-positioning.md`](./composer-phase-and-positioning.md) — First-turn composer / thread-shell positioning
- [`streaming-avatar-disappears-spinner-jumps.md`](./streaming-avatar-disappears-spinner-jumps.md) — Assistant placeholder timing during stream

## Future improvements

- Unify turn 1 with turn `min-height` + pin; reduce reliance on `scrollThreadToBottom`.
- Optional soft-follow during stream when user is near bottom (ChatGPT also only auto-scrolls when already pinned to bottom).
- Tie `--thread-turn-bottom-clearance` to `--thread-dock-inset` responsively for mini / compact layouts.
