# Assistant/reasoning text only updates after stream ends

**Date:** 2026-06-14  
**Area:** `chat.svelte.ts`, `reducers/chat.ts`, `ChatThread.svelte`  
**Symptom:** Reasoning and final assistant reply appear all at once when the turn finishes; no token-by-token updates in the thread during SSE streaming. Backend and SSE parser were fine (CometMind flushes each event).

## Root cause

The reducer and store were structurally correct (`reduceChatState` returns a new `items` array each event), but the **renderer did not reliably re-run** on every stream delta.

1. **`notifyItems()` was only called in `send()`’s `finally` block**, not after each `applyEvent()`. The comment on `notifyItems()` said it existed “for streaming updates”, but the hot path never invoked it during the event loop.

2. **`$state` deep proxy on a large `items` array** replaced every ~50–200 ms during streaming. Using **`$state.raw`** plus explicit array replacement is the intended pattern when the whole list is reassigned per event.

3. **In-place mutations on cloned assistant objects** (`text += delta`, `host.reasoning = { ... }`) without replacing the item in the array made it harder to reason about referential change. Fixed by **`publishAssistant()`** — spread into a new object and `items[index] = next` on each `text_delta` / `reasoning_delta`.

## Fix

**`chat.svelte.ts`**

```ts
let items = $state.raw<ChatItem[]>([]);

function applyEvent(...) {
  const reduced = reduceChatState(...);
  items = reduced.items;
  // ...
  notifyItems(); // items = items.slice() — after EVERY stream event
}
```

**`reducers/chat.ts`**

- `publishAssistant(next)` replaces the assistant entry in `items` instead of mutating fields on the cloned reference only.
- Tool results similarly replace the tool object in the array.

**`ChatThread.svelte`**

- `let threadItems = $derived(chatStore.items)` for a clear reactive subscription in the template `{#each}`.

## Prevention checklist

- [ ] Any handler that applies stream events must end with **`items = …`** (new array reference). Call **`notifyItems()`** after each `applyEvent()` unless you have proof `$state.raw` assignment alone is sufficient.
- [ ] Prefer **`$state.raw`** for `ChatItem[]` when updates are always full-array replacements.
- [ ] For assistant/tool rows, **replace the item in the array** when text/reasoning/output changes during streaming.
- [ ] Debug: `localStorage.setItem('cometline.debug.chat', '1')` and watch `store:stream-event` logs — `after` should grow on every delta while streaming.

## Not the problem

- CometMind SSE buffering (`server.go` calls `Flush()` per event).
- `ChatThread` `{#each}` keys — `(item.id)` is correct; do not key on text length unless you intentionally want remounts.

## Related

- [user-message-hidden-during-reasoning.md](./user-message-hidden-during-reasoning.md) — `reveal` flag + clone interaction
