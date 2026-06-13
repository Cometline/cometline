# Cometline Frontend Phased Development Plan

Cometline is the SvelteKit frontend and desktop shell for CometMind. It should make the local agent runtime feel like a polished personal AI app while keeping all agent logic, persistence, model calls, tools, permissions, and memory inside CometMind.

## North Star

Build a SvelteKit UI that can run inside Electron for local desktop distribution and later deploy as a hosted web frontend with minimal component changes. The UI consumes CometMind through REST/SSE and uses Electron IPC only for native shell operations.

## Frontend Architecture Decision

Use SvelteKit as the application framework, even for the desktop app. Phase 1 packages the SvelteKit app with Electron and serves/renders it against the local CometMind process. Future cloud mode can reuse the same route tree with a remote CometMind-compatible API.

Desktop mode:

```text
Electron main process
  -> starts CometMind Go binary
  -> waits for http://127.0.0.1:7700/api/v1/health
  -> opens SvelteKit renderer
  -> exposes minimal IPC for native-only features

SvelteKit renderer
  -> calls CometMind REST/SSE directly
  -> never calls LLM providers
  -> never executes tools
  -> never stores raw provider secrets
```

## Load-Bearing Boundaries

- CometMind is the brain; Cometline is a UI and desktop shell.
- REST/SSE is the runtime contract boundary.
- Electron IPC is only for app version, process status, restart, native dialogs, tray/hotkeys, and notifications.
- Renderer state is cache/UI state, not source of truth.
- Permission prompts must be generated from CometMind state, not guessed from tool names in the UI.

## Phase 0 — Scaffold SvelteKit Desktop Shell

Goal: turn the docs-only repo into a runnable SvelteKit + Electron project.

Day-by-day:

1. Create SvelteKit project structure with TypeScript, Tailwind, Vitest, and Svelte 5 runes.
2. Add Electron main/preload processes with `contextIsolation: true` and `nodeIntegration: false`.
3. Add CometMind process manager that resolves dev/prod binary paths.
4. Add health polling and startup error screen.
5. Add typed API client generated from or manually aligned with `cometmind/openapi.yaml`.
6. Add CI commands: typecheck, unit test, build.

Exit criteria:

- `pnpm dev` opens a desktop window.
- The app can show CometMind connected/disconnected state.
- No renderer code imports Node APIs directly.

## Phase 1 — Session List And Chat MVP

Goal: implement the current OpenAPI surface cleanly.

Day-by-day:

1. Build route shell: `/`, `/sessions/[id]`, `/settings`.
2. Implement workspace picker or default workspace selection for session creation/listing.
3. Implement session list from `GET /api/v1/sessions`.
4. Implement session creation with `POST /api/v1/sessions`.
5. Implement transcript rendering from `GET /api/v1/sessions/{id}/messages`.
6. Implement message send with `POST /api/v1/sessions/{id}/message` and `ReadableStream` SSE parsing.
7. Render `text_delta`, `reasoning_start`, `reasoning_delta`, `tool_call`, `tool_result`, `step_finish`, `error`, and `done` events.
8. Implement abort button with `POST /api/v1/sessions/{id}/abort`.

Exit criteria:

- User can create a session, send a message, watch streaming output, see tool cards, and abort a run.
- UI recovers from dropped SSE with a clear retry/reconnect state.

## Phase 2 — Permission UX

Goal: make local tool execution understandable and controllable.

Day-by-day:

1. Add UI types for CometMind permission events once Phase 1 backend events exist.
2. Render pending tool calls with risk class, command/path summary, and exact requested input.
3. Add approve once, approve for session, always allow, and deny controls.
4. Add a run-paused state while the backend waits for permission.
5. Add permission history in the tool card expanded view.
6. Add settings screen for allowlists/blocklists once backend API exists.
7. Add tests for permission cards, denial states, and resumed streaming.

Exit criteria:

- User can safely approve or deny impactful tool calls without leaving chat.
- UI never executes or simulates tool approval locally; all decisions go through CometMind.

## Phase 3 — Provider, Model, And Secrets Settings

Goal: expose provider management without leaking credentials into the renderer.

Day-by-day:

1. Add settings routes for providers, default model, max tokens, and max steps.
2. Consume `GET /api/v1/providers`, `GET /api/v1/models`, and `GET /api/v1/config` once backend adds them.
3. Add provider credential forms that submit secrets only to CometMind.
4. Add provider test flow with loading, success, invalid key, rate limit, and network error states.
5. Ensure no raw secrets are stored in localStorage, Svelte stores, logs, screenshots, or error toasts.
6. Add tests for redacted provider UI states.

Exit criteria:

- User can configure and test providers from the app.
- Refreshing the app never exposes raw secrets.

## Phase 4 — Memory UI

Goal: make memory visible and editable before automatic learning becomes prominent.

Day-by-day:

1. Add memory settings: enabled state, scopes, max injected items, and write approval mode.
2. Add memory list/search/edit/delete screens.
3. Show memory source, scope, confidence, and last-used metadata.
4. Render `memory_injected` events in chat as collapsible context chips.
5. Add staged memory review UI for proposed writes after backend supports extraction.
6. Add tests for memory CRUD forms and staged write review.

Exit criteria:

- User can answer: “What does Cometline remember about me, and why did this run use it?”
- Automatic memory writes are reviewable before becoming active.

## Phase 5 — Skills UI

Goal: expose procedural memory as manageable user-facing assets.

Day-by-day:

1. Add skills index route with installed/enabled/disabled state.
2. Add skill detail view that renders `SKILL.md` metadata and body.
3. Add skill invocation UI through slash command autocomplete.
4. Add staged skill update review with diff viewer.
5. Add create/import/delete flows once backend APIs exist.
6. Add tests for malformed skills, disabled skills, and staged update review.

Exit criteria:

- User can browse, invoke, enable, disable, and review skills from the UI.
- Agent-created skill changes require explicit review.

## Phase 6 — Artifacts And Rich Output

Goal: support generated outputs beyond plain chat text.

Day-by-day:

1. Define artifact card components for files, markdown, HTML, SVG, Mermaid, images, and logs.
2. Add artifact preview route or side panel.
3. Add sandboxed iframe preview for HTML artifacts.
4. Add download/open-in-folder actions through Electron IPC.
5. Add tests for safe rendering and unsupported artifact types.

Exit criteria:

- Tool and agent outputs can become durable artifacts.
- HTML previews are sandboxed and cannot access Node/Electron APIs.

## Phase 7 — Scheduler UI

Goal: let users create and inspect background automations safely.

Day-by-day:

1. Add jobs list route with enabled/disabled, next run, last run, and status.
2. Add job creation form for prompt, schedule, target space/session, and permission policy.
3. Add run history view with logs and tool decisions.
4. Add pause/resume/run-now/delete actions.
5. Add tests for invalid schedules and dangerous headless permission policies.

Exit criteria:

- Users can manage scheduled automations without editing config files.
- Every background run has visible history and safety policy.

## Phase 8 — Subagents And Coding Delegation UI

Goal: make delegated workstreams understandable.

Day-by-day:

1. Add child task cards inside the parent chat timeline.
2. Add subagent progress states: queued, running, waiting, completed, failed, cancelled.
3. Add expand view for delegated transcript, tool calls, and final summary.
4. Add cancellation controls for child tasks.
5. Add tests for nested progress rendering and failed child tasks.

Exit criteria:

- Coding delegation feels like an integrated part of the conversation.
- User can inspect what the delegated agent did before accepting results.

## Phase 9 — Browser And Computer-Use UX

Goal: make browser automation visible, permissioned, and hard to misuse.

Day-by-day:

1. Add browser session cards with current URL/title/screenshot preview.
2. Show requested browser actions before mutating actions such as click/type/upload.
3. Add domain allowlist/blocklist settings.
4. Add evidence trail: page read, screenshot, action, result.
5. Add tests for blocked domains, permission prompts, and screenshot rendering.

Exit criteria:

- User can see where the agent is acting and stop it before risky browser actions.

## Phase 10 — Gateway/Cloud Readiness

Goal: prepare the UI architecture for non-desktop deployments.

Day-by-day:

1. Move API base URL and mode detection into a single environment/config module.
2. Separate desktop-only components behind capability checks.
3. Add auth placeholders for hosted mode, but keep desktop mode auth-free on localhost.
4. Add responsive layouts for mobile/tablet if gateway chat surfaces become important.
5. Add cloud build target once backend API supports remote deployment.

Exit criteria:

- The SvelteKit app can run as desktop local mode or remote web mode with clear capability differences.

## Recommended Immediate Build Order

1. Phase 0: SvelteKit + Electron scaffold.
2. Phase 1: session/chat MVP against existing OpenAPI.
3. Phase 2: permission UX once CometMind permission events exist.
4. Phase 3: provider/config/secrets settings.
5. Phase 4: memory UI.
6. Phase 5: skills UI.

Do not build rich artifacts, scheduler, browser automation, or gateway UI before permission and secrets UX are stable.
