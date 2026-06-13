# Streaming UI not live-updating

**Date:** 2026-06-14  
**Components:** `chat.svelte.ts`, `reducers/chat.ts`

## Symptom

Assistant text and reasoning appeared only after the stream finished, even though SSE events were arriving.

## Root cause

`chatStore.items` used in-place mutation during streaming. Svelte 5 did not detect deep mutations on the items array, so the thread did not re-render until the stream ended and a full replace occurred.

## Fix

- Store items with **`$state.raw`** and call **`notifyItems()`** (`items = items.slice()`) after every `applyEvent()`.
- Use **`publishAssistant()`** in the reducer to replace assistant objects immutably instead of mutating nested fields on shared references.

## How to avoid regressions

When streaming into a list rendered by `{#each}`, ensure each event produces a new array reference (or new item references) that Svelte can track. Do not rely on mutating objects inside `$state.raw` arrays without notifying.
