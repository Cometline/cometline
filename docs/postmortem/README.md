# Postmortems

Short write-ups of non-obvious bugs in the Cometline UI layer: symptoms, root cause, fix, and how to avoid regressions. Read these before changing `chat.svelte.ts`, `reducers/chat.ts`, `ChatView.svelte`, or `ChatThread.svelte`.

| Date       | Topic                                                   | File                                                                                                 |
| ---------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 2026-06-14 | User message vanishes when reasoning starts             | [user-message-hidden-during-reasoning.md](./user-message-hidden-during-reasoning.md)                 |
| 2026-06-14 | Assistant/reasoning text only appears after stream ends | [streaming-ui-not-live-updating.md](./streaming-ui-not-live-updating.md)                             |
| 2026-06-14 | Row transitions missing after the first message         | [chat-transitions-missing-after-first-message.md](./chat-transitions-missing-after-first-message.md) |
| 2026-06-14 | Hero → chat composer dock transition jank               | [hero-composer-dock-transition-jank.md](./hero-composer-dock-transition-jank.md)                     |

## When to add a postmortem

Add one when:

- The bug was caused by Svelte reactivity, transitions, or keyed `{#each}` behavior
- The fix is non-obvious without reading the component tree
- A future refactor could easily reintroduce the same failure mode
