# 08 - cometline Frontend (SvelteKit)

> **Prerequisite:** [07-cometline-desktop.md](./07-cometline-desktop.md)  
> **Next:** [09-contracts-codegen.md](./09-contracts-codegen.md)

## Purpose

The **renderer** draws the screens. It owns the chat UI, the jobs UI, and the settings UI. It also owns skill drafts, mini routes, and session navigation. It also owns streaming state reduction.

A renderer is the part of the app that draws the UI. **Streaming** means the reply arrives in small pieces while the model is still writing. **Reduction** means turning those events into UI state.

The renderer only reads CometMind's REST/SSE API. **REST** is a request-and-response web API. **SSE** means Server-Sent Events: a live stream of events from the server.

It is never a second source of truth for messages, tool results, jobs, or memories. A source of truth is the one place that stores the real data. That place is CometMind.

## Tech stack

| Layer | Choice |
|-------|--------|
| Framework | SvelteKit 2 + Svelte 5 (runes: `$state`, `$derived`, `$effect`) |
| Language | TypeScript strict mode |
| Styling | Tailwind CSS v4 |
| State | Module-level Svelte 5 `$state` singletons; `$state.raw` on hot collections |
| API client | Hand-written `cometmind.ts` + generated OpenAPI types |
| Tests | Vitest, Storybook |

Runes are the Svelte 5 state tools listed above. Strict mode turns on stricter type checks. A singleton is one shared object for the whole app. A hot collection is a list that changes often. `$state.raw` stores that list without watching every nested field.

Patterns reference: `cometline/docs/FRONTEND_PATTERNS.md`

## Route structure

Rebind means the same view connects to a new session. A drawer is a side panel. A draft is work that is not final. Promote means accept that draft as a real skill. The shell is the frame around the page. A poll repeats a check on a timer. Init means the startup step.

| Route | File | Role |
|-------|------|------|
| `/` | `routes/+page.svelte` | Start the most recent session, or create an empty saved session |
| `/session/[id]` | `routes/session/[id]/+page.svelte` | Active chat. One kept `ChatView` rebinds between sessions |
| `/gallery` | `routes/gallery/+page.svelte` | Generated or presented media. Copy, download, and delete. Warn if the session is gone |
| `/jobs` | `routes/jobs/+page.svelte` | Jobs board and detail drawer |
| `/skill-drafts` | `routes/skill-drafts/+page.svelte` | Review a draft skill, or promote it |
| `/settings` | `routes/settings/+page.svelte` | Direct settings route, outside the normal shell |
| `/mini`, `/mini/session/[id]` | `routes/mini/` | Compact mini-window chat |
| Layout | `routes/+layout.svelte` | Health poll, settings load, workspace init, runtime SSE |

**Critical:** The session route keeps `ChatView` mounted on purpose across `/session/[id]` navigation. Mounted means the component stays in the page. It is not destroyed and created again.

`ChatView` rebinds its controller and store snapshot to the new `sessionId`. A snapshot is a copy of the current state. The lower `ChatThread` boundary owns keyed row rendering. Keyed means each row has a stable id.

This keeps in-flight Markdown when the user switches sessions. It also keeps scroll state and background streams. In-flight means the reply is still arriving.

## Store architecture

All stores are Svelte 5 `$state` singletons. They are exported from `src/lib/stores/`.

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

Orchestration means code that runs steps in order. Flattened means one simple list, not groups inside groups. Compaction means making stored memory shorter. A sidecar is the CometMind process that runs next to the app. A dismissal closes an inbox item.

### Default model

`ProviderSettings` includes `defaultModelId` and `defaultProviderId`. `modelStore.setProviders()` selects the default on startup. `modelStore.selectDefault()` resets to that default when you return home. Set this under Settings → Providers → Model roles.

### Persisted message context

The composer can attach context to the user turn. That context includes workspace file selections and line ranges. It also includes web pages, terminal selections, and assistant-response references.

The context is sent as `MessageContextRef` metadata. Metadata is extra data about the message, not the message text. The backend saves those references with the message.

`MessageContextChips.svelte` restores clickable chips when the transcript reloads. A chip is a small clickable label.

Keep each reference small. Put the content in the turn payload only when it is needed. A payload is the data sent with the turn. The transcript stores the stable source, label, and role. Those fields are enough to open the item again.

### chatStore.send - the streaming loop

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

A few words in those steps: stage means show the user message early. A bubble is one message block in the chat. A batch groups small text updates. Flush means apply the waiting updates now. A structural event changes which rows exist, not only their text. Synthesize means create a `done` event if the stream did not send one.

`chatStore` keeps a transcript cache for each session. It also keeps a stream handle for each session. A route change does not abort another session's response.

`writeSessionItems()` updates the view that is on screen. It also publishes the new snapshot to other windows.

Source: `chat.svelte.ts`. Search for `send`, `scheduleBatchForSession`, `applyEventToSession`, and `writeSessionItems`. Search by name. Line numbers change over time.

Cancel: `chatStore.cancel()` aborts the controller. It also calls `DELETE .../runs/current`.

## HTTP/SSE client

`src/lib/client/cometmind.ts` wraps CometMind REST and SSE. A wrapper is a small layer that calls the API for the UI.

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

`src/lib/types.ts` matches CometMind wire types. A wire type is the data shape sent over the network.

| Type | Mirrors |
|------|---------|
| `Session` | Session resource |
| `ProviderConfig` / `ProviderSettings` | Settings JSON |
| `TranscriptItem` | GET messages response row |
| `StreamEvent` | SSE frame union |
| `ChatItem` | Renderer-only row model (not on wire) |

A union is one type that can be one of several shapes. Not on the wire means the API does not send this type.

`ChatItem` is the UI view of one conversation row. It adds rendering state that the API does not have. That state includes pending activity and segmented reasoning. It also includes tool timing and subagent progress. Segmented reasoning means the reasoning text is split into parts.

Expand and collapse state stays in the thread fold controller. It is not part of the transcript model.

## SSE reducer

`src/lib/reducers/chat.ts` is a pure function layer. A pure function does not change outside state, and it does not fetch. A side effect is a change outside the function, such as a network call.

### Entry points and ownership

Hydrate means build UI rows from saved transcript rows.

| Function | When |
|----------|------|
| `initChatState` | Create an empty reducer state |
| `reduceChatState` | Public event reducer; selects the hot or structural path |
| `reduceChatStateDelta` | Private shallow-copy path for token-like events |
| `itemsFromTranscript` | Hydrate GET transcript rows in `stores/chat-transcript.ts` |

A shallow copy copies the list, not every object inside it. The hot path here is the path for token-like events.

`chatStore.loadTranscript()` fetches the transcript and publishes the cache. Transcript conversion stays outside the event reducer on purpose. Saved rows and live SSE events have different shapes.

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

Settle means mark that step as finished. Finalize means mark the subagent as finished. Persisted means saved.

`memory_updated` and `memory_compaction_completed` come from the runtime event stream. They are not chat rows. That stream is `GET /api/v1/events` in `+layout.svelte`. The handler is `memory-toasts.svelte.ts`.

### Immutability contract

```text
token-like event:
  shallow-copy ChatItem[] → update active assistant/reasoning → publish

structural event:
  clone ChatState/items → add/remove/relink rows → publish
```

The input state is never changed in place. Each reduction publishes a new array reference. Immutability means you do not edit the old value.

This split matters. Copying every transcript row for every token would still be correct. It would waste CPU time. CPU is processor time. Editing the published array in place would hide the update. Readers of `$state.raw` would miss it.

## Performance architecture

Download speed is not the slow part. Electron serves the renderer from the local `app://bundle`.

The heavy work is local. The app loads files from disk. It parses JavaScript and then evaluates it. Evaluate means it runs the parsed code. It builds reactive objects. A reactive object updates the screen when its data changes. The app reduces streaming events many times. It also draws long transcripts.

The frontend improves two areas. One is the startup graph, the set of files loaded at start. The other is the streaming hot path. The hot path is the code that runs for every token.

### Startup work: critical, idle, and on demand

Critical work runs on the first path. Idle work runs after the first screen is ready, while the app is waiting. On demand means the work runs only when the user opens that part. Chrome, in the diagram, means the outer frame of the window.

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

The idle deadline is 1.5 seconds. Mount means the component has been added to the page.

`AppShell.svelte` stores each dynamic-import promise. A dynamic import loads a file later. Idle preload and an early click then share one request.

If Workspace Panel or Inbox fails to load, the promise is reset. The UI then shows a retry control. If the intro fails, the intro closes. The user is not left on an overlay that cannot load. An overlay is a layer that covers the page.

The preload callback is cancelled if the shell unmounts. Unmount means the shell component is removed.

This is deferral, not permanent removal. Deferral means the work happens later. The feature is not deleted.

The main window normally evaluates Workspace Panel and Inbox after it becomes idle. This moves several parts off the first-render path. Those parts are CodeMirror, the terminal, and file preview. They also include Git diff, Inbox Markdown, and related components.

Their memory cost is still there after preload. That is the steady-state cost. Steady-state means after the delayed load has finished.

Panel state remains available across the lazy boundary because `shellStore` and `terminalStore` own it. A lazy boundary is the point where a file loads later. `WorkspacePanel.svelte` is a projection of session-scoped state. A projection is a view of data stored somewhere else. That state includes content, history, and tree expansion. It also includes editor, web, and terminal state.

Loading or remounting the component does not create a second source of truth.

While the chunk is not available, Back and Forward use session history. They do not call a panel ref that is not connected. A chunk is the JavaScript file that has not loaded yet. A ref is a handle to that panel.

Current startup limits:

- `SettingsModal`, `SetupWizard`, and `FileSearchModal` stay as static `AppShell` imports. Static means they load with the shell, not later.
- The shared root layout also runs in mini windows and settings windows. It runs health polling, runtime SSE, and the Inbox summary. It also runs skill-draft refresh and session loading. Only selected main-window work is gated. Gated means that work is limited to the main window.
- Idle preload improves first paint and how fast the UI responds. First paint is the first moment the window shows pixels. It does not reduce all bytes parsed during a long main-window session.

### Permanently smaller resource sets

A tradeoff is a gain in one place and a cost in another.

| Resource | Current boundary | Tradeoff |
|----------|------------------|----------|
| Material file icons | Curated filename/extension map + explicit SVG glob | Unknown or uncommon files use the generic file icon |
| Shiki | `createHighlighterCore`, JavaScript regex engine, one theme, explicit grammars | Unsupported language hints fall back to escaped plaintext |
| KaTeX CSS | Dynamic import only after rendered HTML contains KaTeX markup | The first math render loads the CSS once |

A curated set is a small chosen set, not every file. Fall back means use a simpler option. Escaped plaintext means the text is shown without colors, and special characters are made safe. The first math render loads the KaTeX CSS once.

The icon resolver no longer imports the full Material Icon Theme manifest. It also does not import every SVG. `?url&no-inline` emits stable asset URLs for only the curated set. This is a permanent bundle reduction, not delayed work. A bundle is the packaged app files.

The Shiki highlighter is a module-level promise singleton in each renderer. Assistant Markdown and Git diff highlighting share it. If setup fails, the promise is cleared, so a later render can retry.

The JavaScript regex engine avoids an Oniguruma WASM dependency. WASM is WebAssembly, a small binary format. Aliases are normalized before the code checks the loaded grammar set. An alias is another name for the same language. A grammar is the rule set used to color that language.

KaTeX JavaScript still belongs to the Markdown module. Its stylesheet is no longer global. `renderMarkdown()` waits for the CSS import only when the parsed output contains `class="katex`. Routes and messages with no math skip that style work.

### Streaming hot path

```text
SSE token events
  → per-session pendingBatchEvents
  → one requestAnimationFrame callback
  → selective reducer cloning
  → $state.raw ChatItem[] assignment
  → keyed thread rows update
```

`text_delta`, `reasoning_delta`, `reasoning_start`, and `step_finish` can be batched. A structural event flushes the waiting batch first. This keeps event order around tool calls, errors, and completion.

The `finally` path also flushes. Cancel, or a broken stream, cannot leave buffered text unsent. Buffered text is text waiting in the batch.

`$state.raw` is a choice for transcript arrays and other hot collections. The renderer replaces the collection reference. It does not ask Svelte to proxy the whole object graph. A proxy is a wrapper that watches every nested change.

The reducer delta path shallow-copies the item array. Structural events use a full copy. That slower path runs only when row identity or row shape changes.

`AssistantMarkdown.svelte` adds another limit in the slow render layer. During streaming, parse and highlight work runs about once per 40 ms. This limit is a throttle. A throttle caps how often work runs.

It rejects stale async results with a render version. Stale means the result belongs to an earlier render. It skips work when the content and resources cache key is unchanged.

`markdown/render.ts` keeps two Marked instances and reuses them. It also keeps a cache of highlighted code. That cache holds at most 128 active entries.

### Session switching and multiple windows

`chatStore` caches transcript items, errors, context budgets, and stream handles by session ID. A route switch changes the bound session. Other handles keep running.

The session route keeps `ChatView` mounted. `ChatThread` takes a snapshot of the `$state.raw` items. It uses keyed turns and keyed items. Stable rows then keep their DOM nodes and their scroll state. The DOM is the page structure the browser draws.

The main window and the mini window keep the same data. They share session metadata, transcript snapshots, streaming flags, and unread markers. They use `BroadcastChannel('cometline-window-sync')`. This avoids asking the backend again for every token.

`chat-items` currently serializes the full `ChatItem[]` snapshot. Serialize means it turns the whole list into a message. A very long transcript costs more to copy and to send between windows.

### Thread rendering tradeoffs

`ChatThread` groups rows into turns. It attaches reasoning, tools, memory, and subagents to the assistant stack. A stack here is the group of rows under that assistant turn.

Rows that already sit inside that stack are hidden. Scroll work waits for the next animation frame. It does not run on every token.

If several reasoning pieces arrive in a row, `coalesceReasoningEntries` joins several reasoning pieces into one Thinking block. A truncated continuation should not show as a stack of Thinking buttons. Truncated means the text was cut short. A continuation is the next piece after that cut.

The thread is **not virtualized**. Virtualized means the UI draws only the rows on screen. Current speed depends on batching, conditional row visibility, stable keys, and selective reactivity. Selective reactivity means only the changed parts update.

If a very long transcript becomes the slow part, windowing must keep several behaviors. Windowing means drawing only a slice of the list. It must keep find-in-session and scroll anchoring. Scroll anchoring keeps the scroll position from jumping. It must also keep first-turn flight, expandable activity state, and active-stream behavior.

First-turn flight is the animation on the first turn. Windowing cannot be added as a generic list optimization. A generic list optimization would ignore these chat behaviors.

### Measurement boundaries

The repository does not keep a profiling build. It does not keep startup marks. It does not keep a script that reports bundle size. Profiling means measuring speed while the app runs. A startup mark is a timed point during launch.

This keeps diagnostics out of the shipped architecture. The shipped architecture is the design included in the app that users run. Diagnostics are measurement tools. You must measure again. Do not copy numbers from an old report.

When you judge a change:

1. Compare the initial route graph separately from all build artifacts. The route graph is the set of files the first page loads. An artifact is a file the build produces. Lazy loading can improve the route graph without making the full build smaller.
2. Use a production Electron build, not only Vite dev mode.
3. Measure repeated cold launches on the same hardware. A cold launch starts the app from a closed state. Report the median and the p95. The median is the middle result. The p95 is a slow result: 95 percent of runs are faster. A raw or gzip bundle delta is supporting evidence. It is not a launch-time result.
4. Profile a long active stream separately from startup. Also profile a long transcript separately from startup. Those runs use the reducer, Markdown, the DOM, and cross-window costs. Looking at the bundle cannot show those costs.
5. After you change a loading boundary, check the related behaviors. Check panel retry, session switching, and active background turns. Also check mini-window sync, first-turn flight, and the fallback for an unsupported code language.

## Component map

Chrome means the outer frame, such as the sidebar. A surface is the UI area for one feature. A knob, in the table, means a runtime control. An affordance is a control the user can use.

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

An adapter connects one part of the UI to another. MCP is a protocol for external tools. OAuth is a standard login flow.

### Workspace panel

Each session has its own Wiki, Workspace, Changes, Web, and Terminal surfaces. They are independent. One session does not replace another's panel state.

The pure transition model is `workspace/workspace-panel-state.ts`. Pure means this module does not touch the screen. `shellStore` adapts it to reactive session maps, panel history, focus, and Electron visibility.

`WorkspacePanel.svelte` is the toolbar and the interaction host. It passes webview lifecycle and page capture to `WorkspaceWebSurface.svelte`. Lifecycle means how that part starts, updates, and closes. It passes editor layers to `WorkspaceFileSurface.svelte`. It passes Git selection and diff rendering to `GitChangesBrowser.svelte` and `GitDiffView.svelte`. It passes terminal lifecycle to `TerminalPanel.svelte`.

Before a different file is selected, the shell waits for the active editor's leave guard. A leave guard asks you to confirm before you leave unsaved work. If you cancel, the current path stays the same. The draft also stays unchanged.

Electron watches the active workspace. It emits coalesced file and Git changes through preload. Coalesced means several changes are joined into one signal. The layout routes those signals to workspace refresh state.

A file preview can reload. It can keep a dirty draft. Dirty means the draft has edits that are not saved. It can also open a full-page diff. It does not drop an external change without showing it.

### Conversation orchestration

`ChatView.svelte` is still where the screen is assembled. The lifecycle rules are split into two controllers.

A staged user row is shown before the request finishes. Gating means the controller can skip the transcript load. Hero means the centered composer. Docked means the composer is no longer centered.

| Owner | Responsibilities |
|-------|------------------|
| `conversation-controller.ts` | Session-scoped turn queues, staged user rows, send sequencing, pending first-message consumption, transcript-load gating, cancellation, title refresh |
| `chat-view-controller.svelte.ts` | Hero/docked composer state, enqueue/cancel adapters, view-level commands |
| `ChatView.svelte` | Bind the current session, expose reactive snapshots, adapt desktop/mini first-turn flights, render thread/composer states |

The queue map is module-level, and it is keyed by session ID. A queue can keep taking waiting turns after its original `ChatView` route is no longer active. It continues until the queue is empty. When the user returns, the controller reconnects change notifications to the visible view.

This is why background turns work with the retained route component. Retained means the route component stays mounted.

## Slash commands and skills

`src/lib/skills/slash-commands.ts` defines the built-in commands:

- Built-in: `/change`, `/clear`, `/create-skill`, `/model`, `/job`
- The filter scores relevance. Relevance means how well a command matches the typed text. A name prefix match scores 3. A name substring match scores 2. A description match scores 1.
- Workspace skills are discovered through the CometMind API.

The commands do not all use one dispatch path. A dispatch path is the code that runs the command. `/change`, `/model`, and `/job` open picker flows. The client handles `/clear` locally. `/create-skill` expands into an agent prompt. A discovered skill is submitted through the normal conversation path.

## Settings UI

Settings panels live under `src/lib/components/settings/`. Validation lives in `settings/schema.ts`.

There are three persistence modes. See [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md). Persistence means how a value is saved.

1. **Pending-save.** The UI keeps a draft until you choose "Save changes".
2. **Instant-save.** These save at once: shortcuts, `openAtLogin`, and the Discord toggle.
3. **Action-based.** These run an action: fetch models, Codex or xAI sign-in, and memory compaction. They also include job operations, and MCP tests, reconnect, OAuth, or import.

Electron merges `cometline-settings.json` and `cometline-desktop.json` for the UI. On write, it splits them again.

## Cross-route first-message handoff

A handoff passes a message from one route to the next page.

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

The normal new-chat action creates an empty saved session and opens it. The user can then send from that composer. That path does not use this handoff.

The pending-message path is for flows that already have a payload before navigation. It is keyed by session ID. A route change cannot take another session's payload.

The queue stages the user row and starts the SSE request once each. Both guards are required. Without them, navigation can race transcript hydration. A race means two actions overlap, and the result depends on which one finishes first. Hydration means loading saved messages into the UI. The race can create a duplicate first bubble, or it can drop the first bubble.

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

Reducer tests are useful. They do not need Electron. They do not need CometMind to be running. See `reducers/chat.test.ts` if that file is present.

## Common frontend bugs

| Symptom | Root cause |
|---------|------------|
| Streaming does not update live | The reducer changes the array in place |
| Session switch loses the stream | The in-flight response is dropped |
| First turn is invisible | Transcript load races navigation |
| Tool call ID does not match after a fork | The fork does not remap IDs |
| Memory settings save stays disabled | The dirty-state check is not a pure derivation |

A fork is a new session copied from another session. Dirty means the form has unsaved changes. A derivation computes a value from other values. Impure means that check is not a pure function.

## What's next

[09-contracts-codegen.md](./09-contracts-codegen.md) shows how OpenAPI, sqlc, and generated clients keep the three modules matched.
