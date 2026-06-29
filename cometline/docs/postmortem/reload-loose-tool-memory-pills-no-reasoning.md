# Reload: reasoning-less tool/memory rows become loose, ungrouped pills

**Date:** 2026-06-21
**Components:** `chat.svelte.ts` (`itemsFromTranscript`), `conversation/thinking-attribution.ts`, `ChatThread.svelte`

## Symptom

In a turn where the model produced **no reasoning**, the UI looked correct during the live SSE stream: `list_dir`, `run_command`, and `Memories used · 5` were all folded into a single collapsible activity group.

But after the user pressed cmd+r to reload the session (rehydrating from the transcript API), the same turn "fell apart": `list_dir`, `run_command`, and `Memories used · 5` each became an independent, ungrouped pill — no longer wrapped by the activity group.

This bug **only happened on reload (rehydration)**, never during the live stream.

## Root cause

The frontend attribution scan `buildThinkingAttribution` (`thinking-attribution.ts`) is a **forward scan**: it only attaches tool/memory rows to an assistant it has **already seen**. So the assistant item must appear **before** its tool/memory items in the array for grouping to work.

- **Live stream path:** the assistant placeholder is created on the first event (`text_delta` / `reasoning_start` / `tool_call`), so the assistant always precedes its tools/memory → grouping works.

- **Reload path:** the backend `transcript.go` persists an assistant message in the order `reasoning → memory → tool → assistant text`. When the turn has **no reasoning**, the order becomes `memory → tool → tool → assistant text`. The old `itemsFromTranscript` only lazily created the assistant placeholder on text/reasoning rows (`ensureAssistant` was called only by `appendAssistantText` / `appendReasoning`). The `memory` and `tool` branches never called `ensureAssistant`.

    The resulting array was `[user, memory, tool, tool, assistant]` — the assistant ended up **last**. So during the `buildThinkingAttribution` scan:
    - The `tool` branch requires `currentAssistantId` to already be set (`thinking-attribution.ts` line `else if (item.type === 'tool' && currentAssistantId)`), but the assistant hadn't appeared yet → both tools were **never attributed** → rendered as loose, ungrouped pills.
    - Memory was eventually attached to the assistant via the `pendingMemories` mechanism, but since the assistant had no reasoning and the tools weren't grouped, the whole turn still looked like scattered pills.

In short: **the assistant placeholder's creation timing on reload did not match the live stream ordering**, so the attribution scan missed it.

## Fix

In `itemsFromTranscript` (`chat.svelte.ts`), the `tool` and `memory` branches now call `ensureAssistant(i)` **before** pushing, forcing the assistant placeholder to appear **before** its tool/memory rows in the output array — matching the live stream order:

- `tool` branch: `const host = ensureAssistant(i)` first, then compute `afterSegment` from `host`, then push the tool.
- `memory` branch: `ensureAssistant(i)` first, then push the memory card.

The rehydrated array becomes `[user, assistant, memory, tool, tool]`. `buildThinkingAttribution` now attributes memory/tools to the assistant; `buildAssistantTimeline` produces `['memory', 'tool', 'tool']`; `shouldGroupAssistantTimeline` returns `true` → the whole turn is wrapped by `AssistantActivityGroup` again, consistent regardless of whether the model produced reasoning.

(No changes needed in the backend `transcript.go`, the reducer, or the live stream path.)

Added a regression test `groups reasoning-less memory and tools under the assistant on reload` (`chat.svelte.test.ts`): reloads a `memory → tool → tool → assistant` transcript and asserts that the assistant index precedes memory/tool, the timeline is `['memory','tool','tool']`, grouping is triggered, and `toolIdsInBuffer.size === 2` / `memoryIdsInBuffer.size === 1` (no loose pills).

### Follow-up fix: `each_key_duplicate`

The first version of the fix introduced a new bug: when the first sub-row of a turn (e.g. memory) sat at loop index `i`, `ensureAssistant(i)` created the assistant placeholder with id `history-${i}`, and `itemFromTranscript(item, i)` for the memory at the same index also produced `history-${i}` → two items with the same key → Svelte's keyed `{#each}` in `ChatThread.svelte` threw `each_key_duplicate: duplicate key 'history-1'`.

Fix: auto-created assistant placeholders now use a distinct prefix id `history-assistant-${index}`, which lives in a separate namespace from the transcript rows' `history-${index}` (each index is unique within its own namespace), eliminating collisions entirely. Added a "all item ids are unique" assertion to the regression test (`new Set(ids).size === ids.length`), because the original test suite didn't catch this duplicate-key issue.

Verified: `pnpm run check` = 0 errors / 0 warnings; `pnpm run test` = 287 passed.

## How to avoid regressions

- **On reload (rehydration), the assistant placeholder must be emitted before all of its sub-rows (reasoning / memory / tool / subagent)**, in the same order as the live stream path. Any new sub-row type added to `itemsFromTranscript` should call `ensureAssistant(i)` before pushing.
- Remember that `buildThinkingAttribution` is a **forward scan**: it can only attribute to an assistant it has already seen. When modifying attribution or reordering logic, always test with both "with reasoning" and "without reasoning" turns — reasoning-less turns are the case that exposes ordering bugs.
- If the backend `transcript.go` ever changes the row order for an assistant message (reasoning → memory → tool → text), re-check the frontend rehydration and attribution assumptions.
- **When producing multiple items from the same loop index, ensure their ids never collide** (auto-created placeholders use a distinct prefix like `history-assistant-${index}`). Keyed `{#each}` throws on duplicate keys, and this is easy to miss in existing tests — reordering logic tests should also assert "all item ids are unique".
