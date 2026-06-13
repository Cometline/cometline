# Cometline Desktop — High-Level Design Document

> SvelteKit desktop application for the Cometline local agent runtime, packaged with Electron.
> Ships as a native `.dmg` / `.AppImage` / `.exe` — no terminal, no manual server management.
>
> Part of the [Cometline](../../../HLD.md) project.

| Field        | Value                                    |
|--------------|------------------------------------------|
| Version      | 0.1.0 (Draft)                            |
| Repo         | `github.com/cometline/cometline`         |
| Authors      | Cometline Team                           |
| Status       | Proposed                                 |
| Last Updated | 2026-02-23                               |

---

## Table of Contents

1. [Responsibility](#1-responsibility)
2. [Why Electron](#2-why-electron)
3. [Architecture](#3-architecture)
4. [Routes & Views](#4-routes--views)
5. [State Management](#5-state-management)
6. [IPC & API Layer](#6-ipc--api-layer)
7. [Component Design](#7-component-design)
8. [Non-Functional Requirements](#8-non-functional-requirements)
9. [Tech Stack](#9-tech-stack)
10. [Package Layout](#10-package-layout)
11. [Phase 2: Cloud / Hosted Version](#11-phase-2-cloud--hosted-version)

---

## 1. Responsibility

Cometline desktop is the **desktop application** for Cometline. It is an Electron app that:

- Automatically starts and manages the CometMind binary when the app opens
- Provides a SvelteKit session list, chat interface, and settings surface
- Renders the SvelteKit frontend against the local CometMind API and displays real-time streaming output from the agent loop
- Gives the user a self-contained, installable native app — no terminal, no manual server management, no `brew install` prerequisites

Cometline desktop does **not** implement any agent logic, tool execution, LLM calls, or data persistence. All of that lives in CometMind. Cometline desktop is an Electron shell around a SvelteKit frontend: it starts CometMind, opens the local app against `http://127.0.0.1:7700`, and leaves application state and APIs to the locally running CometMind server.

> **Note on CometMind's scope.** CometMind (formerly *CometCode*) is now a **general AI agent runtime**, not a coding-only tool. Its agent loop, tool registry, persistence, and HTTP+SSE server are domain-agnostic. When a task is coding work, CometMind **delegates it to a dedicated coding agent (e.g. OpenCode) over ACP (Agent Communication Protocol)** and folds the result back into the session. From Cometline desktop's perspective nothing changes — it still talks to the same local server and renders the same streaming session output — but the agent behind it is general-purpose, with coding handled by a specialized executor.

---

## 2. Why Electron

### 2.1 The Core Reason: Native Desktop Capabilities

Cometline desktop is an Electron app, not a browser tab. This is a deliberate architecture decision, not a workaround. The capabilities that matter:

| Capability | Browser (SvelteKit) | Electron (desktop) |
|-----------|--------------------|--------------------|
| Spawn CometMind binary automatically | No | Yes — `child_process.spawn` |
| Auto-start / auto-stop server | No — user must run manually | Yes — app lifecycle manages it |
| Direct SQLite read (future) | No — browser sandbox | Yes — Node.js main process |
| Offline / no server in PATH | No | Yes — self-contained |
| System tray, global hotkeys | No | Yes |
| Single-file distribution | No — separate install steps | Yes — `.dmg` / `.exe` / `.AppImage` |

### 2.2 The User Experience Difference

**With Electron:**
1. User installs Cometline.app
2. Opens it — CometMind starts automatically in the background
3. Closes it — CometMind stops

**Without Electron (browser-based):**
1. User installs CometMind separately
2. Remembers to run `cometmind serve`
3. Opens browser, navigates to `localhost:7700`
4. Keeps the terminal open

Eliminating steps 2–4 is the entire point of Cometline desktop.

### 2.3 Why a Static Go Binary Makes This Work

Go compiles to a **self-contained static binary** — no Go runtime to install, no shared libraries, no `PATH` setup. The `cometmind` executable is identical in nature to tools like `ripgrep` or `ffmpeg`. This makes embedding it inside an Electron app straightforward:

```
Node.js runtime    → already inside Electron
Go runtime         → compiled into the cometmind binary
                     user never needs to install Go
```

Contrast with a Python or Ruby backend, where you would need to bundle the entire interpreter. Go's static compilation is what makes the Electron approach clean.

### 2.4 Keeping the Door Open for a Future Cloud Version

The renderer is a SvelteKit application with no hard dependency on Electron APIs. Electron-specific code is confined to the **main process** and a thin **preload bridge**. This means the same route tree, components, stores, and API client can be reused in a hosted/cloud deployment if one is ever built. See [Section 11](#11-phase-2-cloud--hosted-version).

---

## 3. Architecture

### 3.1 Electron Process Model

Electron has two types of processes:

```
┌─────────────────────────────────────────────────────────────┐
│              Cometline desktop (Electron)                   │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Main Process (Node.js)                              │   │
│  │                                                      │   │
│  │  ┌───────────────┐   ┌──────────────────────────┐   │   │
│  │  │ app lifecycle │   │ CometMind process manager│   │   │
│  │  │ window create │   │ (child_process.spawn)    │   │   │
│  │  └───────────────┘   └──────────────────────────┘   │   │
│  │                                                      │   │
│  │  ┌──────────────────────────────────────────────┐   │   │
│  │  │  ipcMain handlers                            │   │   │
│  │  │  (bridge between renderer and Node.js)       │   │   │
│  │  └──────────────────────────────────────────────┘   │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │ IPC                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Renderer Process (Chromium + Svelte)                │   │
│  │                                                      │   │
│  │  ┌─────────────────────────────────────────────┐    │   │
│  │  │  SvelteKit route + component tree           │    │   │
│  │  │  (same app shell as future cloud version)   │    │   │
│  │  └────────────────────┬────────────────────────┘    │   │
│  │                       │                              │   │
│  │          ┌────────────┴────────────┐                 │   │
│  │          │  REST + SSE (localhost) │                 │   │
│  │          └────────────┬────────────┘                 │   │
│  └───────────────────────────────────────────────────── ┘   │
└──────────────────────────────────────────────────────────────┘
                            │
                   ┌────────▼────────┐
                   │ CometMind server│
                   │ 127.0.0.1:7700  │
                   │ (child process) │
                   └─────────────────┘
```

**Main process** responsibilities:
- Create and manage the `BrowserWindow`
- Spawn and monitor the CometMind binary as a child process
- Expose `ipcMain` handlers for operations the renderer cannot do directly (e.g. open file dialog, check if CometMind is alive)
- Handle app lifecycle: `app.on('ready')`, `app.on('window-all-closed')`, etc.

**Renderer process** responsibilities:
- Run the Svelte 5 UI
- Communicate with CometMind via REST + SSE over `localhost`
- Use `ipcRenderer` (via preload) only for native OS features

### 3.2 Why the Go Binary Needs No Runtime

Go compiles to a **self-contained static binary**. There is no Go runtime to install, no shared libraries to link, no `PATH` setup needed. The `cometmind` executable is identical in nature to tools like `ripgrep` or `ffmpeg` — you copy the file, you run it.

This is what makes the Electron approach work cleanly:

```
Node.js runtime    → already inside Electron
Go runtime         → compiled into the cometmind binary itself
                     user never needs to install Go
```

Contrast with a Python or Ruby backend, where you would need to bundle the entire interpreter. Go's static compilation makes it an ideal language for a binary that gets embedded inside another application.

### 3.3 CometMind Process Lifecycle

The user never sees a terminal, never types a command, never knows a Go server is running.

#### Startup

```
User double-clicks Cometline.app
          │
          ▼
Electron main process starts (Node.js)
          │
          ▼
Resolve binary path:
  dev:   ../../cometmind/dist/cometmind
  prod:  path.join(process.resourcesPath, 'cometmind')
          │
          ▼
child_process.spawn(binaryPath, ['serve', '--port', '7700'], {
    stdio: ['ignore', 'pipe', 'pipe']  // capture logs
})
          │
          ├── pipe stdout/stderr → app log file (~/.cometmind/cometline.log)
          │
          ▼
Poll GET http://127.0.0.1:7700/api/v1/health
  every 100ms, timeout 5s
          │
          ├── not ready yet → keep polling
          ├── timeout       → show error dialog, offer retry
          │
          ▼  200 OK
BrowserWindow.loadURL('http://127.0.0.1:7700')
          │
          ▼
User sees UI — CometMind is invisible
```

#### Shutdown

```
User closes window or presses Cmd+Q
          │
          ▼
app.on('before-quit')
          │
          ▼
cometmindProcess.kill('SIGTERM')
  → CometMind flushes pending DB writes, closes SQLite, exits
          │
          ├── exited within 3s → proceed
          ├── still running    → cometmindProcess.kill('SIGKILL')
          │
          ▼
app.quit()
```

#### Crash Recovery

```
cometmindProcess.on('exit', (code) => {
    if (userIsStillUsingApp) {
        // notify renderer via IPC
        mainWindow.webContents.send('cometmind:crashed', code)
        // renderer shows "CometMind stopped — Restart?" dialog
        // on confirm: re-run the startup sequence above
    }
})
```

### 3.4 Binary Bundling

`electron-builder` copies the CometMind binary into the packaged app's `resources/` directory at build time.

**`electron-builder.yml`:**
```yaml
extraResources:
  - from: "../cometmind/dist/cometmind-${os}-${arch}"
    to: "cometmind"

afterPack: "./scripts/chmod-binary.js"   # chmod +x resources/cometmind on mac/linux
```

**Packaged app structure (macOS example):**
```
Cometline.app/
└── Contents/
    ├── MacOS/
    │   └── Cometline             ← Electron shell binary
    └── Resources/
        ├── app.asar              ← Svelte UI + all JS (renderer)
        └── cometmind             ← Go binary, statically compiled, no deps
```

**Platform matrix:**

| Platform | CometMind binary | Cometline desktop output |
|----------|-----------------|--------------|
| macOS (arm64) | `cometmind-darwin-arm64` | `.dmg` (universal) |
| macOS (amd64) | `cometmind-darwin-amd64` | `.dmg` (universal) |
| Linux (amd64) | `cometmind-linux-amd64` | `.AppImage` |
| Windows (amd64) | `cometmind-windows-amd64.exe` | NSIS `.exe` |

GoReleaser (in the cometmind repo) produces all four binaries. The Cometline desktop CI pipeline downloads the appropriate binary before running `electron-builder`.

### 3.5 Communication Channels

| Channel | Used for |
|---------|----------|
| REST (HTTP, localhost) | CRUD: create session, list sessions, get messages, config |
| SSE (HTTP streaming, localhost) | Agent loop output: text tokens, tool calls, step finish |
| IPC (Electron contextBridge) | OS-native only: open folder dialog, get app version, receive crash notifications |

The renderer process talks to CometMind **directly over localhost HTTP**, not through IPC. IPC is reserved strictly for things that require Node.js or OS access — it is not a proxy for CometMind API calls.

---

## 4. Routes & Views

### 4.1 Route Map

```
/                        → Session List
/sessions/[id]           → Chat View
/settings                → Settings
```

Electron packages the SvelteKit renderer. Desktop mode should use SvelteKit routing with a static/SPA adapter strategy, while keeping the same route tree compatible with a future hosted deployment.

### 4.2 Session List (`/`)

**Layout:**
```
┌─────────────────────────────────────────────────────┐
│  Cometline            [+ New Session]  [⚙ Settings] │
├─────────────────────────────────────────────────────┤
│  🔍 Search sessions...                              │
├─────────────────────────────────────────────────────┤
│  fix auth bug                                       │
│  claude-sonnet-4  ·  2 hours ago  ·  42 messages    │
├─────────────────────────────────────────────────────┤
│  refactor payment service                           │
│  gpt-4o  ·  yesterday  ·  18 messages               │
├─────────────────────────────────────────────────────┤
│  ...                                                │
└─────────────────────────────────────────────────────┘
```

**Behaviour:**
- On load, fetches `GET /api/v1/sessions`
- Search filters the list client-side by session title
- Click a session → navigate to `/sessions/[id]`
- "New Session" → `POST /api/v1/sessions` → navigate to new session
- Right-click or hover → delete button → `DELETE /api/v1/sessions/:id`

### 4.3 Chat View (`/sessions/[id]`)

**Layout:**
```
┌─────────────────────────────────────────────────────┐
│ ← Sessions  │  fix auth bug  │  claude-sonnet-4     │
├─────────────────────────────────────────────────────┤
│                                                     │
│  You                                     14:32      │
│  fix the JWT validation in middleware.go            │
│                                                     │
│  Assistant                               14:32      │
│  I'll read the file first.                          │
│  ▶ read_file  middleware.go     ✓  8ms  [expand]    │
│                                                     │
│  The issue is on line 47. Here's the fix:           │
│                                                     │
│  ```go                                              │
│  if time.Now().Unix() >= claims.ExpiresAt {         │
│  ```                                                │
│                                                     │
│  ▶ write_file  middleware.go    ✓  3ms  [expand]    │
│  ▶ run_command go test ./...    ✓  1.4s [expand]    │
│                                                     │
│  All tests pass. The fix is applied.                │
│                                                     │
├─────────────────────────────────────────────────────┤
│  ┌───────────────────────────────────────────────┐  │
│  │ Message CometMind...                          │  │
│  └───────────────────────────────────────────────┘  │
│  [Ctrl+Enter to send]              [■ Stop]  [Send] │
└─────────────────────────────────────────────────────┘
```

**Behaviour:**
- On load: `GET /api/v1/sessions/:id/messages` to populate history
- Send message: `POST /api/v1/sessions/:id/message` → open SSE stream
- `text_delta` events: append token to streaming message
- `tool_call` events: render `ToolCallCard` in pending state
- `tool_result` events: update card to completed
- `step_finish` events: update token usage
- `done` event: mark stream complete
- Stop button: `POST /api/v1/sessions/:id/abort`

### 4.4 Settings (`/settings`)

**Sections:**

| Section | Contents |
|---------|----------|
| **Server** | CometMind port (default 7700); connection status badge |
| **Providers** | API key fields for Anthropic and OpenAI-compatible backends |
| **Defaults** | Default model (populated from `GET /api/v1/providers`) |
| **Appearance** | Theme: light / dark / system |
| **About** | App version; CometMind binary version |

API keys typed here are sent to CometMind via `POST /api/v1/config` to update `~/.cometmind/config.toml`. They are not stored in the renderer or in Electron's storage.

---

## 5. State Management

Cometline desktop uses **SvelteKit + Svelte 5 runes** throughout. No external state library.

### 5.1 Approach

| State scope | Approach |
|-------------|----------|
| Component-local | `$state` / `$derived` inside `.svelte` files |
| Cross-route shared | `.svelte.ts` modules with exported `$state` |
| Persisted across restarts | Electron's `electron-store` (JSON file in `userData`) |

### 5.2 Shared State Modules

**`src/lib/stores/connection.svelte.ts`**
```ts
export const connection = $state({
    port: 7700,
    status: 'connecting' as 'connecting' | 'connected' | 'error',
    error: null as string | null,
})
```

**`src/lib/stores/sessions.svelte.ts`**
```ts
export const sessions = $state({
    list: [] as Session[],
    activeId: null as string | null,
    loading: false,
})
```

**`src/lib/stores/chat.svelte.ts`**
```ts
export const chat = $state({
    messages: [] as Message[],
    streaming: false,
    streamBuffer: '',
    pendingToolCalls: {} as Record<string, ToolCall>,
})
```

**`src/lib/stores/settings.svelte.ts`**
```ts
// Persisted via electron-store
export const settings = $state({
    theme: 'system' as 'light' | 'dark' | 'system',
    defaultModel: '',
    port: 7700,
})
```

### 5.3 Data Flow

```
User clicks Send
      │
      ▼
ChatInput.svelte → api.sendMessage(sessionId, content)
      │
      ▼
src/lib/api/sse.ts
  fetch POST /api/v1/sessions/:id/message
  ReadableStream reader loop
      │
      ├── text_delta   → chat.streamBuffer += delta
      │                   (Svelte re-renders on next tick)
      ├── tool_call    → chat.pendingToolCalls[id] = { status: 'pending', ... }
      ├── tool_result  → chat.pendingToolCalls[id].status = 'done'
      ├── step_finish  → update token display
      └── done         → chat.streaming = false
                         commit streamBuffer to messages
```

---

## 6. IPC & API Layer

### 6.1 What Goes Through IPC vs REST

```
Renderer process
      │
      ├── REST + SSE (fetch to localhost:7700)
      │   └── everything CometMind-related
      │       sessions, messages, streaming, abort
      │
      └── IPC (contextBridge → ipcMain)
          └── OS / Electron-specific only
              ├── get app version
              ├── get CometMind status (is it running?)
              ├── open folder dialog (for workspace init)
              └── show native notification
```

The renderer must never call Node.js APIs directly. All Node.js access goes through the preload script's `contextBridge`.

### 6.2 Preload Script (`electron/preload.ts`)

The preload script is the **only** bridge between renderer and main process. It exposes a minimal, explicit API via `contextBridge` — the renderer cannot call arbitrary Node.js code.

```ts
// Exposed to renderer as window.electronAPI
contextBridge.exposeInMainWorld('electronAPI', {
    // App info
    getAppVersion: () =>
        ipcRenderer.invoke('app:version'),

    // CometMind process status
    getCometMindStatus: () =>
        ipcRenderer.invoke('cometmind:status'),  // 'starting' | 'ready' | 'crashed'

    // Crash notification — main process pushes this to renderer
    onCometMindCrashed: (cb: (exitCode: number) => void) =>
        ipcRenderer.on('cometmind:crashed', (_, code) => cb(code)),

    // Restart CometMind after crash
    restartCometMind: () =>
        ipcRenderer.invoke('cometmind:restart'),

    // Native file dialog
    openFolderDialog: () =>
        ipcRenderer.invoke('dialog:openFolder'),
})
```

The renderer accesses this as `window.electronAPI.getCometMindStatus()` etc. It never calls `ipcRenderer` directly.

### 6.3 REST Client (`src/lib/api/client.ts`)

Typed wrappers for all CometMind REST endpoints:

```ts
export const api = {
    sessions: {
        list():               Promise<Session[]>
        get(id: string):      Promise<Session>
        create(opts: NewSessionOpts): Promise<Session>
        delete(id: string):   Promise<void>
    },
    messages: {
        list(sessionId: string): Promise<Message[]>
    },
    providers: {
        list(): Promise<Provider[]>
    },
    config: {
        get():              Promise<Config>
        update(p: Partial<Config>): Promise<void>
    },
    abort(sessionId: string): Promise<void>
}
```

All functions throw a typed `ApiError` on non-2xx responses.

### 6.4 SSE Consumer (`src/lib/api/sse.ts`)

Uses `fetch` + `ReadableStream` (not `EventSource`, because `EventSource` only supports GET):

```ts
export async function streamMessage(
    sessionId: string,
    content: string,
    onEvent: (event: AgentEvent) => void,
    signal: AbortSignal,
): Promise<void>
```

### 6.5 TypeScript Types (`src/lib/api/types.ts`)

Hand-written TypeScript matching CometMind's JSON contract exactly:

```ts
export type Session = {
    id: string
    title: string
    model_id: string
    provider_id: string
    status: 'active' | 'archived'
    token_usage: TokenUsage
    created_at: number
    updated_at: number
}

export type AgentEvent =
    | { type: 'text_delta';   delta: string }
    | { type: 'tool_call';    id: string; tool: string; input: unknown }
    | { type: 'tool_result';  id: string; tool: string; output: string; error?: string }
    | { type: 'step_finish';  usage: TokenUsage }
    | { type: 'error';        message: string; code?: string }
    | { type: 'done' }
```

---

## 7. Component Design

### 7.1 Component Tree

```
src/
└── App.svelte                       ← root; router outlet + toast container
    ├── routes/SessionList.svelte    ← /
    │   ├── SessionCard.svelte
    │   └── SearchInput.svelte
    │
    ├── routes/ChatView.svelte       ← /sessions/[id]
    │   ├── MessageList.svelte
    │   │   ├── ChatMessage.svelte
    │   │   │   ├── StreamingText.svelte
    │   │   │   └── ToolCallCard.svelte
    │   │   └── TypingIndicator.svelte
    │   ├── ChatInput.svelte
    │   └── StatusBar.svelte
    │
    ├── routes/Settings.svelte       ← /settings
    │   ├── ServerSection.svelte
    │   ├── ProvidersSection.svelte
    │   └── AppearanceSection.svelte
    │
    └── Toast.svelte
```

### 7.2 Key Components

**`ChatMessage.svelte`**
- `role: user` → plain text bubble
- `role: assistant` → Markdown rendered with syntax-highlighted code blocks
- Embeds one `ToolCallCard` per tool call in the message

**`StreamingText.svelte`**
- Appends tokens one by one without re-rendering the whole message
- Shows a blinking cursor while `chat.streaming === true`

**`ToolCallCard.svelte`**
- Default: collapsed — shows tool name, status icon, duration
- Expanded on click: full input JSON + output text
- Status transitions: `pending` (spinner) → `running` → `done` ✓ / `error` ✗

**`ChatInput.svelte`**
- Auto-resizing textarea (max 8 lines)
- `Enter` → send; `Shift+Enter` → newline
- Disabled + shows Stop button while `chat.streaming === true`
- Stop button calls `api.abort(sessionId)`

**`StatusBar.svelte`**
- Session title (editable inline)
- Active provider + model name
- Cumulative token count for the session
- CometMind connection indicator (green dot / red dot)

---

## 8. Non-Functional Requirements

### 8.1 Performance

| ID | Requirement |
|----|-------------|
| P-01 | App window visible within 3s of launch on a modern machine (includes CometMind startup). |
| P-02 | Session list renders up to 500 sessions without jank. |
| P-03 | Streaming tokens render within one animation frame (≤16ms) of receipt. |
| P-04 | No layout shift when tool call cards expand or collapse. |

### 8.2 Reliability

| ID | Requirement |
|----|-------------|
| R-01 | If CometMind crashes, Cometline desktop shows an error state and offers a restart button. |
| R-02 | If the SSE stream drops mid-session, Cometline desktop shows a reconnect prompt. |
| R-03 | App close always sends SIGTERM to CometMind and waits for graceful exit (max 3s). |

### 8.3 Distribution

| ID | Requirement |
|----|-------------|
| D-01 | macOS: signed `.dmg` containing a universal binary (amd64 + arm64). |
| D-02 | Linux: `.AppImage` (amd64). |
| D-03 | Windows: NSIS installer `.exe` (amd64). |
| D-04 | The CometMind Go binary is bundled at `resources/cometmind` inside the Electron package. The user never needs to install Go or CometMind separately. |
| D-05 | The CometMind binary is built by GoReleaser in the cometmind repo CI. Cometline desktop CI downloads the pre-built binary before running `electron-builder`. |

### 8.4 Security

| ID | Requirement |
|----|-------------|
| SE-01 | `nodeIntegration: false` in all BrowserWindow instances. |
| SE-02 | `contextIsolation: true` — renderer cannot access Node.js directly. |
| SE-03 | All Node.js access from renderer goes through the `contextBridge` preload. |
| SE-04 | CometMind server binds to `127.0.0.1` only; not reachable from the network. |

### 8.5 Developer Experience

| ID | Requirement |
|----|-------------|
| DX-01 | `pnpm dev` starts Vite + Electron in dev mode with HMR for renderer changes. |
| DX-02 | `pnpm build` produces packaged apps for macOS, Linux, Windows via `electron-builder`. |
| DX-03 | `svelte-check` and TypeScript errors fail the build. |
| DX-04 | `pnpm test` runs Vitest unit tests for stores and API client. |

---

## 9. Tech Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Desktop shell | **Electron** | Cross-platform; Node.js main process can spawn CometMind and access local files. |
| UI framework | **SvelteKit + Svelte 5** | One route/component tree for desktop-first local mode and future hosted mode. |
| Language | **TypeScript** | Type safety against CometMind's API contract. |
| Reactivity | **Svelte 5 Runes** | `$state`, `$derived`, `$effect` — no virtual DOM. |
| Styling | **Tailwind CSS v4** | Utility-first; fast iteration. |
| Routing | **SvelteKit routing** | File-based routes packaged in Electron for desktop mode. |
| Markdown | **marked** | Small, fast Markdown parser. |
| Syntax highlighting | **highlight.js** | Language auto-detection for code blocks. |
| Persistent settings | **electron-store** | JSON file in `app.getPath('userData')`; survives app restarts. |
| Build tool | **Vite + electron-vite** | Fast HMR for renderer; handles main + preload + renderer in one config. |
| Packaging | **electron-builder** | Cross-platform packaging; code signing; auto-update support. |
| Testing | **Vitest** | Unit tests for stores and API client. |
| Package manager | **pnpm** | Fast, disk-efficient. |

---

## 10. Package Layout

```
cometline/
│
├── package.json
├── electron-builder.yml            # packaging config (targets, icons, signing)
├── vite.config.ts                  # electron-vite config
├── tsconfig.json
│
├── electron/                       # Main process (Node.js — never in renderer bundle)
│   ├── main.ts                     # app lifecycle, BrowserWindow creation
│   │                               # startup sequence: spawn → health poll → loadURL
│   ├── preload.ts                  # contextBridge: the only renderer↔main bridge
│   └── cometmind.ts                # CometMind child process manager:
│                                   #   spawn(), pollHealth(), kill(), restart()
│                                   #   emits 'cometmind:crashed' to renderer on exit
│
├── src/                            # Renderer process (Svelte, runs in Chromium)
│   ├── App.svelte                  # Root component + router
│   ├── app.css                     # Global styles + Tailwind
│   │
│   ├── lib/
│   │   ├── api/
│   │   │   ├── client.ts           # Typed REST wrappers
│   │   │   ├── sse.ts              # SSE stream consumer
│   │   │   └── types.ts            # Session, Message, AgentEvent, etc.
│   │   │
│   │   ├── stores/
│   │   │   ├── connection.svelte.ts
│   │   │   ├── sessions.svelte.ts
│   │   │   ├── chat.svelte.ts
│   │   │   └── settings.svelte.ts
│   │   │
│   │   └── components/
│   │       ├── ChatMessage.svelte
│   │       ├── ChatInput.svelte
│   │       ├── MessageList.svelte
│   │       ├── StreamingText.svelte
│   │       ├── ToolCallCard.svelte
│   │       ├── TypingIndicator.svelte
│   │       ├── SessionCard.svelte
│   │       ├── SearchInput.svelte
│   │       ├── StatusBar.svelte
│   │       └── Toast.svelte
│   │
│   └── routes/
│       ├── SessionList.svelte
│       ├── ChatView.svelte
│       └── Settings.svelte
│
├── resources/
│   ├── icon.icns                   # macOS icon
│   ├── icon.ico                    # Windows icon
│   └── icon.png                    # Linux icon
│
└── docs/
    └── HLD.md                      # This document
```

---

## 11. Phase 2: Cloud / Hosted Version

**Out of scope for Phase 1.** This section documents the upgrade path if a hosted version is ever built.

Because Electron-specific code is strictly isolated to `electron/` and the preload bridge, the Svelte component tree in `src/` can be reused in a SvelteKit deployment with minimal changes.

| Layer | Phase 1 (Electron) | Phase 2 (Cloud) |
|-------|--------------------|-----------------|
| Desktop shell | Electron | Removed |
| Preload / IPC | `contextBridge` | Removed |
| Persistent settings | `electron-store` | `localStorage` or server session |
| API base URL | `http://127.0.0.1:7700` | Configurable via `PUBLIC_COMETMIND_URL` |
| CometMind server | Local child process | Remote HTTPS |
| Auth | None (localhost only) | Auth layer required |
| Svelte components | Unchanged | Unchanged |
| Stores | Minimal Electron deps | Drop `electron-store` |

No Svelte component, API type, SSE consumer, or store logic needs to change. The investment in Phase 1 is not thrown away.

---

*End of document.*
