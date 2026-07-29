# 07 — cometline Desktop Shell (Electron)

> **Prerequisite:** [06-cometmind-features.md](./06-cometmind-features.md)  
> **Next:** [08-cometline-frontend.md](./08-cometline-frontend.md)

## Purpose

The Electron layer owns everything the **browser sandbox cannot**: process management, filesystem access, native dialogs, auto-update, and OS integration. The SvelteKit renderer is a web app that talks to CometMind over HTTP and to Electron over IPC.

## Three-process model

```text
┌─────────────────────────────────────────────────┐
│  Electron MAIN (Node.js)                        │
│  src/main.ts -> domains/runtime.ts               │
│  Sidecar, IPC, settings, updater                 │
├─────────────────────────────────────────────────┤
│  CometMind SIDECAR (Go binary)                  │
│  cometmind serve — port 7700                    │
├─────────────────────────────────────────────────┤
│  RENDERER (Chromium + SvelteKit)                │
│  src/ — chat UI, no Node access                 │
└─────────────────────────────────────────────────┘
         ▲                    ▲
         │ IPC                │ HTTP/SSE
         │ (preload)          │ (fetch)
         └──── BrowserWindow ─┘
```

## Security posture

`BrowserWindow` webPreferences in `electron/src/domains/windows.ts`:

| Setting            | Value                     | Why                        |
| ------------------ | ------------------------- | -------------------------- |
| `contextIsolation` | `true`                    | Renderer can't access Node |
| `nodeIntegration`  | `false`                   | No `require()` in renderer |
| Preload only       | `electron/src/preload.ts` | Controlled API surface     |

Native access is **only** through `window.electronAPI`. `electron/src/app.ts` is only the entrypoint; `electron/src/domains/runtime.ts` composes the sidecar lifecycle, settings, IPC, windows, updater, terminal, personas, screen capture, and workspace watcher domains.

## Preload bridge

`electron/src/preload.ts` uses `contextBridge.exposeInMainWorld('electronAPI', { ... })`. Its `ElectronAPI` type lives in `electron/src/shared/api.ts`.

The renderer checks `window.electronAPI` before calling — this lets `pnpm run dev` work in a plain browser without Electron.

## IPC contract

| Method                                                                    | Purpose                                              |
| ------------------------------------------------------------------------- | ---------------------------------------------------- |
| `restartCometMind()`                                                      | Full sidecar restart (rare; mainly host/port)        |
| `getWorkspacePath()` / `selectWorkspacePath()` / `setWorkspacePath(path)` | Workspace management                                 |
| `getProviderSettings()` / `saveProviderSettings(settings)`                | Merge/split settings read/write + apply side effects |
| `fetchProviderModels(config)`                                             | Model list for API-key providers                     |
| Codex / xAI auth helpers                                                  | Subscription session sign-in / auth path discovery   |
| `getMcpOAuthStatus()` / `startMcpOAuth()`                                 | Native-browser MCP OAuth flow                        |
| `readCursorMcpConfig()`                                                   | Import Cursor-style MCP config                       |
| `notifyJob()`                                                             | Desktop notifications for job changes                |
| `watchWorkspace()` / `onWorkspaceChanged()`                               | Debounced external file/Git changes refresh renderer surfaces |
| Screen-capture access / preference methods                                | Read permission state and open OS settings           |
| `setSidebarOpen(payload)`                                                 | macOS traffic-light animation                        |
| `getFullScreen()` / `onFullScreenChange`                                  | Window state sync                                    |
| `getAppVersion()`                                                         | Version string                                       |
| `checkForUpdates()` / `installUpdate()`                                   | Sparkle / electron-updater                           |
| `setOpenAtLogin(enabled)`                                                 | macOS login item                                     |

The stable renderer contract lives in `electron/src/shared/api.ts` and `electron/src/preload.ts`; handler composition lives in `electron/src/domains/runtime-ipc.ts` and channel registration in `electron/src/domains/ipc.ts`. Prefer those modules over line-number maps.

## Sidecar lifecycle

### Binary resolution

```text
Development:  ../cometmind/dist/cometmind (built by make dev)
Production:   extraResources/cometmind (bundled by electron-builder)
```

### Start sequence

```text
spawn(cometmind, ["serve", "--port", "7700", "--watch-parent"])
  → poll GET /api/v1/health until 200
  → mark runtime as connected
```

`--watch-parent` exits the sidecar if Electron main dies (no parent pid argv).

### Stop sequence

```text
SIGTERM sidecar
  → wait for process exit (critical!)
  → resolve stop promise
```

**Why wait:** Port 7700 and SQLite WAL lock must release before restart. Starting a new sidecar before the old one exits causes connection failures.

### Reload vs restart vs gateway recycle

| Change class                                                                                                    | Behavior                                                |
| --------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| Almost all runtime settings (providers, memory, MCP, ACP/harness, storage cleanup, jobs reconcile, autonomy, …) | In-place `Runtime.Reload` — chat turn can continue      |
| `cometmind.gateway.*` (Discord token/env)                                                                       | Recycle **gateway process only**; main `serve` stays up |
| Listen host / port (process bind)                                                                               | Full main-sidecar restart                               |
| Desktop-only fields (`appearance`, `shortcuts`, `app`)                                                          | No CometMind apply — Electron-only                      |

Manual `restartCometMind()` IPC still forces a full restart. Details: [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md) § Restart Rules.

## Settings persistence

### Two-file model

| File                                   | Owns                                                                                                            |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `~/.cometmind/cometline-settings.json` | Runtime: providers, `cometmind.*` (ACP, MCP, memory, jobs, gateway, …). Mode `0600`.                            |
| `~/.cometmind/cometline-desktop.json`  | Desktop UI: `appearance`, `shortcuts`, `app` (+ stamped `systemPromptPath`). Agent tools never write this file. |

Electron **merges** both for the Settings UI and **splits** on every write. First read of a monolithic settings JSON peels desktop keys into the desktop file (idempotent migration). CometMind loads **only** the runtime file.

Legacy fallback: `~/.cometmind/config.toml` is read only when JSON is missing.

### Save flow

```text
Renderer draft → saveProviderSettings IPC
  → normalize full settings blob
  → split write: cometline-settings.json + cometline-desktop.json
  → refresh shortcuts / open-at-login / icon / native listeners
  → sync Discord gateway process if needed
  → Runtime.Reload, gateway recycle, or full restart per classify rules
  → renderer reconnects only when a full restart was requested
```

### Normalization

`electron/src/domains/settings.ts` and `settings/schema.ts` both normalize — Electron is authoritative on save; renderer validates on load. The split/merge helpers live in `electron/src/domains/settings-domain.ts`; classify logic remains on the Go side in `cometmind/internal/settingsapply/`.

Key sections:

| File     | Section                                               | Contents                                                                         |
| -------- | ----------------------------------------------------- | -------------------------------------------------------------------------------- |
| settings | `providers[]`, `defaultModelId` / `defaultProviderId` | Provider configs and default model roles                                         |
| settings | `cometmind`                                           | maxTokens, ACP/harness, MCP, memory, jobs, autonomy, scheduler, storage, gateway |
| desktop  | `appearance`                                          | Hero glow, caret trail, …                                                        |
| desktop  | `shortcuts`                                           | Keyboard bindings                                                                |
| desktop  | `app`                                                 | openAtLogin, intro completion, persona, …                                        |

## Model discovery

Owned by **Electron main**, not CometMind:

| Provider method                | Discovery                                                            |
| ------------------------------ | -------------------------------------------------------------------- |
| `opencode-go`                  | Hardcoded list                                                       |
| `anthropic`                    | `GET {baseURL}/v1/models` + `x-api-key`                              |
| `openai` / `openai-compatible` | `GET {baseURL}/models` + bearer                                      |
| `codex`                        | ChatGPT subscription session (`~/.codex/auth.json` or `$CODEX_HOME`) |
| `xai`                          | Borrowed Grok session (`~/.cometmind/xai/auth.json`)                 |

Called via `fetchProviderModels` IPC (API-key providers) or dedicated Codex/xAI auth IPC from Settings UI.

## Workspace management

```text
getWorkspacePath():
  1. COMETMIND_WORKSPACE_PATH env
  2. ~/.cometmind/cometline-workspace.json
  3. Default (often repo root or last used)

setWorkspacePath(path):
  → persist to cometline-workspace.json
  → POST /api/v1/workspaces to CometMind
  → renderer reloads session list
```

## Logs and other paths

| Path                                      | Role                                      |
| ----------------------------------------- | ----------------------------------------- |
| `~/.cometmind/logs/cometline.log`         | Main sidecar log (10MB rotate → `.log.1`) |
| `~/.cometmind/logs/cometline-gateway.log` | Discord gateway log                       |
| `~/.cometmind/tool-output/`               | Spilled tool output                       |
| `~/.cometmind/agent-tmp/`                 | Shared agent tmp                          |

## Production URL scheme

Packaged app registers custom protocol `app://bundle`:

```text
app://bundle/index.html     → SPA entry
app://bundle/_app/...       → static assets
Fallback to index.html for client-side routes (/session/{id})
```

Without SPA fallback, reloading a session URL in the packaged app returns 404.

## Auto-update

- macOS: electron-updater + notarization
- `checkForUpdates` / `installUpdate` IPC
- `UpdateButton.svelte` floating UI in renderer
- State pushed via `onUpdateState` callback

## Native macOS features

| Feature                 | Implementation                           |
| ----------------------- | ---------------------------------------- |
| Traffic light animation | Sidebar open/close moves window controls |
| Hide on close           | Window hides instead of quitting         |
| Tray icon               | Optional system tray                     |
| Login item              | `setOpenAtLogin`                         |

## Packaging pipeline

```bash
make package
  → cd cometmind && go build → dist/cometmind
  → cd cometline && pnpm build (SvelteKit static)
  → electron-builder (DMG/ZIP)
```

`package.json` `extraResources` includes the sidecar binary outside the asar archive.

`pnpm run build:electron-main` runs `scripts/build-electron.mjs`: it bundles the ESM main-process source from `electron/src/main.ts` to `electron/dist/main.js`, and the TypeScript preload source from `electron/src/preload.ts` to the CommonJS preload bundle Electron loads.

## Electron concern map

Prefer symbol / module orientation over line numbers:

| Concern                                    | Where to look                                                                |
| ------------------------------------------ | ---------------------------------------------------------------------------- |
| ESM main entrypoint and composition        | `electron/src/main.ts`, `app.ts`, `domains/runtime.ts`                       |
| Sidecar spawn / stop / health              | `domains/cometmind-lifecycle.ts`                                             |
| Settings merge / split / normalize         | `domains/settings.ts`, `domains/settings-domain.ts`                          |
| Reload / restart / gateway apply           | `domains/runtime-ipc.ts`, `domains/cometmind-lifecycle.ts`                   |
| Workspace                                  | `domains/settings.ts`, `domains/runtime-ipc.ts`                              |
| Model discovery and subscription auth      | `domains/provider-auth.ts`                                                   |
| Ollama health, model management, and pulls | `services/ollama.ts`                                                         |
| MCP OAuth / Cursor import                  | `domains/provider-auth.ts`, `domains/runtime-ipc.ts`                         |
| Auto-updater                               | `domains/auto-updater.ts`                                                    |
| Window / tray / traffic lights             | `domains/windows.ts`, `domains/app-menu-tray.ts`, `domains/window-chrome.ts` |
| Preload and typed IPC contract             | `preload.ts`, `shared/api.ts`, `domains/ipc.ts`                              |

## Invariants

| Rule                                             | If broken                   |
| ------------------------------------------------ | --------------------------- |
| Renderer never imports Node                      | Security collapse           |
| Native only via preload IPC                      | Responsibility blur         |
| Sidecar stop waits for exit                      | Port/WAL lock               |
| Settings files mode 0600                         | API key exposure            |
| Desktop keys stay out of agent tools             | Agents rewrite UI state     |
| `app://bundle` SPA fallback                      | Packaged route reload fails |
| MCP OAuth token files stay outside settings JSON | Secret/token leakage        |

## What's next

[08-cometline-frontend.md](./08-cometline-frontend.md) — SvelteKit routes, stores, the SSE reducer, and chat UI components.
