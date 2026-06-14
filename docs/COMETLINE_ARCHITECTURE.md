# Cometline Architecture

Cometline is a local-first desktop AI app assembled from three repos in this workspace: `comet-sdk`, `cometmind`, and `cometline`.

Docs policy: keep Cometline docs to this file plus `PHASE.md`. This file records current architecture, runtime behavior, operational notes, and root-cause history. `PHASE.md` records the living task board and roadmap.

## One-Sentence Purpose

Cometline lets a user run a local desktop AI assistant with persistent sessions, tool visibility, provider switching, and a polished native-feeling UI while keeping the trusted agent runtime outside the renderer.

## Repo Roles

```text
comet-sdk
  Provider-normalized LLM I/O.
  Owns provider request/response conversion, streaming, reasoning, tool-call events,
  retry helpers, and fixtures.

cometmind
  Local agent runtime.
  Owns agent loop, SQLite persistence, workspace/session APIs, tool registry,
  provider factory, local HTTP/SSE server, CLI, and TUI.

cometline
  Desktop shell and UI.
  Owns Electron lifecycle, CometMind process startup, SvelteKit routes,
  chat rendering, settings UI, transitions, and desktop assets.
```

Dependency direction:

```text
cometline renderer
  -> CometMind local API on 127.0.0.1:7700
    -> cometmind runtime/storage/tools
      -> comet-sdk provider interface
        -> Anthropic / OpenAI / OpenAI-compatible APIs
```

The rule: Cometline is not the brain. CometMind is the brain. Comet SDK is only the model I/O boundary.

## Current Implementation Map

| Concept          | Owner                                       | Current status                                                                                                                   |
| ---------------- | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| Provider runtime | `comet-sdk`                                 | Implemented for Anthropic and OpenAI-compatible providers, including DeepSeek `reasoning_content` stream compatibility.          |
| Agent runtime    | `cometmind/internal/agent`                  | Implemented multi-step loop with streaming and tool calls.                                                                       |
| Persistence      | `cometmind/internal/db`, `internal/session` | Implemented SQLite workspaces, sessions, messages, tool calls, and sqlc queries.                                                 |
| Local API        | `cometmind/server`, `openapi.yaml`          | Implemented health, sessions, transcripts, stream message, abort, and delete.                                                    |
| Desktop runtime  | `cometline/electron`                        | Implemented Electron main/preload, CometMind spawn, health polling, provider settings IPC, and app icons.                        |
| Renderer UI      | `cometline/src`                             | Implemented SvelteKit home/session routes, sidebar, chat thread, composer, settings modal, transitions, and delete confirmation. |
| Secrets          | Electron JSON settings                      | MVP-only. API keys are currently stored in `~/.cometmind/cometline-settings.json`; must move before distribution.                |
| Permissions      | Not implemented                             | Next major safety feature. CometMind executes requested tools directly today.                                                    |
| Memory           | Not implemented                             | Future phase.                                                                                                                    |

## Runtime Contracts

### CometMind API Used By Cometline

- `GET /api/v1/health`
- `POST /api/v1/sessions`
- `GET /api/v1/sessions?workspace_path=...`
- `GET /api/v1/sessions/{id}`
- `DELETE /api/v1/sessions/{id}`
- `GET /api/v1/sessions/{id}/messages`
- `POST /api/v1/sessions/{id}/message` returning SSE
- `POST /api/v1/sessions/{id}/abort`

SSE event names currently rendered:

- `text_delta`
- `reasoning_start`
- `reasoning_delta`
- `tool_call`
- `tool_result`
- `step_finish`
- `error`
- `done`

### Electron IPC Used By Cometline

- `cometline:get-workspace-path`
- `cometline:get-provider-settings`
- `cometline:fetch-provider-models`
- `cometline:save-provider-settings`
- `cometmind:restart`

IPC is for OS/native capabilities only. Chat/session data stays on REST/SSE.

## Product Boundaries

Cometline desktop may:

- Start, stop, and restart the CometMind child process.
- Persist desktop-level provider settings for MVP development.
- Render session/chat/tool/error state from CometMind.
- Fetch provider model lists through Electron IPC for the current MVP.

Cometline desktop must not:

- Execute tools.
- Build provider requests itself.
- Mutate the CometMind database directly.
- Treat renderer state as source of truth.
- Store long-lived secrets in browser localStorage.

## Current UX Shape

- `/` is the new-session landing view with centered project icon and hero composer.
- Sending from `/` creates a session, queues the first user message, and navigates to `/session/[id]`.
- `/session/[id]` renders the active thread and consumes pending first messages.
- `Command+B` / `Ctrl+B` collapses the sidebar.
- `Command+,` / `Ctrl+,` opens provider settings.
- The composer model selector uses the model store populated by static defaults or fetched provider models.
- `project_icon.png` is used for the empty state and assistant avatar.
- `app_icon.png` / `buildResources/icon.*` are used for the desktop app icon.

## Developer Commands

From the repository root:

```bash
make install
make dev
make check
make build
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

- CometMind database: `~/.cometmind/cometmind.db`
- Cometline provider settings: `~/.cometmind/cometline-settings.json`
- Electron-spawned CometMind logs: `~/.cometmind/cometline.log`

## Manual Test Checklist

1. Run `make dev` from the repository root.
2. Confirm the Dock/window icon is the rounded Cometline app icon. If macOS shows the old icon, fully quit Electron and relaunch.
3. Press `Command+B`; sidebar should collapse/expand smoothly.
4. Press `Command+,`; settings modal should open.
5. Enter provider base URL and API key, fetch models, select a model, then save.
6. Send a first message in a new chat; `/` should create a session, navigate to `/session/[id]`, animate the request upward, then reveal the real user bubble.
7. If the API key is missing, the chat should show one clean error card, not a raw JSON blob or dangling typing bubble.
8. Hover a session row and click trash. The first deletion should show the in-app confirmation sheet.
9. Check `Don't ask again`, delete, then delete another session; native browser confirm should not appear.

## Root-Cause Notes

### Renderer Could Not Reach CometMind From Vite

Symptom: runtime overlay showed `Failed to fetch` even when `curl /health` returned OK.

Cause: browser renderer origin `http://127.0.0.1:5173` needed local CORS headers from CometMind.

Fix: CometMind server allows local/file origins and `GET, POST, DELETE, OPTIONS`.

### DeepSeek Tool Calls Failed After First Tool Result

Symptom: tool call succeeded, then the second model step failed with DeepSeek requiring `reasoning_content` to be passed back.

Cause: OpenAI-compatible parser handled `delta.reasoning` but not DeepSeek's `delta.reasoning_content`.

Fix: `comet-sdk/provider/openai` treats `reasoning_content` as a reasoning alias and does not duplicate reasoning at `[DONE]`.

### First Send Looked Frozen And Message Disappeared

Symptom: first send created a session but the chat window did not show the current message; UI felt frozen.

Cause: selecting the new session triggered transcript loading while `chatStore.send()` was starting, clearing items and cancelling the stream.

Fix: the app now queues the first message before routing to `/session/[id]`; session route owns consumption and streaming.

### First Bubble Appeared Before The Flight Animation Finished

Symptom: the real user bubble appeared at the top-right while the animated text was still mid-flight.

Cause: chat user message mounted before the visual transition completed.

Fix: session route waits for the 560ms first-message flight before revealing the real user bubble. This intentionally delays the first network send for visual continuity.

### Failed Send Left A Weird Empty Assistant Bubble

Symptom: missing API key error left a typing bubble plus raw JSON error text.

Cause: assistant placeholder was not removed when the stream failed before text arrived, and raw server error bodies were rendered directly.

Fix: empty assistant placeholders are removed on error, and API-key errors are normalized to a concise settings hint.

### Streaming Text Only Appeared After The Turn Ended

Symptom: assistant and reasoning text appeared all at once after SSE finished instead of token-by-token.

Cause: stream-event handlers changed nested chat item fields without always publishing a new list/item reference for Svelte to observe reliably.

Fix: every streaming event that changes chat content must publish updated chat items. Prefer immutable item replacement or explicit array reassignment for assistant text, reasoning text, and tool results.

Prevention: when changing `chat.svelte.ts` or `ChatThread.svelte`, verify text/reasoning/tool output updates during an active stream, not only after `done`.

### User Bubble Vanished When Reasoning Mounted

Symptom: the user message disappeared briefly when the assistant began reasoning.

Cause: Svelte `transition:` can run both intro and outro when a keyed row shares a conditional multi-root fragment with siblings that mount mid-stream. Visibility flags also become fragile if chat items are mutated in place while stream events clone/replace list entries.

Fix: use one-shot `in:` transitions for rows that should only enter once, avoid transition churn in multi-root `{#each}` branches, and update visibility flags through immutable item replacement.

Prevention: do not add `transition:` to a row whose sibling assistant/reasoning/tool nodes can appear during streaming unless the outro behavior is intentional.

## Main Gaps

### 1. Tool Permission Gates

CometMind currently executes tool calls directly. Before real daily use, add:

- Tool risk metadata.
- Pending permission events in SSE.
- Approve/deny API.
- Run-loop pause/resume for pending permissions.
- Cometline approval UI.

### 2. Secret Storage

Move API keys out of Electron JSON settings. Preferred direction:

- CometMind-owned provider config API.
- OS keychain integration.
- Renderer receives redacted secret state only.

### 3. Provider/Model API Ownership

Cometline currently fetches `<baseURL>/models` via Electron IPC. Longer term, CometMind should own provider discovery through endpoints such as:

- `GET /api/v1/config`
- `PATCH /api/v1/config`
- `GET /api/v1/providers`
- `GET /api/v1/models`
- `POST /api/v1/providers/test`

### 4. Stop/Abort UI

The backend has abort support. The renderer still needs a visible stop button during streaming.

### 5. Memory

No memory tables, retrieval, or UI exist yet. Add only after permissions and secrets are safer.

## Build Priorities

1. Permission events and approval UX.
2. Safer provider settings and secret storage.
3. Stop/abort streaming control.
4. Session title generation and sidebar polish.
5. Memory model and memory UI.
