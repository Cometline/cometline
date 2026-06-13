# User message hidden when reasoning starts

**Date:** 2026-06-14  
**Components:** `ChatThread.svelte`, `chat.svelte.ts`, `reducers/chat.ts`

## Symptom

On the first turn, when the assistant began reasoning, the user's message briefly vanished or flickered out of view.

## Root cause

1. **`transition:fly` on the user row** re-ran when the first-turn assistant slot mounted as a sibling inside the same `{#each}` iteration. Svelte played outro + intro on the user row, making the bubble look like it disappeared.

2. **`revealStagedUser()` mutated items in place** while the reducer replaced items with fresh clones on each stream event, which could reset `reveal: false` on the staged user.

## Fix

- Use **`in:fly`** on the user bubble only (mount once), not `transition:fly` on the row wrapper.
- Make **`revealStagedUser()`** replace the items array immutably.
- In **`cloneItem`**, preserve `reveal: false` until explicitly revealed (`reveal ?? true` for history items without the field).

## How to avoid regressions

Do not put `transition:fly` on elements inside a keyed `{#each}` block that gain new sibling DOM when streaming starts. Use `in:fly` for entrance-only motion.
