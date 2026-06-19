# First-turn `transition:fly` bugs (missing transitions + hidden user message)

**Date:** 2026-06-14  
**Components:** `ChatView.svelte`, `ChatThread.svelte`, `chat.svelte.ts`, `reducers/chat.ts`

## Symptom A: Row transitions missing after the first message

After sending the first message and entering the chat thread, new rows (assistant replies, tool cards, follow-up user bubbles) appeared instantly with no fly-in animation. The hero → thread fade still worked; only per-row entrances in `ChatThread` were missing.

## Symptom B: User message hidden when reasoning starts

On the first turn, when the assistant began reasoning, the user's message briefly vanished or flickered out of view.

## Root causes

### 1. First-turn layout stayed active for the whole stream

`awaitingFirstAssistant` was cleared in `onFirstTurnComplete`, which runs when `chatStore.send()` finishes — often many seconds after the 560ms first-turn flight.

While `awaitingFirstAssistant` was true:

- The first assistant reply was rendered in a **first-turn slot** under the user row, not in the normal `{#each}` assistant branch.
- That slot has no row-level `in:fly` / `transition:fly`.
- The real assistant item was **excluded** from the keyed list (`!(awaitingFirstAssistant && item.id === firstAssistantId && firstUserId)`).

So for most of the first reply (and sometimes the entire turn), assistant and tool rows either bypassed the animated list path or mounted in a context where row intros never ran.

### 2. Assistant/tool rows used `transition:fly` instead of `in:fly`

The first user bubble had already been fixed to use `in:fly` because `transition:fly` on a row inside `{#each}` can re-run outro + intro when sibling blocks mount (e.g. the first-turn assistant slot). Assistant, tool, status, and error rows still used `transition:fly`, which is vulnerable to the same pattern: intros do not run reliably when new keyed siblings appear in the same list.

Using `in:fly` limits animation to mount-only, which is what we want for chat rows.

### 3. `transition:fly` on user row re-ran when assistant slot mounted

`transition:fly` on the user row re-ran when the first-turn assistant slot mounted as a sibling inside the same `{#each}` iteration. Svelte played outro + intro on the user row, making the bubble look like it disappeared.

### 4. `revealStagedUser()` mutated items in place

`revealStagedUser()` mutated items in place while the reducer replaced items with fresh clones on each stream event, which could reset `reveal: false` on the staged user.

## Fix

1. **`ChatView.svelte`** — Do **not** clear `awaitingFirstAssistant` when the flight overlay completes. Keep that flag until the first stream finishes (`onFirstTurnComplete`). Flight completion only sets `firstTurnFlightDone`.

2. **`ChatThread.svelte`** — Split first-turn layout into two helpers:
    - **`showFirstTurnAvatarSlot()`** — Keep the avatar placeholder under the user row until the first assistant row is ready in the normal list (during flight, and after flight until `showAssistantRow` is true).
    - **`firstAssistantInNormalList()`** — Once `firstTurnFlightDone && showAssistantRow(item)`, render the first assistant in the keyed `{#each}` list so `in:fly` runs.

3. **`ChatThread.svelte`** — Replace row-level `transition:fly` with `in:fly` for assistant, tool, status, and error rows.

4. **User bubble** — Use **`in:fly`** on the user bubble only (mount once), not `transition:fly` on the row wrapper.

5. **`revealStagedUser()`** — Replace the items array immutably.

6. **`cloneItem`** — Preserve `reveal: false` until explicitly revealed (`reveal ?? true` for history items without the field).

### Avatar gap regression (same session)

An earlier attempt cleared `awaitingFirstAssistant` on flight done. That removed the avatar slot immediately while the first assistant row was not yet in the list (no reasoning/text yet), so the avatar vanished for a noticeable gap. The slot/list handoff helpers above fix that without tying layout to stream end.

## How to avoid regressions

- **First-turn state:** `firstTurnFlightDone` tracks the **flight** (~560ms). `awaitingFirstAssistant` tracks the **first turn** until the stream ends. Do not conflate them — clearing `awaitingFirstAssistant` on flight done removes the avatar slot before the assistant row exists.

- **Avatar handoff:** Use an explicit slot-vs-list rule (`showFirstTurnAvatarSlot` / `firstAssistantInNormalList`) so the avatar stays visible between flight completion and the first assistant `showAssistantRow`.

- **Chat row motion:** Prefer `in:fly` (or `in:fade`) on the element that should animate once when it enters the transcript. Avoid `transition:fly` on rows inside a keyed `{#each}` unless you explicitly need outro animation on remove.

- **Dual render paths:** If an item can render in a "slot" and in the main list, document which path owns transitions. Do not leave the slot active for the full stream if the list path is the one that should animate.

- **Do not put `transition:fly` on elements inside a keyed `{#each}` block** that gain new sibling DOM when streaming starts. Use `in:fly` for entrance-only motion.

## Verification

1. New chat → send first message → after the flight, reasoning/tools/text should fly in as they arrive.
2. Send a second message → user bubble and new assistant reply should fly in.
3. First user bubble should not disappear when reasoning starts.
