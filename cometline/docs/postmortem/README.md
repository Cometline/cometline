# Postmortems

Short write-ups of non-obvious bugs in the Cometline UI layer (and cross-cutting CometMind runtime issues that surface in chat): symptoms, root cause, fix, and how to avoid regressions. Read these before changing `chat.svelte.ts`, `reducers/chat.ts`, `ChatView.svelte`, `ChatThread.svelte`, `chat-turn-queue.ts`, `Composer.svelte`, `src/lib/skills/slash-commands.ts`, `keyboard-shortcuts.ts`, `HeroComposerFrame.svelte`, `hero-composer-appearance.ts`, `settings.svelte.ts`, `SettingsCometMindPanel.svelte`, `src/lib/client/cometmind.ts`, `src/lib/generated/cometmind-api/`, `cometmind/openapi.yaml`, `cometmind/internal/event/event.go`, `cometmind/internal/contract/`, `cometmind/internal/gateway/discord/adapter.go`, or Electron IPC in `preload.cjs` / `main.cjs`.

| Date       | Topic                                                   | File                                                                                                 |
| ---------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 2026-06-14 | Assistant avatar missing after tool use                 | [assistant-avatar-missing-after-tool-use.md](./assistant-avatar-missing-after-tool-use.md)           |
| 2026-06-14 | macOS traffic-light sidebar transition                  | [macos-traffic-light-sidebar-transition.md](./macos-traffic-light-sidebar-transition.md)             |
| 2026-06-14 | Assistant/reasoning text only appears after stream ends | [streaming-ui-not-live-updating.md](./streaming-ui-not-live-updating.md)                             |
| 2026-06-14 | First-turn `transition:fly` bugs (missing transitions + hidden user message) | [first-turn-fly-transition-bugs.md](./first-turn-fly-transition-bugs.md)           |
| 2026-06-14 | Hero composer glow layering and animation               | [hero-composer-glow-layering.md](./hero-composer-glow-layering.md)                                   |
| 2026-06-14 | Composer phase and positioning bugs (hero dock jank + stuck hero) | [composer-phase-and-positioning.md](./composer-phase-and-positioning.md)                   |
| 2026-06-14 | Duplicate user message on rapid submit                  | [duplicate-user-message-on-rapid-submit.md](./duplicate-user-message-on-rapid-submit.md)             |
| 2026-06-14 | Release and update pipeline bugs (packaged 404 + draft blocks auto-update) | [release-and-update-pipeline.md](./release-and-update-pipeline.md)                 |
| 2026-06-15 | First-turn transcript invisible after user bubble flight | [first-turn-transcript-invisible.md](./first-turn-transcript-invisible.md)                           |
| 2026-06-16 | Fetch models IPC fails with DataCloneError                | [fetch-models-data-clone-error.md](./fetch-models-data-clone-error.md)                               |
| 2026-06-16 | Memory subsystem bugs (embedding settings + extract wrong provider) | [memory-subsystem-bugs.md](./memory-subsystem-bugs.md)                                     |
| 2026-06-16 | Shift+Enter in composer sends instead of newline          | [composer-shift-enter-sends-instead-of-newline.md](./composer-shift-enter-sends-instead-of-newline.md) |
| 2026-06-16 | macOS tray icon oversized and gray                        | [macos-tray-icon-oversized-and-gray.md](./macos-tray-icon-oversized-and-gray.md)                       |
| 2026-06-16 | Slash commands (Cometline + Discord gateway)              | [slash-commands-cometline-and-discord.md](./slash-commands-cometline-and-discord.md)                 |
| 2026-06-16 | OpenAPI contract codegen (ISSUE-5)                        | [openapi-contract-codegen.md](./openapi-contract-codegen.md)                                         |
| 2026-06-17 | Forked session HTTP 400 from stale tool_call IDs          | [forked-session-tool-call-id-mismatch.md](./forked-session-tool-call-id-mismatch.md)                 |
| 2026-06-17 | Session switch slow first load and stuck "no messages"    | [session-switch-slow-and-stuck-load.md](./session-switch-slow-and-stuck-load.md)                     |
| 2026-06-17 | Session switch: in-flight response lost, stuck, or replayed | [session-switch-in-flight-response-lost-and-rerender.md](./session-switch-in-flight-response-lost-and-rerender.md) |
| 2026-06-17 | Streaming avatar disappears and spinner jumps between turns | [streaming-avatar-disappears-spinner-jumps.md](./streaming-avatar-disappears-spinner-jumps.md)       |

## When to add a postmortem

Add one when:

- The bug was caused by Svelte reactivity, transitions, or keyed `{#each}` behavior
- The fix is non-obvious without reading the component tree
- A future refactor could easily reintroduce the same failure mode
- You add or change slash commands, Discord Application Commands, or skill write/export/delete APIs
- You change the HTTP/SSE wire contract, OpenAPI spec, or generated API clients (`make generate`)
