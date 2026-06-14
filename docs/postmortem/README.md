# Postmortems

Short write-ups of non-obvious bugs in the Cometline UI layer: symptoms, root cause, fix, and how to avoid regressions. Read these before changing `chat.svelte.ts`, `reducers/chat.ts`, `ChatView.svelte`, `ChatThread.svelte`, `chat-turn-queue.ts`, `Composer.svelte`, `HeroComposerFrame.svelte`, or `hero-composer-appearance.ts`.

| Date       | Topic                                                   | File                                                                                                 |
| ---------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 2026-06-14 | Assistant avatar missing after tool use                 | [assistant-avatar-missing-after-tool-use.md](./assistant-avatar-missing-after-tool-use.md)           |
| 2026-06-14 | macOS traffic-light sidebar transition                  | [macos-traffic-light-sidebar-transition.md](./macos-traffic-light-sidebar-transition.md)             |
| 2026-06-14 | User message vanishes when reasoning starts             | [user-message-hidden-during-reasoning.md](./user-message-hidden-during-reasoning.md)                 |
| 2026-06-14 | Assistant/reasoning text only appears after stream ends | [streaming-ui-not-live-updating.md](./streaming-ui-not-live-updating.md)                             |
| 2026-06-14 | Row transitions missing after the first message         | [chat-transitions-missing-after-first-message.md](./chat-transitions-missing-after-first-message.md) |
| 2026-06-14 | Hero composer glow layering and animation           | [hero-composer-glow-layering.md](./hero-composer-glow-layering.md)                                   |
| 2026-06-14 | Hero → chat composer dock transition jank               | [hero-composer-dock-transition-jank.md](./hero-composer-dock-transition-jank.md)                     |
| 2026-06-14 | Composer stuck in hero layout after session switch      | [session-switch-composer-stuck-hero.md](./session-switch-composer-stuck-hero.md)                     |
| 2026-06-14 | Duplicate user message on rapid submit                  | [duplicate-user-message-on-rapid-submit.md](./duplicate-user-message-on-rapid-submit.md)             |

## When to add a postmortem

Add one when:

- The bug was caused by Svelte reactivity, transitions, or keyed `{#each}` behavior
- The fix is non-obvious without reading the component tree
- A future refactor could easily reintroduce the same failure mode
