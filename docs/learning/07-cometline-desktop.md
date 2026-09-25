# 07 — cometline Desktop Shell (Electron)

> **Prerequisite:** [06-cometmind-features.md](./06-cometmind-features.md)  
> **Next:** [08-cometline-frontend.md](./08-cometline-frontend.md)

## Purpose

The Electron layer owns work the browser sandbox cannot do. That work is process management, file access, native dialogs, auto-update, and OS integration.

A **sandbox** here is the limit on what a web page can do. A **native dialog** is a system window, such as a file picker. **OS integration** means operating-system features, such as login items and the tray icon.

The SvelteKit renderer is a web app. It talks to CometMind over HTTP. It talks to Electron over IPC. The **renderer** is the page the user sees. **IPC** means inter-process communication. It is the message channel between the page and Electron main.

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

**Main** is the Electron Node.js process. The **sidecar** is the CometMind program that Electron starts beside itself. **Chromium** is the browser engine inside Electron. **Node** is the JavaScript runtime that can read files and start processes. The renderer must not use Node. **SSE** means Server-Sent Events. The server pushes events on one open connection. A **preload** script runs before the page and exposes a small API.

## Security posture

**Posture** means the security rules this app chooses.

`BrowserWindow` webPreferences are set in `electron/src/domains/windows.ts`. **webPreferences** are the security settings for that window.

| Setting            | Value                     | Why                        |
| ------------------ | ------------------------- | -------------------------- |
| `contextIsolation` | `true`                    | The renderer cannot reach Node |
| `nodeIntegration`  | `false`                   | No `require()` in the renderer |
| Preload only       | `electron/src/preload.ts` | Only a small API is exposed |

**contextIsolation** keeps page scripts away from Electron objects. The page sees only what preload exposes.

Native access goes only through `window.electronAPI`. `electron/src/app.ts` is only the entrypoint. `electron/src/domains/runtime.ts` connects the domains. Those domains cover the sidecar lifecycle, settings, IPC, and windows. They also cover the updater, the terminal, personas, screen capture, and the workspace watcher.

A **persona** is the written character of the assistant. A **workspace** is one project folder.

## Preload bridge

`electron/src/preload.ts` calls `contextBridge.exposeInMainWorld('electronAPI', { ... })`. The `ElectronAPI` type lives in `electron/src/shared/api.ts`.

**contextBridge** is the Electron API that exposes a limited object to the page.

The renderer checks `window.electronAPI` before it calls a method. That check lets `pnpm run dev` work in a normal browser, with no Electron.

## IPC contract

A few names in the table need a short note. **Debounced** means the app waits briefly, then runs once. **Traffic lights** are the red, yellow, and green window buttons on macOS. **Sparkle** is a macOS update library. A **side effect** is a follow-up action after save, such as reload. A **renderer surface** is a part of the UI that shows workspace data.

| Method                                                                    | Purpose                                              |
| ------------------------------------------------------------------------- | ---------------------------------------------------- |
| `restartCometMind()`                                                      | Full sidecar restart. Rare. Used mainly for host or port. |
| `getWorkspacePath()` / `selectWorkspacePath()` / `setWorkspacePath(path)` | Workspace path management                            |
| `getProviderSettings()` / `saveProviderSettings(settings)`                | Read and write settings. Merge and split the two files. Apply side effects. |
| `fetchProviderModels(config)`                                             | Model list for providers that use an API key         |
| Codex / xAI auth helpers                                                  | Subscription session sign-in, or auth path discovery |
| MCP OAuth                                                                 | CometMind HTTP OAuth flow, not Electron IPC          |
| `readCursorMcpConfig()`                                                   | Import a Cursor-style MCP config                     |
| `notifyJob()`                                                             | Desktop notifications for job changes                |
| `watchWorkspace()` / `onWorkspaceChanged()`                               | Debounced refresh of renderer surfaces after outside file or Git changes |
| Screen-capture access / preference methods                                | Read permission state and open OS settings           |
| `setSidebarOpen(payload)`                                                 | macOS traffic-light animation                        |
| `getFullScreen()` / `onFullScreenChange`                                  | Sync full-screen window state                        |
| `getAppVersion()`                                                         | Version string                                       |
| `checkForUpdates()` / `installUpdate()`                                   | Sparkle and electron-updater                         |
| `setOpenAtLogin(enabled)`                                                 | macOS login item                                     |

**OAuth** is a browser login flow. The user approves access, and the app receives a token. **MCP** means Model Context Protocol. It connects outside tool servers.

The stable renderer contract lives in `electron/src/shared/api.ts` and `electron/src/preload.ts`. Handler composition lives in `electron/src/domains/runtime-ipc.ts`. Channel registration lives in `electron/src/domains/ipc.ts`. Prefer those modules. Do not rely on line-number maps.

## Sidecar lifecycle

### Binary resolution

```text
Development:  ../cometmind/dist/cometmind (built by make dev)
Production:   extraResources/cometmind (bundled by electron-builder)
```

**extraResources** is the electron-builder folder for files shipped beside the app. **electron-builder** is the tool that packages the desktop app.

### Start sequence

```text
spawn(cometmind, ["serve", "--port", "7700", "--watch-parent"])
  → poll GET /api/v1/health until 200
  → mark runtime as connected
```

`--watch-parent` makes the sidecar exit if Electron main dies. It does not take a parent process id as an argument. A **process id** is the number the operating system gives a running program.

### Stop sequence

```text
SIGTERM sidecar
  → wait for process exit (critical!)
  → resolve stop promise
```

**SIGTERM** is the signal that asks a process to exit. The stop code must wait for that exit. Port 7700 must be free before a restart. The SQLite WAL lock must also be released. **SQLite** is the local database. **WAL** means write-ahead log. SQLite uses it while writing. If a new sidecar starts before the old one exits, the connection fails.

### Reload vs restart vs gateway recycle

| Change class                                                                                                    | Behavior                                                |
| --------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| Almost all runtime settings (providers, memory, MCP, ACP/harness, storage cleanup, jobs reconcile, autonomy, and more) | In-place `Runtime.Reload`. The chat turn can continue. |
| `cometmind.gateway.*` (Discord token or env)                                                                    | Recycle the gateway process only. Main `serve` stays up. |
| Listen host or port (the process bind)                                                                          | Full restart of the main sidecar                        |
| Desktop-only fields (`appearance`, `shortcuts`, `app`)                                                          | CometMind does not apply these. Only Electron handles them. |

**In place** means the running process loads the new settings and does not exit. **Recycle** means stop that one process and start it again. **Bind** means the address and port the server listens on. **ACP** is the coding-harness setting. A **harness** is an outside coding program. **Reconcile** means the jobs worker checks the queue on its interval. **Autonomy** means jobs can start with no new user message.

A manual `restartCometMind()` IPC call still forces a full restart. Details are in the Restart Rules section of [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md).

## Settings persistence

### Two-file model

| File                                   | Owns                                                                                                            |
| -------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `~/.cometmind/cometline-settings.json` | Runtime settings: providers and `cometmind.*` (ACP, MCP, memory, jobs, gateway, and more). Mode `0600`.         |
| `~/.cometmind/cometline-desktop.json`  | Desktop UI: `appearance`, `shortcuts`, `app`, plus a stamped `systemPromptPath`. Agent tools never write this file. |

**Mode `0600`** means only the file owner can read and write the file. **Stamped** means the app writes `systemPromptPath` into that file.

Electron merges both files for the Settings UI. On every write, it splits them again.

The first read of an old settings JSON file moves desktop keys into the desktop file. That old file held both runtime and desktop settings. This migration is idempotent. **Idempotent** means running it again does not change the result. A **migration** updates old data to the current shape. CometMind loads only the runtime file.

If the JSON file is missing, the app can still read `~/.cometmind/config.toml`. That older file is used only in that case. **TOML** is the older settings format.

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

A **blob** here means the full settings object. **Normalize** means rewrite values into one standard shape. **Classify** means choose reload, gateway recycle, or full restart.

### Normalization

`electron/src/domains/settings.ts` and `settings/schema.ts` both normalize settings. Electron is authoritative on save. The renderer checks values when it loads them. **Authoritative** means the copy Electron writes is the one that counts.

Split and merge helpers live in `electron/src/domains/settings-domain.ts`. Classify logic stays on the Go side, in `cometmind/internal/settingsapply/`.

Key sections:

| File     | Section                                               | Contents                                                                         |
| -------- | ----------------------------------------------------- | -------------------------------------------------------------------------------- |
| settings | `providers[]`, `defaultModelId` / `defaultProviderId` | Provider configs and default model roles                                         |
| settings | `cometmind`                                           | ACP/harness, MCP, memory, jobs, autonomy, scheduler, storage, gateway. `maxTokens` may still be in the file, but the agent does not use it as the reply limit. |
| desktop  | `appearance`                                          | Hero glow, caret trail, and more                                                 |
| desktop  | `shortcuts`                                           | Keyboard bindings                                                                |
| desktop  | `app`                                                 | openAtLogin, intro completion, persona, and more                                 |

A **provider** is one model service configuration. Hero glow and caret trail are visual settings.

## Model discovery

Electron main owns model discovery. CometMind does not.

| Provider method                | Discovery                                                            |
| ------------------------------ | -------------------------------------------------------------------- |
| `opencode-go`                  | Hardcoded list                                                       |
| `anthropic`                    | `GET {baseURL}/v1/models` with header `x-api-key`                    |
| `openai` / `openai-compatible` | `GET {baseURL}/models` with a bearer token                           |
| `codex`                        | ChatGPT subscription session (`~/.codex/auth.json` or `$CODEX_HOME`) |
| `xai`                          | Borrowed Grok session (`~/.cometmind/xai/auth.json`)                 |

**Hardcoded** means the list is written in the source. It is not fetched from the network. A **bearer** token is a secret sent with the request to prove access. **Borrowed** means the token comes from the local Grok login. It is not an API key.

The Settings UI calls `fetchProviderModels` IPC for API-key providers. Codex and xAI use their own auth IPC.

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

**Persist** means save to disk. **env** means an environment variable. That is a value set outside the program.

## Logs and other paths

| Path                                      | Role                                      |
| ----------------------------------------- | ----------------------------------------- |
| `~/.cometmind/logs/cometline.log`         | Main sidecar log. At 10MB it rotates to `.log.1`. |
| `~/.cometmind/logs/cometline-gateway.log` | Discord gateway log                       |
| `~/.cometmind/tool-output/`               | Spilled tool output                       |
| `~/.cometmind/agent-tmp/`                 | Shared temporary files for the agent      |

**Rotate** means rename the full log to `.log.1` and start a new log. **Spilled** tool output is text that was too large for the tool result. The full text is saved in this folder.

## Production URL scheme

The packaged app registers the custom protocol `app://bundle`. A **protocol** here is the `app://` name at the start of a URL.

```text
app://bundle/index.html     → SPA entry
app://bundle/_app/...       → static assets
Fallback to index.html for client-side routes (/session/{id})
```

**SPA** means single-page app. The package is one HTML page. The client changes the route. **Static assets** are built files such as scripts and styles. **Fallback** means an unknown path still returns `index.html`.

Without that fallback, reloading a session URL in the packaged app returns 404.

## Auto-update

- On macOS, updates use electron-updater and notarization.
- The IPC methods are `checkForUpdates` and `installUpdate`.
- `UpdateButton.svelte` draws the update control over the page.
- Update state is pushed to the renderer through the `onUpdateState` callback.

**Notarization** is Apple's check of the app before a user opens it. A **callback** is a function Electron calls when the state changes.

## Native macOS features

| Feature                 | Implementation                           |
| ----------------------- | ---------------------------------------- |
| Traffic light animation | Opening or closing the sidebar moves the window controls |
| Hide on close           | The window hides instead of quitting     |
| Tray icon               | Optional icon in the system tray         |
| Login item              | `setOpenAtLogin`                         |

The **system tray** is the row of small status icons on the desktop.

## Packaging pipeline

```bash
make package
  → cd cometmind && go build → dist/cometmind
  → cd cometline && pnpm build (SvelteKit static)
  → electron-builder (DMG/ZIP)
```

The `extraResources` field in `package.json` includes the sidecar binary. That binary stays outside the asar archive. **asar** is Electron's app archive.

`pnpm run build:electron-main` runs `scripts/build-electron.mjs`. It bundles the ESM main-process source from `electron/src/main.ts` to `electron/dist/main.js`. It also bundles the TypeScript preload from `electron/src/preload.ts` to the CommonJS preload file that Electron loads.

**ESM** is the modern JavaScript module format. **CommonJS** is the older module format used by the preload. A **bundle** is one output file built from many source files.

## Electron concern map

Prefer module and symbol names over line numbers.

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

**Composition** means how the main process connects its parts. **Spawn** means start a process. A **pull** here means download an Ollama model.

## Invariants

An **invariant** is a rule that must stay true.

| Rule                                             | If broken                   |
| ------------------------------------------------ | --------------------------- |
| The renderer never imports Node                  | The page can run system code |
| Native access goes only through preload IPC      | Main and renderer duties get mixed |
| Sidecar stop waits for process exit              | Port 7700 or the WAL lock stays busy |
| Settings files use mode 0600                     | Other users can read API keys |
| Desktop keys stay out of agent tools             | Agents can change UI state  |
| `app://bundle` has an SPA fallback               | Reload of a packaged route fails |
| MCP OAuth token files stay outside settings JSON | Secrets can be written into the settings file |

## What's next

[08-cometline-frontend.md](./08-cometline-frontend.md) covers SvelteKit routes, stores, the SSE reducer, and chat UI components.
