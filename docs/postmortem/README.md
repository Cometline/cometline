# Postmortems

Short write-ups of non-obvious bugs in the Cometline UI layer: symptoms, root cause, fix, and how to avoid regressions. Read these before changing `chat.svelte.ts`, `reducers/chat.ts`, or `ChatThread.svelte`.

| Date | Topic | File |
|------|--------|------|
| 2026-06-14 | User message vanishes when reasoning starts | [user-message-hidden-during-reasoning.md](./user-message-hidden-during-reasoning.md) |
| 2026-06-14 | Assistant/reasoning text only appears after stream ends | [streaming-ui-not-live-updating.md](./streaming-ui-not-live-updating.md) |

## When to add a postmortem

Add one when:

- The bug was caused by Svelte reactivity, transitions, or store update patterns rather than backend/SSE logic.
- The fix is easy to revert accidentally (e.g. swapping `in:fly` back to `transition:fly`).
- Future contributors would otherwise repeat the same mistake.

Keep entries concise: symptom → cause → fix → prevention checklist.
