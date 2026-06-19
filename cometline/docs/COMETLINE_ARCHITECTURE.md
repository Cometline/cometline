# Cometline Architecture

Cometline is a local-first desktop AI app assembled from three modules in this workspace: `comet-sdk`, `cometmind`, and `cometline`.

This file records current architecture, runtime behavior, operational notes, and root-cause history. For a contributor map of the whole monorepo, see [`../../ARCHITECTURE_GUIDE.md`](../../ARCHITECTURE_GUIDE.md).

## One-Sentence Purpose

Cometline lets a user run a local desktop AI assistant with persistent workspace sessions, visible streaming reasoning and tool activity, semantic memory, provider switching, subagent delegation, and a polished native-feeling UI — while keeping the trusted agent runtime outside the renderer.

## Repo Roles

```text
comet-sdk
  Provider-normalized LLM I/O.
  Owns provider request/response conversion, streaming, reasoning, tool-call events,
  multimodal input, retry helpers, and fixtures.

cometmind
  Local agent runtime.
  Owns agent loop, semantic memory, SQLite persistence, workspace/session APIs,
  tool registry, ACP delegation, skills, Discord gateway, provider factory,
  local HTTP/SSE server, and CLI.

cometline
  Desktop shell and UI.
  Owns Electron lifecycle, CometMind sidecar startup, SvelteKit routes,
  chat rendering, memory/settings UI, web panel, transitions, auto-update,
  and desktop assets.
```

Dependency direction:

```text
cometline renderer
  -> CometMind local API on 127.0.0.1:7700
    -> cometmind runtime / storage / tools / memory / ACP
      -> comet-sdk provider interface
        -> Anthropic / OpenAI / OpenAI-compatible APIs

Electron main process
  -> spawns cometmind serve --watch-parent
  -> persists ~/.cometmind/cometline-settings.json (single settings SSOT)
  -> exposes OS/native capabilities over preload IPC
  -> optionally spawns cometmind gateway run --platform discord
```

The rule: Cometline is not the brain. CometMind is the brain. Comet SDK is only the model I/O boundary.

## Current Implementation Map

| Concept | Owner | Current status |
|---|---|---|
| Provider runtime | `comet-sdk` | Anthropic and OpenAI-compatible providers, including DeepSeek `reasoning_content`, embedded thinking tags, and vision input |
| Agent runtime | `cometmind/internal/agent` | Multi-step loop with streaming, reasoning, tool calls, memory retrieve/extract |
| Semantic memory | `cometmind/internal/memory` | Embedding retrieval, post-turn extraction, compaction, REST API + Cometline settings panel |
| ACP delegation | `cometmind/internal/acp` | `delegate_coding_task` spawns OpenCode (default); child sessions with progress/awaiting-input SSE |
| Agent Skills | `cometmind/internal/skills` | Discovery, system-prompt index, load/read/write tools, Cometline slash commands |
| Discord gateway | `cometmind/internal/gateway` | Allowlisted bot with per-thread sessions; Cometline can start/stop subprocess |
| Persistence | `cometmind/internal/db` | SQLite workspaces, sessions, messages, tool calls, memories, gateway mappings |
| Local API | `cometmind/server`, `openapi.yaml` | Health, sessions, transcript, stream message, abort, respond, skills, memory |
| Desktop runtime | `cometline/electron` | Sidecar spawn, health polling, settings IPC, updater, tray, workspace picker |
| Renderer UI | `cometline/src` | SvelteKit routes, sidebar, chat thread, composer, settings modal, web panel, animations |
| Secrets | Electron JSON settings | MVP-only. API keys in `~/.cometmind/cometline-settings.json`; move to OS keychain before wide distribution |
| Tool permission gates | Not implemented | CometMind executes requested tools directly; ACP permission prompts are subagent-scoped only |

## Runtime Contracts

### CometMind API Used By Cometline

Sessions and chat:

- `GET /api/v1/health`
- `POST /api/v1/workspaces`
- `POST /api/v1/sessions`
- `GET /api/v1/sessions?workspace_path=...`
- `GET /api/v1/sessions/{id}`
- `PATCH /api/v1/sessions/{id}`
- `DELETE /api/v1/sessions/{id}`
- `GET /api/v1/sessions/{id}/messages`
- `GET /api/v1/sessions/{id}/children`
- `POST /api/v1/sessions/{id}/message` returning SSE
- `POST /api/v1/sessions/{id}/abort`
- `POST /api/v1/sessions/{id}/respond`

Skills and memory:

- `GET /api/v1/skills`, `GET /api/v1/skills/{name}`, `POST /api/v1/skills/sync`
- `DELETE /api/v1/skills/{name}`, `GET /api/v1/skills/{name}/export`
- `GET /api/v1/memories`, `POST /api/v1/memories`, `PATCH /api/v1/memories/{id}`, `DELETE /api/v1/memories/{id}`
- `POST /api/v1/memories/search`
- `GET /api/v1/memory/settings`, `PUT /api/v1/memory/settings`
- `POST /api/v1/memory/compact`, `GET /api/v1/memory/compact/preview`

Renderer client: `src/lib/client/cometmind.ts`.

SSE event names currently rendered:

- `text_delta`, `reasoning_start`, `reasoning_delta`
- `tool_call`, `tool_result`, `step_finish`
- `subagent_started`, `subagent_progress`, `subagent_awaiting_input`, `subagent_finished`
- `memory_injected`, `memory_updated`
- `error`, `done`

Only one in-flight run per session is allowed (`409 session_running`).

### Electron IPC Used By Cometline

Exposed as `window.electronAPI` in `electron/preload.cjs`:

| IPC | Purpose |
|---|---|
| `restartCometMind()` | Restart the sidecar |
| `getWorkspacePath()` / `selectWorkspacePath()` / `setWorkspacePath()` | Workspace selection |
| `getProviderSettings()` / `saveProviderSettings()` | Read/write full settings blob |
| `fetchProviderModels(config)` | Query provider model list from main process |
| `getDiscordGatewayStatus()` / `setDiscordGatewayEnabled()` | Discord bot subprocess |
| `getOpenAtLogin()` / `setOpenAtLogin()` | macOS login item |
| `setSidebarOpen()` | Animate macOS traffic lights |
| `getFullScreen()` / `onFullScreenChange()` | Fullscreen sync |
| `getAppVersion()` | App version string |
| `getUpdateState()` / `checkForUpdates()` / `installUpdate()` / `onUpdateState()` | Auto-update |
| `openExternal(url)` | Open http(s)/mailto in system browser |
| `setShortcutCaptureActive()` / `setSessionNavigationSuspended()` | Pause global shortcuts |
| `setWebPanelOpen()` / `onCloseWebPanel()` / `onToggleWebPanel()` / `onOpenWebPanel()` | Web panel routing |
| `onNavigateSession()` | Previous/next chat from main-process shortcuts |

IPC is for OS/native capabilities only. Chat, session, memory, and skill data stay on REST/SSE.

## Product Boundaries

Cometline desktop may:

- Start, stop, and restart the CometMind child process (and optional Discord gateway subprocess).
- Persist desktop-level provider, CometMind, memory, appearance, and shortcut settings.
- Render session/chat/tool/memory/subagent state from CometMind.
- Fetch provider model lists through Electron IPC.
- Open external links and host an in-app webview panel.

Cometline desktop must not:

- Execute tools.
- Build provider LLM requests itself.
- Mutate the CometMind database directly.
- Treat renderer state as source of truth for transcripts.
- Store long-lived secrets in browser localStorage.

## Current UX Shape

### Routes

- `/` — new-session landing with centered project icon and hero composer.
- `/session/[id]` — active thread; consumes pending first messages from the home route.

### Chat flow

1. Sending from `/` creates a session, queues the first user message, and navigates to `/session/[id]`.
2. `ChatView` consumes the pending message through the turn queue and `startChat`.
3. First turn plays a flight animation before revealing the real user bubble and starting the SSE stream.
4. Subsequent turns stream immediately; the composer supports queuing while a turn is active.

### Settings sections

- **Providers** — API keys, base URLs, model fetch/enable
- **CometMind** — ACP config, skills list/sync/export/delete, Discord gateway
- **Memory** — config, CRUD, search, compaction preview/run
- **General** — open at login
- **Hero glow** — composer appearance and caret trail
- **Shortcuts** — rebindable keyboard shortcuts
- **About** — version, workspace change, update check, replay intro

### Other UX

- `Command+B` / `Ctrl+B` collapses the sidebar (macOS traffic lights animate).
- `Command+,` / `Ctrl+,` opens settings.
- `Command+C` stops streaming when nothing is selected.
- Narrow viewports (<900px) auto-collapse the sidebar.
- `project_icon.png` / avatars for empty state and assistant bubble.
- `app_icon.png` / `buildResources/icon.*` for the desktop app and tray.

## Developer Commands

From the repository root:

```bash
make install
make dev
make check
make build
make package
make port
make clean-log
```

Provider overrides can be passed at runtime:

```bash
COMETMIND_PROVIDER=openai \
COMETMIND_MODEL=deepseek-v4-flash \
COMETMIND_BASE_URL=https://opencode.ai/zen/go/v1 \
COMETMIND_API_KEY='...' \
make dev
```

Never commit real provider API keys to docs, Makefiles, source files, or tests.

## Runtime Files

| Path | Purpose |
|---|---|
| `~/.cometmind/cometmind.db` | CometMind SQLite database |
| `~/.cometmind/cometline-settings.json` | Single settings file (desktop UI + CometMind runtime) |
| `~/.cometmind/config.toml` | Legacy; read once for migration if JSON is missing |
| `~/.cometmind/cometline-workspace.json` | Selected workspace path |
| `~/.cometmind/cometline.log` | Electron-spawned CometMind logs (rotates at 10 MB while running → `.log.1`) |
| `~/.cometmind/cometline-gateway.log` | Discord gateway logs (same rotation) |

Default system prompt: packaged `SOUL.md` path is stored in `cometmind.systemPromptPath` inside `cometline-settings.json`.

## Manual Test Checklist

1. Run `make dev` from the repository root.
2. Confirm the Dock/window icon is the Cometline app icon. If macOS shows a stale icon, fully quit Electron and relaunch.
3. Press `Command+B`; sidebar should collapse/expand smoothly and traffic lights should animate.
4. Press `Command+,`; settings modal should open with Providers, CometMind, Memory, and other sections.
5. Configure provider base URL and API key, fetch models, select a model, then save. CometMind should restart cleanly.
6. Send a first message in a new chat; `/` should create a session, navigate to `/session/[id]`, play the flight animation, then stream the assistant reply live.
7. Attach an image via paste or drag-drop; the user bubble should show it and the model should receive it.
8. Type `/` in the composer; slash-command menu should appear (including `/create-skill` and discovered skills).
9. Click an http(s) link in an assistant message; it should open in the in-app web panel.
10. If the API key is missing, the chat should show one clean error card — not a raw JSON blob or dangling typing bubble.
11. Press `Command+C` during streaming; the turn should abort.
12. Hover a session row and delete. First deletion should show the in-app confirmation sheet; "Don't ask again" should skip future confirms.
13. Open Settings → Memory; verify list/search/compaction controls respond.
14. Optional: enable Discord gateway in Settings → CometMind and confirm status updates.

## Root-Cause Notes

Detailed postmortems live in [`docs/postmortem/`](postmortem/README.md). Summary of the highest-impact fixes:

### Renderer Could Not Reach CometMind From Vite

Symptom: runtime overlay showed `Failed to fetch` even when `curl /health` returned OK.

Cause: browser renderer origin `http://127.0.0.1:5173` needed local CORS headers from CometMind.

Fix: CometMind server allows local/file origins and `GET, POST, PATCH, DELETE, OPTIONS`.

### DeepSeek Tool Calls Failed After First Tool Result

Symptom: tool call succeeded, then the second model step failed with DeepSeek requiring `reasoning_content` to be passed back.

Cause: OpenAI-compatible parser handled `delta.reasoning` but not DeepSeek's `delta.reasoning_content`.

Fix: `comet-sdk/provider/openai` treats `reasoning_content` as a reasoning alias and does not duplicate reasoning at `[DONE]`.

### First Send Looked Frozen And Message Disappeared

Symptom: first send created a session but the chat window did not show the current message; UI felt frozen.

Cause: selecting the new session triggered transcript loading while `chatStore.send()` was starting, clearing items and cancelling the stream.

Fix: the app queues the first message before routing to `/session/[id]`; the session route owns consumption and streaming.

### First Bubble Appeared Before The Flight Animation Finished

Symptom: the real user bubble appeared while the animated text was still mid-flight.

Cause: chat user message mounted before the visual transition completed.

Fix: session route waits for the first-message flight before revealing the real user bubble. This intentionally delays the first network send for visual continuity.

### Failed Send Left A Weird Empty Assistant Bubble

Symptom: missing API key error left a typing bubble plus raw JSON error text.

Cause: assistant placeholder was not removed when the stream failed before text arrived, and raw server error bodies were rendered directly.

Fix: empty assistant placeholders are removed on error, and API-key errors are normalized to a concise settings hint.

### Streaming Text Only Appeared After The Turn Ended

Symptom: assistant and reasoning text appeared all at once after SSE finished instead of token-by-token.

Cause: stream-event handlers changed nested chat item fields without always publishing a new list/item reference for Svelte to observe reliably.

Fix: every streaming event that changes chat content must publish updated chat items. Prefer immutable item replacement or explicit array reassignment.

Prevention: when changing `chat.svelte.ts` or `ChatThread.svelte`, verify text/reasoning/tool output updates during an active stream, not only after `done`.

### User Bubble Vanished When Reasoning Mounted

Symptom: the user message disappeared briefly when the assistant began reasoning.

Cause: Svelte `transition:` can run both intro and outro when a keyed row shares a conditional multi-root fragment with siblings that mount mid-stream.

Fix: use one-shot `in:` transitions for rows that should only enter once; update visibility flags through immutable item replacement.

Prevention: do not add `transition:` to a row whose sibling assistant/reasoning/tool nodes can appear during streaming unless the outro behavior is intentional.

## Main Gaps

### 1. Tool Permission Gates

CometMind currently executes built-in tool calls directly. Before real daily use on untrusted workspaces, add:

- Tool risk metadata
- Pending permission events in SSE
- Approve/deny API
- Run-loop pause/resume for pending permissions
- Cometline approval UI

ACP subagent permission prompts exist today (`subagent_awaiting_input`) but only for delegated coding sessions.

### 2. Secret Storage

Move API keys out of Electron JSON settings. Preferred direction:

- CometMind-owned provider config API
- OS keychain integration
- Renderer receives redacted secret state only

### 3. Provider/Model API Ownership

Cometline currently fetches `<baseURL>/models` via Electron IPC. Longer term, CometMind should own provider discovery through endpoints such as:

- `GET /api/v1/config`
- `PATCH /api/v1/config`
- `GET /api/v1/providers`
- `GET /api/v1/models`
- `POST /api/v1/providers/test`

### 4. Cross-Platform Desktop

Current polish targets macOS (traffic lights, tray, vibrancy, login item). Windows/Linux builds are not first-class yet.

### 5. Gateway Surfaces Beyond Discord

The gateway adapter interface supports additional platforms; only Discord is implemented today.

## Build Priorities

1. Tool permission events and approval UX for built-in tools.
2. Safer provider settings and OS keychain secret storage.
3. Cross-platform desktop parity.
4. CometMind-owned provider/model discovery API.
5. Additional messaging gateway platforms.
