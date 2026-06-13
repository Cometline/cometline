# User message vanishes when agent starts reasoning

**Date:** 2026-06-14  
**Area:** `ChatThread.svelte`, `chat.svelte.ts`, `reducers/chat.ts`  
**Symptom:** After the user sends a message, the user bubble disappears when the assistant begins reasoning, then reappears later (often when the final reply or flight animation completes).

## Root cause

Two separate issues compounded:

### 1. `transition:fly` re-ran when a sibling node mounted

On the first turn, the user row and the first-assistant slot live in the **same `{#each}` fragment**:

```svelte
{#if item.type === 'user'}
  <div class="row user-row" transition:fly={...}>...</div>
  {#if awaitingFirstAssistant && item.id === firstUserId}
    <div class="row assistant-row">...</div>
  {/if}
{/if}
```

When `reasoning_start` / `reasoning_delta` arrives, `showAssistantRow(firstAssistantItem)` becomes true and the assistant slot mounts **next to** the user row. Svelte treated that as a fragment update and re-ran the user row’s **`transition:fly`** (out + in). The fly-out looks like the message disappearing; the fly-in looks like it coming back.

`transition:` runs on **intro and outro**. `in:fly` runs **only on mount**.

### 2. Staged user `reveal` fought streaming clones

First-turn flight uses `stageUser()` → `reveal: false` + `flight-hidden` until `revealStagedUser()` runs.

`revealStagedUser()` originally **mutated** the user object in place, then called `notifyItems()`. Each stream event runs `reduceChatState()`, which **clones the entire items array**. In-place mutations on the pre-clone object could be dropped on the next event, leaving `reveal: false` and the bubble invisible (`opacity: 0`) until something else forced a refresh.

## Fix

1. **`ChatThread.svelte`** — use `in:fly` on the **bubble**, not `transition:fly` on the row:

   ```svelte
   <div class="bubble user-bubble" in:fly={item.reveal === false ? undefined : USER_ROW_IN}>
   ```

2. **`chat.svelte.ts`** — `revealStagedUser()` replaces the array immutably:

   ```ts
   items = items.map((item, i) =>
     i === revealIndex && item.type === 'user' ? { ...item, reveal: true } : item
   );
   ```

3. **`reducers/chat.ts`** — preserve `reveal` when cloning user items:

   ```ts
   if (item.type === 'user') {
     return { ...item, reveal: item.reveal ?? true };
   }
   ```

## Prevention checklist

- [ ] Do **not** put `transition:` on a node that shares a multi-root `{#each}` block with conditional siblings that appear mid-stream.
- [ ] Prefer **`in:`** for one-shot entrance; reserve **`transition:`** for elements that truly mount/unmount.
- [ ] Never mutate chat items in place for visibility flags; always assign a new `items` array (see [streaming-ui-not-live-updating.md](./streaming-ui-not-live-updating.md)).
- [ ] When adding first-turn layout, consider rendering the assistant slot **outside** the user `{#each}` branch if fragment churn becomes hard to reason about.

## Related code

- First-turn flight: `FirstTurnFlight.svelte`, `revealStagedUser()` timing vs `reasoning_*` events
- User staging: `chatStore.stageUser()` / `skipUser` in `start-chat.ts`
