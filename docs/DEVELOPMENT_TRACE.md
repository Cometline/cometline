# Cometline Development Trace

This document tracks implementation state, debugging context, and verification commands for the Cometline desktop shell. It is intentionally practical: update it when the runtime contract, UI behavior, launch flow, or development commands change.

## Current Stack

```text
Cometline Electron main process
  -> starts cometmind/dist/cometmind serve --port 7700
  -> passes provider settings through COMETMIND_* env vars
  -> opens SvelteKit renderer
  -> exposes narrow IPC for workspace path, provider settings, model fetch, restart

SvelteKit renderer
  -> talks to http://127.0.0.1:7700/api/v1 over REST/SSE
  -> owns only UI/cache state
  -> renders sessions, streaming chat, reasoning, tool calls, settings

CometMind
  -> owns agent loop, SQLite sessions/messages/tool calls, providers, tools
```

## Development Commands

From the repository root:

```bash
make install
make dev
make check
make build
make port
make clean-log
```

Useful provider overrides for dev runs:

```bash
COMETMIND_PROVIDER=openai \
COMETMIND_MODEL=deepseek-v4-flash \
COMETMIND_BASE_URL=https://opencode.ai/zen/go/v1 \
COMETMIND_API_KEY='...' \
make dev
```

Cometline also stores provider settings from the in-app settings modal at:

```text
~/.cometmind/cometline-settings.json
```

CometMind launch logs from Electron are written to:

```text
~/.cometmind/cometline.log
```

## Implemented Milestones

### Desktop Shell

- Scaffolded SvelteKit + Electron under `cometline/`.
- Electron main process starts the CometMind binary, polls health, and opens the renderer.
- Dev binary path defaults to `cometmind/dist/cometmind`; `make dev` builds it first.
- Electron uses `contextIsolation: true` and `nodeIntegration: false`.
- Renderer calls CometMind directly through REST/SSE.

### Chat MVP

- Session list is loaded from `GET /api/v1/sessions?workspace_path=...`.
- New sessions are created with current workspace, provider, and model.
- Session transcripts load through `GET /api/v1/sessions/{id}/messages`.
- Sending a message streams `POST /api/v1/sessions/{id}/message` SSE events.
- Chat UI renders user messages, assistant text, reasoning cards, tool call cards, tool results, status/error cards.
- Streaming auto-scroll is throttled with `requestAnimationFrame` to avoid per-token stutter.

### Session Management

- Added `DELETE /api/v1/sessions/{id}` in CometMind.
- Session deletion cascades persisted messages and tool-call rows through SQLite foreign keys.
- Active runs are cancelled before deleting a session.
- Sidebar supports hover delete.
- Delete confirmation uses an in-app sheet with a `Don't ask again` checkbox stored in `localStorage` as `cometline.skipDeleteConfirm`.

### Provider And Model Settings

- Added `Command+,` / `Ctrl+,` settings modal.
- Settings modal accepts provider, base URL, API key, and selected model.
- Electron fetches models from `<baseURL>/models` using the API key.
- Model selector in the composer is populated from fetched models.
- Saving provider settings restarts CometMind with updated `COMETMIND_*` env vars.

### Native-Feel Shell Details

- Added `Command+B` / `Ctrl+B` sidebar collapse with transition.
- Added reduced-motion CSS for transitions and view transitions.
- First message in a new chat animates toward the top-right without blocking send.
- Generated a macOS-style rounded app icon from `project_icon.png`:
  - `cometline/buildResources/icon.png`
  - `cometline/buildResources/icon.icns`
  - `cometline/static/app_icon.png`
- Empty state, assistant avatar, Electron window icon, Dock icon, and electron-builder mac icon use the rounded app icon.

## Important Fixes And Root Causes

### Renderer Could Not Reach CometMind From Vite

Symptom: runtime overlay showed `Failed to fetch` even when `curl /health` returned OK.

Root cause: browser renderer origin `http://127.0.0.1:5173` needed local CORS headers from CometMind.

Fix: CometMind server now allows local/file origins and `GET, POST, DELETE, OPTIONS`.

### DeepSeek Tool Calls Failed After First Tool Result

Symptom: tool call succeeded, then second model step failed with DeepSeek error requiring `reasoning_content` to be passed back.

Root cause: OpenAI-compatible parser handled `delta.reasoning` but not DeepSeek's `delta.reasoning_content`.

Fix: `comet-sdk/provider/openai` now treats `reasoning_content` as a reasoning alias and does not duplicate reasoning at `[DONE]`.

### First Send Looked Frozen And Message Disappeared

Symptom: first send created a session but the chat window did not show the current message; UI felt frozen.

Root cause: selecting the new session triggered transcript loading while `chatStore.send()` was starting, clearing items and cancelling the stream. The first-message animation also awaited 460ms before sending.

Fix: `chatStore.send()` now binds `sessionID` immediately, preventing transcript reload from racing the active stream. First-message animation is non-blocking.

### Failed Send Left A Weird Empty Assistant Bubble

Symptom: missing API key error left a typing bubble plus raw JSON error text.

Root cause: assistant placeholder was not removed when the stream failed before text arrived, and raw server error bodies were rendered directly.

Fix: empty assistant placeholders are removed on error, and API-key errors are normalized to a concise settings hint.

## Current Verification Baseline

These have passed after the current shell/chat/icon work:

```bash
cd cometline && rtk pnpm run check
cd cometline && rtk pnpm run build
rtk make check
rtk make build
```

`pnpm`/SvelteKit may print Node `DEP0205` warnings; these are tooling warnings and not app diagnostics.

## Manual Test Checklist

1. Run `make dev` from the repository root.
2. Confirm the Dock/window icon is the rounded Cometline icon. If macOS shows the old icon, fully quit Electron and relaunch.
3. Press `Command+B`; sidebar should collapse/expand smoothly.
4. Press `Command+,`; settings modal should open.
5. Enter provider base URL and API key, fetch models, select a model, then save.
6. Send a first message in a new chat; the request should animate upward and the session should appear in the sidebar.
7. If the API key is missing, the chat should show one clean error card, not a raw JSON blob or dangling typing bubble.
8. Hover a session row and click trash. The first deletion should show the in-app confirmation sheet.
9. Check `Don't ask again`, delete, then delete another session; native browser confirm should not appear.

## Known Risks

- Provider settings are currently stored in `~/.cometmind/cometline-settings.json`, including API key. This is acceptable for MVP development but should move to OS keychain or CometMind-managed encrypted storage before real distribution.
- Model fetching assumes an OpenAI-compatible `/models` endpoint. Anthropic/provider-native discovery needs a backend-owned provider config API.
- Permission gates for `write_file` and `run_command` are not implemented yet; CometMind currently executes tool calls directly.
- The renderer still calls CometMind directly via localhost. This matches current architecture, but future cloud mode should abstract the API base URL.
- Session titles remain `Untitled` until CometMind generates or updates titles.

## Next Development Targets

1. Move provider secrets out of Electron JSON settings and into a safer storage path.
2. Add explicit tool permission events and approval UI before broad tool usage.
3. Add an abort/stop button during streaming.
4. Improve session title generation and sidebar grouping.
5. Add visual regression coverage for sidebar collapse, settings modal, and chat error states.
6. Add a real provider/model API in CometMind so Cometline does not fetch provider model lists directly.
