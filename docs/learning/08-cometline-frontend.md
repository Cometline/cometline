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
| State | Module-level Svelte 5 `$state` singletons; `$state.raw` on hot collections |
| API client | Hand-written `cometmind.ts` + generated OpenAPI types |
| Tests | Vitest, Storybook |

Patterns reference: `cometline/docs/FRONTEND_PATTERNS.md`

## Route structure

| Route | File | Role |
|-------|------|------|
| `/` | `routes/+page.svelte` | Bootstrap the most recent session or create an empty persisted session |
| `/session/[id]` | `routes/session/[id]/+page.svelte` | Active chat; one retained `ChatView` rebinds between sessions |
| `/gallery` | `routes/gallery/+page.svelte` | Generated/presented media; copy, download, delete; warning if the session is gone |
| `/jobs` | `routes/jobs/+page.svelte` | Jobs board and detail drawer |
| `/skill-drafts` | `routes/skill-drafts/+page.svelte` | Draft skill review/promotion |
| `/settings` | `routes/settings/+page.svelte` | Direct settings route outside normal shell |
| `/mini`, `/mini/session/[id]` | `routes/mini/` | Compact mini-window chat |
| Layout | `routes/+layout.svelte` | Health poll, settings load, workspace init, runtime SSE |

**Critical:** The session route deliberately keeps `ChatView` mounted across `/session/[id]` navigation. `ChatView` rebinds its controller and store snapshot to the new `sessionId`; the lower `ChatThread` boundary owns keyed row rendering. This preserves in-flight Markdown, scroll state, and background streams when the user switches sessions.

## Store architecture

All stores are Svelte 5 `$state`-based singletons exported from `src/lib/stores/`.

| Store | File | Owns |
|-------|------|------|
| `chatStore` | `chat.svelte.ts` | Transcript, streaming, abort, SSE application |
| `sessionStore` | `session.svelte.ts` | Session list, selection, pending first message |
| `settingsStore` | `settings.svelte.ts` | Provider settings draft, save orchestration |
| `modelStore` | `model.svelte.ts` | Flattened model picker options |
| `shellStore` | `shell.svelte.ts` | Sidebar, settings modal, composer focus, session-scoped workspace-panel integration |
| `connectionState` | `runtime.svelte.ts` | Sidecar health polling and connection status |
| `memoryToastStore` | `memory-toasts.svelte.ts` | Non-chat memory / compaction feedback from `/events` |
| `inboxStore` | `inbox.svelte.ts` | Inbox summary, drawer data, replies, dismissals |
| `terminalStore` | `terminal.svelte.ts` | Renderer snapshots of main-process terminal sessions |
| `skillDraftsStore` | `skill-drafts.svelte.ts` | Draft summary and background refresh state |
| `unreadSessionOutputStore` | `unread-session-output.svelte.ts` | Cross-session unread output markers |

### Default model

`ProviderSettings` includes `defaultModelId` / `defaultProviderId`. `modelStore.setProviders()` selects the default on startup; `modelStore.selectDefault()` resets when returning home. Configure under Settings → Providers → Model roles.

### Persisted message context

Composer context (workspace file selections and line ranges, web pages, terminal selections, and assistant-response references) is sent with the user turn as `MessageContextRef` metadata. The backend persists the references with the message, and `MessageContextChips.svelte` restores clickable chips from transcript reloads. Keep the reference lean: content belongs in the turn payload when needed; the transcript stores the stable source/label/role needed to reopen it.

### chatStore.send — the streaming loop

```text
send(sessionId, content, options):
  1. Reject a second active stream for the same session
  2. Create a per-session stream handle + AbortController
  3. Optionally stage/add the user bubble
  4. for await (event of streamMessage(...)):
       batch token-like deltas until the next animation frame
       flush before structural events (tool, error, done, etc.)
       reduce into that session's cached ChatItem[]
  5. finally: flush pending deltas, synthesize done if needed,
       clear streaming state, guarantee visible feedback
```

`chatStore` maintains per-session transcript caches and stream handles, so switching routes does not abort another session's response. `writeSessionItems()` updates the bound view and publishes the new snapshot to other windows.

Source: `chat.svelte.ts` — `send`, `scheduleBatchForSession`, `applyEventToSession`, and `writeSessionItems` (search by name; line numbers drift).

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

`ChatItem` is the UI's view of a conversation row — it adds rendering state such as pending activity, segmented reasoning, tool timing, and subagent progress that doesn't exist in the API. Expansion/collapse state remains in the thread fold controller rather than the transcript model.

## SSE reducer

`src/lib/reducers/chat.ts` is a **pure function** layer — no side effects, no fetch.

### Entry points and ownership

| Function | When |
|----------|------|
| `initChatState` | Create an empty reducer state |
| `reduceChatState` | Public event reducer; selects the hot or structural path |
| `reduceChatStateDelta` | Private shallow-copy path for token-like events |
| `itemsFromTranscript` | Hydrate GET transcript rows in `stores/chat-transcript.ts` |

`chatStore.loadTranscript()` owns fetching and cache publication. Transcript conversion is deliberately outside the event reducer because persisted rows and live SSE events have different shapes.

### Event handling rules

| Event | Reducer behavior |
|-------|------------------|
| `text_delta` | Append to current assistant bubble |
| `reasoning_start` | Open reasoning block on assistant |
| `reasoning_delta` | Append to reasoning block |
| `tool_call` | Create tool row matched by ID |
| `tool_result` | Fill output on matching tool row |
| `step_finish` | Settle pending state, keep assistant |
| `context_budget` | Update the active session's context-budget snapshot |
| `assistant_image` | Attach persisted media to the current assistant row |
| `subagent_started` | Create the child-agent progress row |
| `subagent_progress` | Update subagent bubble |
| `subagent_finished` | Finalize subagent |
| `memory_injected` | Render retrieved memories in the assistant activity timeline |
| `turn_status` | Show pre-output activity status |
| `turn_recover` | Restore partial stream state after mid-turn failure |
| `error` | Auth errors → settings hints; others → error row |
| `done` | Clear streaming flags |

`memory_updated` and `memory_compaction_completed` are consumed via the **runtime event stream** (`GET /api/v1/events` in `+layout.svelte` → `memory-toasts.svelte.ts`), not as chat rows.

### Immutability contract

```text
token-like event:
  shallow-copy ChatItem[] → update active assistant/reasoning → publish

structural event:
  clone ChatState/items → add/remove/relink rows → publish
```

The input state is never mutated and every reduction publishes a new array reference. The distinction matters: cloning every transcript row for every token would preserve correctness but waste CPU; mutating the published array in place would make `$state.raw` consumers miss updates.

## Performance architecture

The performance problem is not remote download speed: Electron serves the renderer from the local `app://bundle`. The expensive work is loading files from disk, parsing and evaluating JavaScript, constructing reactive objects, repeatedly reducing streaming events, and rendering long transcripts. The frontend therefore optimizes both the **startup graph** and the **streaming hot path**.

### Startup work: critical, idle, and on demand

```text
initial main-window path
  +layout.svelte
    → health/runtime SSE/settings/workspace initialization
    → AppShell core chrome + RuntimeOverlay
    → home/session route

idle after mount (deadline: 1.5 s)
  → preload WorkspacePanel
  → preload InboxDrawer

on demand
  → IntroAnimation when intro opens
  → WorkspacePanel immediately if opened before idle preload
  → InboxDrawer immediately if opened before idle preload
```

`AppShell.svelte` caches each dynamic-import promise so idle preload and an early click share one request. Workspace Panel and Inbox failures reset their promises and expose retry UI. Intro failure closes the intro instead of trapping the user behind an unavailable overlay. The preload callback is cancelled if the shell unmounts.

This is **deferral, not permanent removal**: the main window normally evaluates Workspace Panel and Inbox after it becomes idle. The benefit is moving CodeMirror, terminal, file preview, Git diff, Inbox Markdown, and related components off the first-render path. Their steady-state memory cost still arrives after preload.

Panel state survives the lazy boundary because `shellStore` and `terminalStore` own it. `WorkspacePanel.svelte` is a projection of session-scoped content, history, tree expansion, editor, web, and terminal state; loading or remounting the component does not create a second source of truth. While the chunk is unavailable, back/forward shortcuts fall back to session history instead of calling an unbound panel ref.

Current startup limits:

- `SettingsModal`, `SetupWizard`, and `FileSearchModal` remain static `AppShell` imports.
- The shared root layout runs health polling, runtime SSE, Inbox summary, skill-draft refresh, and session loading in mini/settings windows too; only selected main-window work is gated.
- Idle preload improves first paint and responsiveness, but does not reduce all bytes parsed over a long-running main-window session.

### Permanently smaller resource sets

| Resource | Current boundary | Tradeoff |
|----------|------------------|----------|
| Material file icons | Curated filename/extension map + explicit SVG glob | Unknown or uncommon files use the generic file icon |
| Shiki | `createHighlighterCore`, JavaScript regex engine, one theme, explicit grammars | Unsupported language hints fall back to escaped plaintext |
| KaTeX CSS | Dynamic import only after rendered HTML contains KaTeX markup | First math render pays the CSS load once |

The icon resolver no longer imports Material Icon Theme's complete manifest or every SVG. `?url&no-inline` emits stable asset URLs for only the curated set. This is permanent bundle reduction rather than delayed work.

The Shiki highlighter is a module-level promise singleton within each renderer, shared by assistant Markdown and Git diff highlighting. Initialization failure clears the promise so a later render can retry. The JavaScript regex engine avoids an Oniguruma WASM dependency; aliases are normalized before checking the loaded grammar set.

KaTeX JavaScript still belongs to the Markdown module, but its stylesheet is no longer global. `renderMarkdown()` awaits the CSS import only when the parsed output contains `class="katex`, so non-math routes and messages avoid that style work.

### Streaming hot path

```text
SSE token events
  → per-session pendingBatchEvents
  → one requestAnimationFrame callback
  → selective reducer cloning
  → $state.raw ChatItem[] assignment
  → keyed thread rows update
```

`text_delta`, `reasoning_delta`, `reasoning_start`, and `step_finish` are batchable. Structural events flush the pending batch first, preserving event order around tool calls, errors, and completion. The `finally` path also flushes, so cancellation or a broken stream cannot strand buffered text.

`$state.raw` is intentional for transcript arrays and other hot collections: the renderer replaces collection references instead of asking Svelte to deeply proxy a large object graph. The reducer's delta path shallow-copies the item array; structural events take the more expensive full-clone path only when row identity or shape changes.

`AssistantMarkdown.svelte` adds another throttle at the expensive rendering layer. During streaming it limits parse/highlight work to roughly one render per 40 ms, rejects stale async results with a render version, and skips work when its content/resources cache key is unchanged. `markdown/render.ts` keeps two reusable Marked instances and a highlighted-code cache capped at 128 active entries.

### Session switching and multiple windows

`chatStore` caches transcript items, errors, context budgets, and stream handles by session ID. A route switch changes the bound session but leaves other handles running. The session route keeps `ChatView` mounted, while `ChatThread` snapshots `$state.raw` items and uses keyed turns/items so stable rows retain DOM and scroll state.

Main and mini windows synchronize session metadata, transcript snapshots, streaming flags, and unread markers through `BroadcastChannel('cometline-window-sync')`. This avoids backend polling for every token, but `chat-items` currently serializes the full `ChatItem[]` snapshot. Very long transcripts therefore increase cross-window clone and transfer cost.

### Thread rendering tradeoffs

`ChatThread` groups rows into turns, attributes reasoning/tool/memory/subagent activity to assistant stacks, hides rows absorbed by those stacks, and keys both turn and item loops. Scroll work is scheduled with animation frames and DOM settling rather than performed synchronously on every token.

The thread is **not virtualized**. Current performance depends on batching, conditional row visibility, stable keys, and selective reactivity. If very long transcripts become a bottleneck, windowing must preserve find-in-session, scroll anchoring, first-turn flight, expandable activity state, and active-stream behavior; it cannot be added as a generic list optimization.

### Measurement boundaries

The repository does not retain a profiling build, startup marks, or a bundle-size reporting script. This keeps diagnostics out of the shipped architecture, but it also means performance claims must be reproduced rather than copied from an old report.

When evaluating a change:

1. Compare the initial route graph separately from all build artifacts; lazy loading can improve the former without reducing the latter.
2. Use a production Electron build, not only Vite dev mode.
3. Measure repeated cold launches on the same hardware and report median and p95; a raw/gzip bundle delta is supporting evidence, not a launch-time result.
4. Profile a long active stream and a long transcript separately from startup; they exercise reducer, Markdown, DOM, and cross-window costs that bundle inspection cannot show.
5. Verify panel retry, session switching, active background turns, mini-window sync, first-turn flight, and unsupported code-language fallback after changing a loading boundary.

## Component map

| Component | Role |
|-----------|------|
| `AppShell.svelte` | Root chrome, sidebar, shortcuts, and lazy panel boundaries |
| `Sidebar.svelte` | Session list, search, delete |
| `ChatView.svelte` | Reactive session binding, first-turn presentation, and controller adapters |
| `conversation-controller.ts` | Per-session turn queues, send sequencing, pending-message and transcript-load decisions |
| `chat-view-controller.svelte.ts` | Composer placement and view-level turn actions |
| `ChatThread.svelte` | Groups and renders keyed turns from `ChatItem[]` |
| `Composer.svelte` | Textarea, model picker, stop, attachments |
| `WorkspacePanel.svelte` | Lazy-loaded workspace/web/Git/terminal interaction host |
| `inbox/InboxDrawer.svelte` | Lazy-loaded Inbox interaction surface |
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

### Conversation orchestration

`ChatView.svelte` remains the presentation assembly point, but the lifecycle policy is split into two controllers:

| Owner | Responsibilities |
|-------|------------------|
| `conversation-controller.ts` | Session-scoped turn queues, staged user rows, send sequencing, pending first-message consumption, transcript-load gating, cancellation, title refresh |
| `chat-view-controller.svelte.ts` | Hero/docked composer state, enqueue/cancel adapters, view-level commands |
| `ChatView.svelte` | Bind the current session, expose reactive snapshots, adapt desktop/mini first-turn flights, render thread/composer states |

The queue map is module-level and keyed by session ID. A queue can continue draining after its original `ChatView` route is no longer active; when the user returns, the controller reconnects change notifications to the visible view. This is the mechanism that makes background turns compatible with the retained route component.

## Slash commands and skills

`src/lib/skills/slash-commands.ts`:

- Built-in: `/change`, `/clear`, `/create-skill`, `/model`, `/job`
- Filter uses relevance scoring: prefix match (3) > substring (2) > description (1)
- Workspace skills discovered via CometMind API

The commands do not all use one dispatch path. `/change`, `/model`, and `/job` open picker flows; `/clear` is handled locally; `/create-skill` expands into an agent prompt; a discovered skill is submitted through the normal conversation path.

## Settings UI

Modular panels under `src/lib/components/settings/` + `settings/schema.ts`:

Three persistence modes (see [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md)):

1. **Pending-save** — draft until "Save changes"
2. **Instant-save** — shortcuts, openAtLogin, Discord toggle
3. **Action-based** — fetch models, Codex/xAI sign-in, memory compaction, job operations, MCP tests/reconnect/OAuth/import

Electron merges `cometline-settings.json` + `cometline-desktop.json` for the UI and splits on write.

## Cross-route first-message handoff

```text
producer that must send after navigation (for example a job flow):
  create/fork session
  → sessionStore.queuePendingMessage(sessionId, text, attachments, context)
  goto(/session/{id})

ChatView mounts or sees the new sessionId:
  conversation.bindSession()
  conversation.onMount()
    → takePendingMessage(sessionId)
    → enqueue it in that session's queue
    → skip transcript load while pending/cached/in flight
```

The normal new-chat action creates and opens an empty persisted session; a user can then submit from its composer without this handoff. The pending-message path exists for flows that already own a payload before navigation. It is keyed by session ID, so route navigation cannot consume another session's payload. The queue stages the user row and starts the SSE request exactly once; without both guards, navigation can race transcript hydration and produce duplicate or missing first bubbles.

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
        CC[conversation-controller.ts]
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
    CV --> CC
    CMP --> CV
    CC --> CS
    CC --> SS
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
| Streaming doesn't update live | Reducer mutates in place | [`streaming-ui-not-live-updating.md`](../../cometline/docs/postmortem/streaming-ui-not-live-updating.md) |
| Session switch loses stream | In-flight response discarded | [`session-switch-in-flight-response-lost-and-rerender.md`](../../cometline/docs/postmortem/session-switch-in-flight-response-lost-and-rerender.md) |
| First turn invisible | Transcript load races navigation | [`first-turn-transcript-invisible.md`](../../cometline/docs/postmortem/first-turn-transcript-invisible.md) |
| Tool call ID mismatch on fork | Fork doesn't remap IDs | [`forked-session-tool-call-id-mismatch.md`](../../cometline/docs/postmortem/forked-session-tool-call-id-mismatch.md) |
| Memory settings save disabled | Impure dirty-state derivation | [`memory-subsystem-bugs.md`](../../cometline/docs/postmortem/memory-subsystem-bugs.md) |

## What's next

[09-contracts-codegen.md](./09-contracts-codegen.md) — how OpenAPI, sqlc, and generated clients keep the three modules in sync.
