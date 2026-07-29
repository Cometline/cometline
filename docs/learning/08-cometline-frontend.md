# 08 — cometline Frontend (SvelteKit)

> **Prerequisite:** [07-cometline-desktop.md](./07-cometline-desktop.md)  
> **Next:** [09-contracts-codegen.md](./09-contracts-codegen.md)

## Purpose

The renderer owns chat UI, jobs UI, settings UI, skill drafts, mini routes, session navigation, and streaming state reduction. It is a **consumer** of CometMind's REST/SSE API — never a second source of truth for messages, tool results, jobs, or memories.

## Tech stack

| Layer | Choice |
|-------|--------|
| Framework | SvelteKit 2 + Svelte 5 (runes: `$state`, `$derived`, `$effect`) |
| Language | TypeScript strict mode |
| Styling | Tailwind CSS v4 |
| State | Module-level store singletons (not Svelte stores) |
| API client | Hand-written `cometmind.ts` + generated OpenAPI types |
| Tests | Vitest, Storybook |

Patterns reference: `cometline/docs/FRONTEND_PATTERNS.md`

## Route structure

| Route | File | Role |
|-------|------|------|
| `/` | `routes/+page.svelte` | Hero empty state, first session creation |
| `/session/[id]` | `routes/session/[id]/+page.svelte` | Active chat (ChatView keyed by sessionId) |
| `/jobs` | `routes/jobs/+page.svelte` | Jobs board and detail drawer |
| `/skill-drafts` | `routes/skill-drafts/+page.svelte` | Draft skill review/promotion |
| `/settings` | `routes/settings/+page.svelte` | Direct settings route outside normal shell |
| `/mini`, `/mini/session/[id]` | `routes/mini/` | Compact mini-window chat |
| Layout | `routes/+layout.svelte` | Health poll, settings load, workspace init, runtime SSE |

**Critical:** Session route keys `ChatView` by `sessionId` so SvelteKit route reuse doesn't keep stale per-session state.

## Store architecture

All stores are Svelte 5 `$state`-based singletons exported from `src/lib/stores/`.

| Store | File | Owns |
|-------|------|------|
| `chatStore` | `chat.svelte.ts` | Transcript, streaming, abort, SSE application |
| `sessionStore` | `session.svelte.ts` | Session list, selection, pending first message |
| `settingsStore` | `settings.svelte.ts` | Provider settings draft, save orchestration |
| `modelStore` | `model.svelte.ts` | Flattened model picker options |
| `shellStore` | `shell.svelte.ts` | Sidebar, settings modal, composer focus, session-scoped workspace-panel integration |
| `runtimeStore` | `runtime.svelte.ts` | Sidecar health, connection status |
| `memoryToasts` | `memory-toasts.svelte.ts` | Non-chat memory / compaction feedback from `/events` |

### Default model

`ProviderSettings` includes `defaultModelId` / `defaultProviderId`. `modelStore.setProviders()` selects the default on startup; `modelStore.selectDefault()` resets when returning home. Configure under Settings → Providers → Model roles.

### Persisted message context

Composer context (workspace file selections and line ranges, web pages, terminal selections, and assistant-response references) is sent with the user turn as `MessageContextRef` metadata. The backend persists the references with the message, and `MessageContextChips.svelte` restores clickable chips from transcript reloads. Keep the reference lean: content belongs in the turn payload when needed; the transcript stores the stable source/label/role needed to reopen it.

### chatStore.send — the streaming loop

```text
send(sessionId, content, options):
  1. Create AbortController
  2. Optionally add optimistic user bubble
  3. for await (event of streamMessage(...)):
       applyEventToSession(sessionId, event)
  4. finally: emit synthetic done, clear streaming state
```

Source: `chat.svelte.ts` — `send` / `applyEventToSession` (search by name; line numbers drift).

Cancel: `chatStore.cancel()` → abort controller + `DELETE .../runs/current`

## HTTP/SSE client

`src/lib/client/cometmind.ts` wraps CometMind REST/SSE.

### Key functions

| Function | Purpose |
|----------|---------|
| `createSession` | POST /sessions |
| `listSessions` | GET /sessions |
| `getSessionMessages` | GET /sessions/{id}/messages |
| `streamMessage` | POST /sessions/{id}/messages → async generator of StreamEvent |
| `abortSession` | DELETE /sessions/{id}/runs/current |
| Workspace file helpers | List/read/write previewable workspace files |
| Memory/MCP/jobs/skill-draft helpers | Settings and feature API calls |

### streamMessage internals

```text
POST with Accept: text/event-stream
  → read response.body with ReadableStream
  → parse SSE frames (data: {...})
  → yield parsed StreamEvent objects
```

## Type system

`src/lib/types.ts` mirrors CometMind wire types:

| Type | Mirrors |
|------|---------|
| `Session` | Session resource |
| `ProviderConfig` / `ProviderSettings` | Settings JSON |
| `TranscriptItem` | GET messages response row |
| `StreamEvent` | SSE frame union |
| `ChatItem` | Renderer-only row model (not on wire) |

`ChatItem` is the UI's view of a conversation row — it adds rendering state (pending, collapsed reasoning, subagent info) that doesn't exist in the API.

## SSE reducer

`src/lib/reducers/chat.ts` is a **pure function** layer — no side effects, no fetch.

### Entry points

| Function | When |
|----------|------|
| `reduceChatState` | Full state rebuild per event |
| `reduceChatStateDelta` | Optimized path for delta-only events |
| `loadTranscript` | Hydrate from GET messages on session open |

GitNexus process `proc_3_loadtranscript` links all three through the transcript load path.

### Event handling rules

| Event | Reducer behavior |
|-------|------------------|
| `text_delta` | Append to current assistant bubble |
| `reasoning_start` | Open reasoning block on assistant |
| `reasoning_delta` | Append to reasoning block |
| `tool_call` | Create tool row matched by ID |
| `tool_result` | Fill output on matching tool row |
| `step_finish` | Settle pending state, keep assistant |
| `subagent_progress` | Update subagent bubble |
| `subagent_finished` | Finalize subagent |
| `memory_injected` / `memory_updated` | Render memory event rows/cards |
| `turn_status` | Show pre-output activity status |
| `turn_recover` | Restore partial stream state after mid-turn failure |
| `error` | Auth errors → settings hints; others → error row |
| `done` | Clear streaming flags |

`memory_compaction_completed` is primarily consumed via the **runtime event stream** (`GET /api/v1/events` in `+layout.svelte` → `memory-toasts.svelte.ts`), not as a chat row.

### Immutability contract

```text
reduceChatState(prev, event):
  → deep clone prev
  → apply mutations to clone
  → return new ChatItem[] reference
```

Svelte 5 reactivity requires **new object references** for live token rendering. Mutating in place causes the UI to freeze during streaming.

## Component map

| Component | Role |
|-----------|------|
| `AppShell.svelte` | Root chrome, sidebar, settings modal, shortcuts |
| `Sidebar.svelte` | Session list, search, delete |
| `ChatView.svelte` | Per-session orchestration hotspot |
| `ChatThread.svelte` | Renders `ChatItem[]` rows |
| `Composer.svelte` | Textarea, model picker, stop, attachments |
| `SettingsPanel.svelte` | Shell for modular settings sections |
| `settings/SettingsProvidersPanel.svelte` | Providers + model roles |
| `settings/SettingsMemoryPanel.svelte` | Memory + compaction actions |
| `settings/SettingsMCPPanel.svelte` | MCP servers, tests, reconnect, OAuth, Cursor import |
| `settings/SettingsCometMindPanel.svelte` | Runtime knobs including coding task delegation |
| `JobsPage.svelte` / `JobsKanbanBoard.svelte` | Jobs board surface |
| `SkillDraftsPage.svelte` | Draft skill review and promotion |
| `FilePreview.svelte` / `FileEditor.svelte` | Workspace file panel |
| `RuntimeOverlay.svelte` | Blocks UI while sidecar connects |
| `SubagentMessageRow.svelte` / `SubagentPanel.svelte` | Harness / general subagent progress |
| `UpdateButton.svelte` | Auto-update affordance |

### Workspace panel

The panel has independent Wiki, Workspace, Changes, Web, and Terminal surfaces per session. The pure transition model is `workspace/workspace-panel-state.ts`; `shellStore` adapts it to reactive session maps, panel history, focus, and Electron visibility.

`WorkspacePanel.svelte` is the toolbar and interaction host. It delegates webview lifecycle and page capture to `WorkspaceWebSurface.svelte`, editor layers to `WorkspaceFileSurface.svelte`, Git selection/diff rendering to `GitChangesBrowser.svelte` / `GitDiffView.svelte`, and terminal lifecycle to `TerminalPanel.svelte`. Before selecting a different file, the shell awaits the active editor's leave guard so cancelling the confirmation leaves both the current path and draft unchanged.

Electron watches the active workspace and emits coalesced file/Git changes through preload. The layout routes those signals to workspace refresh state; file previews can reload, retain a dirty draft, or open a full-page diff instead of silently losing an external change.

### ChatView orchestration

The busiest component. Responsibilities:

1. Bind session ID before render
2. Load transcript (unless pending first message)
3. Serialize submissions via `createChatTurnQueue`
4. Coordinate first-turn fly animation via `startChat`
5. Wire stop button to `chatStore.cancel`
6. Refresh session title after turn

Source: `ChatView.svelte` (orchestration helpers in `chat-view-controller.svelte.ts` when present)

## Slash commands and skills

`src/lib/skills/slash-commands.ts`:

- Built-in: `/change`, `/clear`, `/create-skill`, `/model`, `/job`, `/list-jobs`
- Filter uses relevance scoring: prefix match (3) > substring (2) > description (1)
- Workspace skills discovered via CometMind API

## Settings UI

Modular panels under `src/lib/components/settings/` + `settings/schema.ts`:

Three persistence modes (see [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md)):

1. **Pending-save** — draft until "Save changes"
2. **Instant-save** — shortcuts, openAtLogin, Discord toggle
3. **Action-based** — fetch models, Codex/xAI sign-in, memory compaction, job operations, MCP tests/reconnect/OAuth/import

Electron merges `cometline-settings.json` + `cometline-desktop.json` for the UI and splits on write.

## Session first-message handoff

```text
+page.svelte:
  createSession() → sessionStore.setPendingMessage(content)
  goto(/session/{id})

ChatView on mount:
  if pendingMessage: consume it, skip transcript load, send immediately
```

Without this queue, navigation races cause duplicate or missing first bubbles.

## Frontend data flow diagram

```mermaid
flowchart LR
    subgraph Routes
        HOME["+page.svelte"]
        SESS["session/[id]/+page.svelte"]
        JOBS["jobs/+page.svelte"]
        DRAFTS["skill-drafts/+page.svelte"]
    end

    subgraph Stores
        SS[sessionStore]
        CS[chatStore]
        MS[modelStore]
    end

    subgraph Client
        API[cometmind.ts]
    end

    subgraph Pure
        RED[reducers/chat.ts]
    end

    subgraph UI
        CV[ChatView]
        CT[ChatThread]
        CMP[Composer]
    end

    HOME --> SS
    SESS --> CV
    JOBS --> API
    DRAFTS --> API
    CV --> CS
    CMP --> CS
    CS --> API
    API -->|SSE events| CS
    CS --> RED
    RED -->|ChatItem[]| CT
```

## Testing

```bash
cd cometline
pnpm run test        # Vitest
pnpm run check       # svelte-check + types
pnpm run lint        # ESLint
pnpm run storybook   # Isolated component development
```

Reducer tests are high value — they don't need Electron or CometMind running. See `reducers/chat.test.ts` if present.

## Common frontend bugs (from postmortems)

| Symptom | Root cause | Doc |
|---------|------------|-----|
| Streaming doesn't update live | Reducer mutates in place | `postmortem/streaming-ui-not-live-updating.md` |
| Session switch loses stream | In-flight response discarded | `postmortem/session-switch-in-flight-response-lost-and-rerender.md` |
| First turn invisible | Transcript load races navigation | `postmortem/first-turn-transcript-invisible.md` |
| Tool call ID mismatch on fork | Fork doesn't remap IDs | `postmortem/forked-session-tool-call-id-mismatch.md` |
| Memory settings save disabled | Impure dirty-state derivation | `postmortem/memory-subsystem-bugs.md` |

## What's next

[09-contracts-codegen.md](./09-contracts-codegen.md) — how OpenAPI, sqlc, and generated clients keep the three modules in sync.
