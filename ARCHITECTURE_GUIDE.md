# Cometline Release Architecture Guide

This guide is a contributor-oriented map of the whole repository. It explains what each module owns, how data moves through the system, which contracts are load-bearing, and where to start when changing behavior.

## One-Sentence Purpose

Cometline lets a user run a local desktop AI assistant with persistent workspace-scoped sessions, visible streaming reasoning/tool activity, provider switching, and a native-feeling desktop UI while keeping the trusted agent runtime outside the renderer.

The repo succeeds if and only if the desktop shell can safely drive a local agent runtime, persist every turn, stream model/tool progress in real time, and keep provider-specific LLM details isolated behind a stable SDK boundary.

## Repository Topography

```text
cometline-release/
+-- comet-sdk/       Go module: provider-agnostic LLM I/O library
+-- cometmind/       Go module: local agent runtime, SQLite persistence, CLI/API
+-- cometline/       SvelteKit + Electron desktop shell
+-- Makefile         root orchestration for dev/check/build/package
`-- AGENTS.md        repository-specific development rules
```

Dependency direction:

```text
Desktop user
  -> cometline renderer
    -> HTTP/SSE on http://127.0.0.1:7700
      -> cometmind server/runtime/session/tools
        -> comet-sdk Provider interface
          -> Anthropic / OpenAI / OpenAI-compatible APIs

Electron main process
  -> spawns cometmind sidecar binary
  -> persists desktop provider settings
  -> exposes OS/native capabilities over preload IPC
```

The rule that explains most boundaries: `cometline` is the UI shell, `cometmind` is the brain and source of truth, and `comet-sdk` is only the model I/O adapter layer.

## Modules At A Glance

| Module | Runtime | Owns | Must Not Own |
|---|---|---|---|
| `comet-sdk` | Go library | Provider-normalized requests, streaming events, retries, tool-call assembly, typed errors | Agent loops, tools, persistence, sessions |
| `cometmind` | Go binary/library packages | Agent loop, SQLite persistence, workspace/session model, local HTTP/SSE API, CLI, built-in tools | Desktop windowing, renderer state, direct UI transitions |
| `cometline` | Electron main + SvelteKit renderer | Native shell, sidecar lifecycle, settings UI, chat rendering, animations, update flow | Tool execution, provider request construction, database writes |

## Tech Stack By Concern

| Concern | Implementation | Why It Exists | Swappable? |
|---|---|---|---|
| Provider-normalized LLM I/O | Go `comet-sdk` with Anthropic/OpenAI providers | One event/request vocabulary for the agent runtime | Provider implementations are swappable behind `cometsdk.Provider` |
| Streaming collection | `comet-sdk/llm.StreamMessage` | Lets UI render deltas while still assembling the final assistant message | Swappable if it preserves event ordering and final result semantics |
| Agent orchestration | `cometmind/internal/agent.Runner` | Multi-step loop: model call, persist, execute tools, continue | Load-bearing |
| Persistence | SQLite via `modernc.org/sqlite` and sqlc | Local-first durable sessions without native CGO dependency | Storage engine is swappable only behind `session.Service`-equivalent contracts |
| HTTP API | Gin in `cometmind/server` | Localhost REST/SSE contract for desktop renderer | Swappable if the API/SSE contract is preserved |
| CLI | Cobra | Thin command surfaces around shared runtime | Swappable |
| Desktop shell | Electron | Native window, sidecar process, updater, preload IPC | Swappable only if sidecar lifecycle and IPC equivalents remain |
| Renderer | SvelteKit/Svelte 5 + TypeScript | Reactive chat UI, settings UI, route flow | Swappable if REST/SSE contracts and UX state rules are preserved |
| Packaging | electron-builder | macOS DMG/ZIP, extraResources sidecar bundling, updater publishing | Swappable with equivalent sidecar bundling/signing behavior |

## Cross-Module Runtime Contracts

### CometMind Local API

The authoritative local backend surface is under `/api/v1`, registered in `cometmind/server/server.go:64-73` and described by `cometmind/openapi.yaml`.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/health` | Sidecar liveness used by Electron and renderer polling |
| `POST` | `/api/v1/workspaces` | Register or ensure an absolute workspace path |
| `POST` | `/api/v1/sessions` | Create a workspace-scoped session |
| `GET` | `/api/v1/sessions?workspace_path=...` | List sessions for the current workspace |
| `GET` | `/api/v1/sessions/{id}` | Fetch a session resource |
| `DELETE` | `/api/v1/sessions/{id}` | Delete a session and cascade messages/tool calls |
| `GET` | `/api/v1/sessions/{id}/messages` | Load transcript items for UI rendering |
| `POST` | `/api/v1/sessions/{id}/messages` | Send a user turn and receive SSE events |
| `DELETE` | `/api/v1/sessions/{id}/runs/current` | Cancel an in-flight run |

The renderer client for these endpoints is `cometline/src/lib/client/cometmind.ts:12-119`.

### SSE Event Contract

CometMind emits JSON SSE frames whose `type` field is the discriminator. The canonical Go event type is `cometmind/internal/event/event.go:9-146`; the renderer TypeScript union is `cometline/src/lib/types.ts:108-116`.

| Event | Key Fields | Meaning |
|---|---|---|
| `reasoning_start` | `type` | Reasoning block begins |
| `reasoning_delta` | `type`, `text` | Reasoning token chunk |
| `text_delta` | `type`, `delta` | Assistant visible text chunk |
| `tool_call` | `type`, `id`, `tool`, `input` | Model requested a tool |
| `tool_result` | `type`, `id`, `tool`, `output`, `error?` | Tool execution completed |
| `step_finish` | `type`, `usage` | One model step ended |
| `error` | `type`, `message`, `code?` | Runtime/model/tool error |
| `done` | `type` | Terminal stream event |

Changing this contract requires changes in `cometmind/internal/event`, `cometmind/server`, `cometline/src/lib/types.ts`, `cometline/src/lib/reducers/chat.ts`, and tests.

### Electron IPC Contract

The only renderer-to-native bridge is `window.electronAPI`, exposed in `cometline/electron/preload.cjs:3-28` and handled in `cometline/electron/main.cjs:1037-1097`.

| IPC Method | Purpose |
|---|---|
| `restartCometMind()` | Restart the sidecar process |
| `getWorkspacePath()` | Return env, stored, or default workspace path |
| `selectWorkspacePath()` | Open a native directory picker |
| `setWorkspacePath(path)` | Persist chosen workspace path |
| `getProviderSettings()` | Read settings plus env overrides |
| `fetchProviderModels(config)` | Query provider model list from Electron main |
| `saveProviderSettings(settings)` | Persist settings, write CometMind config, restart sidecar |
| `setSidebarOpen(payload)` | Animate native macOS traffic light position |
| `getFullScreen()` / `onFullScreenChange(cb)` | Sync fullscreen state |
| `getAppVersion()` | Read app version |
| `getUpdateState()` / `checkForUpdates()` / `installUpdate()` / `onUpdateState(cb)` | Auto-update UI contract |

The renderer treats this object as optional so browser dev mode still works.

## Core Flows

### Flow 1: Desktop Startup

```text
Electron app ready
  -> ensure ~/.cometmind/cometline-settings.json exists from desktop provider settings
  -> spawn cometmind serve --port 7700 --watch-parent
  -> poll /api/v1/health
  -> create BrowserWindow
  -> load Vite dev server or app://bundle production URL
  -> Svelte layout starts health polling, settings load, workspace/session load
```

Key references:

| Step | Source |
|---|---|
| Port and health constants | `cometline/electron/main.cjs:9-20` |
| Resolve sidecar binary | `cometline/electron/main.cjs:251-262` |
| Ensure shared JSON settings file exists | `cometline/electron/main.cjs` settings read/write helpers |
| Spawn sidecar | `cometline/electron/main.cjs:615-650` |
| Health polling | `cometline/electron/main.cjs:692-703` |
| App ready sequence | `cometline/electron/main.cjs:965-979` |
| Renderer boot sequence | `cometline/src/routes/+layout.svelte:14-58` |

### Flow 2: First Message From The Home Screen

```text
User submits hero composer
  -> renderer creates session with workspace_path, model_id, provider_id
  -> sessionStore queues pending first message
  -> route navigates to /session/{id}
  -> ChatView consumes pending message
  -> startChat coordinates first-turn animation
  -> chatStore streams POST /sessions/{id}/messages
  -> reducer turns SSE events into ChatItem rows
  -> session title refresh runs after turn
```

Key references:

| Step | Source |
|---|---|
| Home route create session and queue message | `cometline/src/routes/+page.svelte:29-40` |
| Route keys `ChatView` by session ID | `cometline/src/routes/session/[id]/+page.svelte:9-18` |
| ChatView consumes pending message | `cometline/src/lib/components/ChatView.svelte:92-109` |
| Turn queue and startChat adapter | `cometline/src/lib/components/ChatView.svelte:65-90` |
| Streaming client | `cometline/src/lib/client/cometmind.ts:68-111` |
| Chat store streaming loop | `cometline/src/lib/stores/chat.svelte.ts:211-289` |
| Pure reducer | `cometline/src/lib/reducers/chat.ts:340-352` |

### Flow 3: CometMind HTTP Turn Execution

```text
POST /api/v1/sessions/{id}/messages
  -> validate JSON and session
  -> construct runner for session/workspace
  -> acquire single in-flight run slot
  -> persist user message and maybe title
  -> set SSE headers
  -> run agent loop in goroutine
  -> write each event as SSE and flush
```

Key references:

| Step | Source |
|---|---|
| Route registration | `cometmind/server/server.go:64-73` |
| Message handler | `cometmind/server/server.go:321-390` |
| Single-run acquisition | `cometmind/server/server.go:344-349` |
| User persistence and title | `cometmind/server/server.go:351-362` |
| SSE loop | `cometmind/server/server.go:375-389` |

### Flow 4: Agent Step And Tool Loop

```text
agent.Run
  -> load SDK messages from SQLite
  -> build cometsdk.Request with tools and system prompt
  -> llm.StreamMessage(provider, request)
  -> forward SDK stream events as CometMind events
  -> collect final assistant message/result
  -> save token usage
  -> persist assistant text, reasoning, and tool-call shells
  -> if stop/max_tokens/no tools: finish
  -> execute each tool in workspace registry
  -> persist tool result message
  -> emit tool_result
  -> repeat until stop or MaxSteps
```

Key references:

| Step | Source |
|---|---|
| Runner and `TurnStore` seam | `cometmind/internal/agent/runner.go:16-37` |
| Main loop | `cometmind/internal/agent/runner.go:41-145` |
| SDK message reconstruction | `cometmind/internal/session/service.go:315-389` |
| Assistant/tool persistence | `cometmind/internal/session/service.go:208-250` |
| Tool result persistence | `cometmind/internal/session/service.go:266-300` |
| Built-in tool registry | `cometmind/internal/tools/registry.go:17-59` |

### Flow 5: Provider Settings Save

```text
Settings modal save
  -> renderer sends provider settings over IPC
  -> Electron normalizes providers
  -> writes ~/.cometmind/cometline-settings.json
  -> writes ~/.cometmind/config.toml for CometMind
  -> gracefully stops sidecar
  -> starts sidecar with provider env
  -> renderer reconnects to health polling
```

Key references:

| Step | Source |
|---|---|
| Settings normalization | `cometline/electron/main.cjs:314-412` |
| Read settings and env override | `cometline/electron/main.cjs:414-466` |
| Write settings | `cometline/electron/main.cjs:468-495` |
| Write CometMind config | `cometline/electron/main.cjs:497-534` |
| Sidecar env | `cometline/electron/main.cjs:536-553` |
| Restart IPC | `cometline/electron/main.cjs:1065-1070` |

### Flow 6: Packaging

```text
pnpm run build:mac or make package
  -> build cometmind sidecar into cometmind/dist/cometmind
  -> build SvelteKit static renderer into cometline/build
  -> electron-builder packages renderer, electron files, and sidecar extraResource
  -> packaged app serves renderer over app://bundle
```

Key references:

| Step | Source |
|---|---|
| Root package target | `Makefile:73-74` |
| `build:sidecar` | `cometline/package.json:12` |
| Electron build scripts | `cometline/package.json:14-17` |
| Sidecar extraResource | `cometline/package.json:89-93` |
| App protocol handler | `cometline/electron/main.cjs:776-801` |

# Module Guide: `comet-sdk`

## Purpose

`comet-sdk` is the provider-agnostic LLM I/O library. It exposes one `Provider` interface and one request/event vocabulary so higher layers do not care whether the model backend is Anthropic, OpenAI, or an OpenAI-compatible gateway.

It deliberately does not own agent loops, tool execution, sessions, persistence, UI, or memory. That separation is stated in `comet-sdk/README.md:12-30` and enforced by the small API surface in `comet-sdk/sdk.go`.

## Package Boundaries

```text
comet-sdk/
+-- sdk.go                         public request/message/block/event/config types
+-- errors.go                      typed auth/rate-limit/server/stream errors
+-- llm/                           convenience collection and streaming assembly
+-- provider/anthropic/            Anthropic Messages API adapter
+-- provider/openai/               OpenAI Chat Completions-compatible adapter
`-- internal/
    +-- providerbase/              HTTP error classification, endpoint/options helpers
    +-- retry/                     exponential retry loop
    `-- sse/                       SSE scanner
```

Dependency shape:

```text
sdk.go public types
  -> llm package uses Provider/Event types
  -> provider packages implement Provider
    -> internal/sse parses wire streams
    -> internal/retry retries pre-stream HTTP calls
    -> internal/providerbase shares endpoint/error/options logic
```

## Public API

The main interface is `cometsdk.Provider` in `comet-sdk/sdk.go:15-26`:

```go
type Provider interface {
    ID() string
    Stream(ctx context.Context, req *Request) (<-chan Event, error)
}
```

The core nouns are:

| Type | Source | Role |
|---|---|---|
| `Request` | `comet-sdk/sdk.go:30-55` | Provider-neutral prompt, messages, tools, system prompt, max tokens, temperature, provider options |
| `Message` | `comet-sdk/sdk.go:59-64` | One conversation turn with content and reasoning blocks |
| `Block` variants | `comet-sdk/sdk.go:75-112` | Text, reasoning, tool call, and tool result payloads |
| `Tool` | `comet-sdk/sdk.go:116-122` | JSON-schema tool definition sent to providers |
| `Event` variants | `comet-sdk/sdk.go:126-228` | Stream event vocabulary used by collectors and CometMind |
| `TokenUsage` | `comet-sdk/sdk.go:232-238` | Per-step token accounting |
| `ProviderConfig` and options | `comet-sdk/sdk.go:242-344` | Base URL, HTTP client, timeout, retry count, auth mode, logger |

Finish reasons are normalized by `NormalizeFinishReason` in `comet-sdk/sdk.go:164-195`, so agent code only branches on `stop`, `tool_use`, `max_tokens`, or `error`.

## `llm` Convenience Layer

The `llm` package is a thin helper layer over `Provider.Stream`.

| Function | Role |
|---|---|
| `Collect` | Drains provider events and builds a final response |
| `GenerateText` | Text-only helper that rejects tool calls |
| `GenerateMessage` | Collects text plus tool calls |
| `QuickText` | One-prompt shortcut |
| `StreamMessage` | Streams events to the caller while assembling a final `GenerateMessageResult` |

`StreamMessage` is the API CometMind uses. It starts provider streaming immediately, forwards all substantive events, suppresses `ErrorEvent`/`DoneEvent` from the public event channel, and returns final errors/results through `Result()` (`comet-sdk/llm/stream.go:47-109`).

Important invariant: callers must drain `Events()` before calling `Result()` (`comet-sdk/llm/stream.go:37-40`, `comet-sdk/llm/stream.go:98-100`). CometMind follows this pattern in `cometmind/internal/agent/runner.go:64-79`.

## Provider Implementations

Anthropic and OpenAI providers share the same shape:

```text
constructor
  -> apply default ProviderConfig and options
  -> convert SDK Request to provider wire request
  -> POST streaming endpoint with retry for pre-stream failures
  -> parse SSE body into SDK Events
  -> close event channel on Done/Error/EOF
```

Anthropic owns native Messages API conversion, Anthropic auth, content block assembly, cache-token usage, and Anthropic stop reason mapping. Its request/stream conversion lives in `comet-sdk/provider/anthropic/convert.go` and streaming loop in `comet-sdk/provider/anthropic/stream.go`.

OpenAI owns Chat Completions-compatible conversion, OpenAI auth, `stream_options.include_usage`, tool-call index assembly, and support for reasoning aliases such as `reasoning_content`. Its conversion lives in `comet-sdk/provider/openai/convert.go` and streaming loop in `comet-sdk/provider/openai/stream.go`.

## Streaming Data Flow

```text
caller
  -> llm.StreamMessage(ctx, provider, req)
    -> provider.Stream(ctx, req)
      -> HTTP POST provider endpoint
      -> SSE scanner reads event/data frames
      -> provider convert.go maps wire payloads to cometsdk.Event values
      -> MessageStream.run forwards events and accumulates final message
  -> caller drains Events()
  -> caller calls Result()
```

`MessageStream.run` accumulates visible text, reasoning text, tool call blocks, finish reason, and usage (`comet-sdk/llm/stream.go:111-186`), then constructs a final assistant message (`comet-sdk/llm/stream.go:189-216`).

## Extension Seams

To add a provider:

1. Create `provider/<name>/client.go`, `convert.go`, and `stream.go`.
2. Implement `cometsdk.Provider`.
3. Use `internal/sse` for SSE parsing if applicable.
4. Use `internal/retry` and `internal/providerbase` for consistent retry/error behavior.
5. Emit only canonical SDK event types.
6. Add fixtures and unit tests.

To add a new stream event type:

1. Add an event struct in `sdk.go`.
2. Map provider wire events to it in provider `convert.go` files.
3. Teach `llm` collectors and `StreamMessage` how to assemble/forward it.
4. Update CometMind event translation if the desktop should see it.

## Invariants

| Invariant | Why It Matters |
|---|---|
| Providers return either a pre-stream error or a channel that closes | Prevents consumers from leaking goroutines |
| Provider finish reasons must be normalized | Agent loop must not know provider-specific stop words |
| `Request.Options` cannot override SDK-managed fields | Prevents callers from breaking required payload shape |
| `StreamMessage.Events()` must be drained before `Result()` | Prevents deadlock |
| OpenAI-compatible providers must preserve usage timing | Usage may arrive after finish reason and before `[DONE]` |
| Anthropic tool call IDs are sanitized before wire use | Anthropic rejects unsupported ID characters |

# Module Guide: `cometmind`

## Purpose

`cometmind` is the local agent runtime. It owns reasoning orchestration, session persistence, workspace scoping, tool execution, and the localhost API consumed by the desktop app.

The README frames it as a general AI agent runtime and describes ACP-based delegation for coding tasks (`cometmind/README.md:1-20`, `cometmind/README.md:56-58`). In the current codebase, the implemented runtime is the built-in multi-step LLM/tool loop in `internal/agent`; there is no separate ACP implementation package visible in this repo yet.

## Package Boundaries

```text
cometmind/
+-- main.go                       entry point, calls cmd.Execute
+-- cmd/                          Cobra commands: init, chat, serve, session
+-- server/                       Gin REST/SSE API and run cancellation manager
+-- openapi.yaml                  API contract source of truth
`-- internal/
    +-- runtime/                  composition root for config, DB, sessions, providers
    +-- agent/                    multi-step LLM/tool runner
    +-- session/                  domain service over sqlc DB queries
    +-- db/                       schema, migrations, generated sqlc files
    +-- config/                   TOML/env config and API key resolution
    +-- provider/                 CometMind config -> comet-sdk provider factory
    +-- tools/                    built-in tool interface, registry, implementations
    +-- tools/sandbox/            workspace path escape prevention
    +-- event/                    runtime event union and JSON wire format
    +-- store/                    SQLite open/pragmas/schema bootstrap
    +-- paths/                    data dir, DB path, config path, workspace resolution
    `-- id/                       ULID generation
```

## Runtime Composition

`internal/runtime` is the shared composition root. `runtime.New` loads config, opens SQLite, applies schema/migrations, and constructs the session service (`cometmind/internal/runtime/runtime.go:30-52`). `RunnerFor` wires a session-specific SDK provider, session service, and workspace-scoped tool registry into an `agent.Runner` (`cometmind/internal/runtime/runtime.go:78-92`).

This keeps CLI and HTTP server thin. Each surface asks runtime for services rather than duplicating setup logic.

## Agent Runner

`agent.Runner` is the load-bearing brain. It depends on a narrow `TurnStore` interface instead of concrete SQLite code (`cometmind/internal/agent/runner.go:16-26`), which makes the loop testable.

The loop in `Run` (`cometmind/internal/agent/runner.go:41-145`) performs these steps:

1. Rebuild conversation history from SQLite as `[]cometsdk.Message`.
2. Build a provider-neutral SDK request with current tools.
3. Call `llm.StreamMessage`.
4. Translate SDK events into CometMind-native events.
5. Persist token usage and assistant content/reasoning/tool-call shells.
6. Stop on `stop`, `max_tokens`, or no tool calls.
7. Execute each requested tool through the workspace registry.
8. Persist tool results and append tool result messages.
9. Continue until stop or `MaxSteps`.

The runner always emits a terminal `done` event through `defer` (`cometmind/internal/agent/runner.go:41-44`).

## Session And Data Model

The SQLite schema is in `cometmind/internal/db/schema.sql:1-51`.

| Table | Purpose |
|---|---|
| `workspaces` | Absolute workspace roots, unique by path |
| `sessions` | Workspace-scoped conversations with model/provider IDs and token usage JSON |
| `messages` | User, assistant, tool result, and optional system rows |
| `tool_calls` | Tool-call shells plus execution output/duration/exit code |

`session.Service` is the domain layer over generated sqlc queries. It owns workspace registration, session lifecycle, message appends, assistant step persistence, tool result persistence, token usage writes, SDK-history reconstruction, and UI transcript reconstruction (`cometmind/internal/session/service.go:24-389`, `cometmind/internal/session/transcript.go:33-104`).

Important persisted formats:

| Field | Format | Source |
|---|---|---|
| `messages.reasoning_content` | JSON array of reasoning block payloads | `cometmind/internal/session/service.go:169-206` |
| `messages.content` for `tool_result` | JSON object `{tool_call_id, content, is_error}` | `cometmind/internal/session/service.go:17-22`, `cometmind/internal/session/service.go:266-290` |
| `sessions.token_usage` | JSON-encoded `cometsdk.TokenUsage` snapshot | `cometmind/internal/session/service.go:303-313` |

Schema migrations are managed with `PRAGMA user_version`; current `schemaVersion` is 2 and v1->v2 adds `reasoning_content` (`cometmind/internal/db/migrate.go:29-36`, `cometmind/internal/db/migrate.go:60-91`). Generated sqlc files under `internal/db` must not be hand-edited.

## HTTP/SSE Server

The server is a Gin app built by `server.New` (`cometmind/server/server.go:39-76`). It takes explicit dependencies in `server.Deps`, including a runner factory and optional `RunManager`.

`handlePostMessage` is the critical endpoint (`cometmind/server/server.go:321-390`). It validates input, loads the session and workspace, builds a runner, acquires the per-session run lock, persists the user message, sets SSE headers, runs the agent in a goroutine, writes every event with `writeSSE`, and flushes.

`RunManager` enforces one active run per session. This prevents overlapping writes and interleaved streams for the same conversation.

Local CORS allows Vite dev origins, localhost, packaged `app://`, `file://`, empty origin, and `null` (`cometmind/server/server.go:78-105`).

## CLI And Server Surfaces

The CLI commands in `cmd/` use the same runtime and session service as the server. `chat` creates or reuses a workspace-scoped session, appends a user message, runs the agent, and prints streamed events.

The HTTP server is the primary app integration surface for Cometline. The important architectural point is that CLI and HTTP do not each implement an agent. They are surfaces over the same `agent.Runner` and `session.Service`.

## Config And Provider Factory

`config.Load` reads `~/.cometmind/config.toml`, creates a default config if missing, and overlays `COMETMIND_*` environment variables (`cometmind/internal/config/config.go:51-105`). Provider methods are defined in `cometmind/internal/config/config.go:14-19`.

`internal/provider.NewFor` resolves a session provider ID to a configured provider entry or legacy provider method, resolves API key, applies base URL, maps `openai-compatible` and `opencode-go` to the SDK OpenAI provider, and constructs the concrete `cometsdk.Provider` (`cometmind/internal/provider/factory.go:18-73`).

## Tools

The tool registry is built per workspace (`cometmind/internal/tools/registry.go:17-33`). It exposes tool specs to the LLM through `CometSDK()` and dispatches tool execution by name through `Execute()` (`cometmind/internal/tools/registry.go:38-59`).

Current built-ins:

| Tool | Purpose |
|---|---|
| `read_file` | Read a text file under the workspace |
| `write_file` | Write a file under the workspace |
| `list_dir` | List a directory under the workspace |
| `run_command` | Execute a shell command in the workspace with guardrails |

File tools are workspace-scoped through `internal/tools/sandbox/pathcheck.go` as described in `AGENTS.md`. When adding tools, preserve workspace isolation and think about permission gates; CometMind currently executes tool calls directly.

## Extension Seams

| Change | Where To Start |
|---|---|
| Add an LLM provider | `comet-sdk/provider/<new>` then `cometmind/internal/provider/factory.go` |
| Add a built-in tool | New `internal/tools/*.go`, then register in `internal/tools/registry.go` |
| Add an API endpoint | `cometmind/server/server.go`, `cometmind/openapi.yaml`, server tests |
| Change DB schema | `internal/db/schema.sql`, `internal/db/migrate.go`, `sqlc generate`, session service updates |
| Change stream event contract | `internal/event/event.go`, server/CLI consumers, renderer types/reducer |
| Change agent loop behavior | `internal/agent/runner.go` and its tests |

## Invariants

| Invariant | Consequence If Broken |
|---|---|
| Sessions are workspace-scoped | Sidebar/session list leaks across projects |
| One in-flight run per session | Interleaved tool results and corrupted transcripts |
| Runner emits `done` for every turn | Renderer/CLI can hang waiting for stream completion |
| Tool calls are persisted before execution results | Tool result messages cannot be linked back to model calls |
| Tool paths cannot escape workspace root | Agent can read/write outside trusted workspace |
| Schema changes need migrations and regenerated sqlc | Existing user DBs break or generated types drift |
| `cometmind` commands run from `cometmind/` module | Go commands fail because there is no root `go.work` |

# Module Guide: `cometline`

## Purpose

`cometline` is the desktop shell and renderer. It owns the native window, sidecar lifecycle, provider settings UI, chat routes, streaming UI reduction, animations, and update flow. It does not own model calls, tool execution, or persistence.

The existing app-specific architecture doc is `cometline/docs/COMETLINE_ARCHITECTURE.md`.

## Runtime Boundaries

```text
Electron main process: cometline/electron/main.cjs
  -> Node/Electron privileges
  -> filesystem, child process, updater, native window, IPC handlers

Preload: cometline/electron/preload.cjs
  -> contextBridge exposes window.electronAPI
  -> the only bridge from renderer to main

SvelteKit renderer: cometline/src
  -> browser-like runtime
  -> talks to CometMind over fetch/SSE
  -> talks to native features only through optional electronAPI methods
```

Security posture: `BrowserWindow` has `contextIsolation: true`, `nodeIntegration: false`, and preload-only native access (`cometline/electron/main.cjs:825-830`).

## Electron Main Process

`electron/main.cjs` owns:

| Concern | Source |
|---|---|
| Default provider settings and model lists | `cometline/electron/main.cjs:43-113` |
| Custom `app://bundle` scheme registration | `cometline/electron/main.cjs:115-129` |
| Native traffic-light animation | `cometline/electron/main.cjs:149-244` |
| Sidecar binary resolution | `cometline/electron/main.cjs:251-262` |
| Settings/config paths | `cometline/electron/main.cjs:264-280` |
| Settings normalization and migration | `cometline/electron/main.cjs:282-412` |
| Settings read/write | `cometline/electron/main.cjs:414-495` |
| Generated CometMind config | `cometline/electron/main.cjs:497-534` |
| Provider env for sidecar | `cometline/electron/main.cjs:536-553` |
| Workspace selection and persistence | `cometline/electron/main.cjs:555-606` |
| Sidecar start/stop | `cometline/electron/main.cjs:615-690` |
| Auto-updater | `cometline/electron/main.cjs:705-766` |
| Packaged bundle serving | `cometline/electron/main.cjs:768-801` |
| Window creation and macOS hide-on-close | `cometline/electron/main.cjs:803-880` |
| Model discovery | `cometline/electron/main.cjs:882-963` |
| App lifecycle | `cometline/electron/main.cjs:965-1035` |
| IPC handlers | `cometline/electron/main.cjs:1037-1097` |

The sidecar stop path waits for process exit before resolving so port 7700 and the SQLite WAL lock are released before a restart (`cometline/electron/main.cjs:652-690`).

## Renderer Routes

| Route | File | Role |
|---|---|---|
| `/` | `cometline/src/routes/+page.svelte` | Hero empty state and first session creation |
| `/session/[id]` | `cometline/src/routes/session/[id]/+page.svelte` | Active chat route, keyed by session ID |
| Root layout | `cometline/src/routes/+layout.svelte` | Starts health polling, loads settings, initializes workspace, loads session list |

The session route keys `ChatView` by `sessionId` so SvelteKit route reuse does not keep stale per-session state (`cometline/src/routes/session/[id]/+page.svelte:9-18`).

## Renderer Stores

All stores are Svelte 5 `$state`-based singletons.

| Store | Owns |
|---|---|
| `chat.svelte.ts` | Active transcript, streaming state, abort controller, transcript loading, SSE event application |
| `session.svelte.ts` | Session list, selected session, pending first-message queue |
| `settings.svelte.ts` | Provider settings, appearance, shortcuts, model fetch/save orchestration |
| `model.svelte.ts` | Flattened provider/model picker options and selected model |
| `shell.svelte.ts` | Sidebar/settings/composer/workspace/fullscreen shell state |
| `runtime.svelte.ts` | Sidecar health polling and connection status |

`chatStore.send` is the central streaming loop (`cometline/src/lib/stores/chat.svelte.ts:211-289`). It sets up an `AbortController`, optionally adds the user bubble, consumes `streamMessage`, applies every event through the pure reducer, and sends a final synthetic `done` in `finally`.

## Renderer Data Types

The frontend contract types are in `cometline/src/lib/types.ts`.

| Type | Source | Mirrors |
|---|---|---|
| `Session` | `cometline/src/lib/types.ts:1-12` | CometMind session resource |
| `ProviderConfig` | `cometline/src/lib/types.ts:37-49` | Electron settings provider entry |
| `ProviderSettings` | `cometline/src/lib/types.ts:80-85` | `cometline-settings.json` shape |
| `TranscriptItem` | `cometline/src/lib/types.ts:96-106` | `GET /sessions/{id}/messages` items |
| `StreamEvent` | `cometline/src/lib/types.ts:108-116` | CometMind SSE frames |
| `ChatItem` | `cometline/src/lib/types.ts:118-140` | Renderer-only row model |

## SSE Parsing And Reduction

`streamMessage` posts to CometMind and yields parsed `StreamEvent` objects (`cometline/src/lib/client/cometmind.ts:68-111`). The chat reducer then transforms stream events into immutable UI state (`cometline/src/lib/reducers/chat.ts:105-287`, `cometline/src/lib/reducers/chat.ts:340-352`).

Important reducer rules:

| Rule | Source |
|---|---|
| API-key/auth errors become user-readable settings hints | `cometline/src/lib/reducers/chat.ts:27-45` |
| Empty assistant placeholders are removed | `cometline/src/lib/reducers/chat.ts:47-51`, `cometline/src/lib/reducers/chat.ts:280-286` |
| Reasoning attaches to the assistant bubble | `cometline/src/lib/reducers/chat.ts:53-69`, `cometline/src/lib/reducers/chat.ts:196-223` |
| Tool call/result rows are matched by tool ID | `cometline/src/lib/reducers/chat.ts:226-260` |
| `step_finish` settles pending state without clearing current assistant | `cometline/src/lib/reducers/chat.ts:262-268` |
| Reducer inputs are cloned, not mutated | `cometline/src/lib/reducers/chat.ts:320-352` |

## Chat UI Components

| Component | Role |
|---|---|
| `AppShell.svelte` | Root chrome, sidebar, settings modal, keyboard shortcuts, traffic-light sync |
| `Sidebar.svelte` | Session list, search, delete confirmation, responsive overlay behavior |
| `ChatView.svelte` | Per-session chat orchestration, first-turn animation, turn queue, transcript load |
| `Composer.svelte` | Textarea, model picker, stop button, queued-message preview |
| `ChatThread.svelte` | Renders `ChatItem` rows |
| `SettingsPanel.svelte` | Providers, shortcuts, appearance, update/workspace controls |
| `RuntimeOverlay.svelte` | Blocks UI while sidecar is connecting |
| `UpdateButton.svelte` | Floating update status/install affordance |

`ChatView` is the orchestration hotspot. It binds the session before render, loads transcript unless a pending first message exists, serializes submissions through `createChatTurnQueue`, starts animations through `startChat`, and calls `chatStore.cancel` on stop (`cometline/src/lib/components/ChatView.svelte:28-160`).

## Settings And Model Discovery

Provider settings currently live in `~/.cometmind/cometline-settings.json`, and Electron writes a generated `~/.cometmind/config.toml` for CometMind. Both are written with `0600` permissions (`cometline/electron/main.cjs:486-490`, `cometline/electron/main.cjs:527-530`).

Model discovery is owned by Electron main, not the renderer:

| Method | Behavior |
|---|---|
| `opencode-go` | Return hardcoded model list |
| `anthropic` | `GET {baseURL}/v1/models` with `x-api-key` |
| `openai` / `openai-compatible` | `GET {baseURL}/models` with bearer auth |

The current architecture doc calls this an MVP gap: long term, provider/model config should move to CometMind-owned endpoints.

## Packaging

`cometline/package.json` defines the packaging pipeline. `build:sidecar` compiles `../cometmind` into `../cometmind/dist/cometmind` (`cometline/package.json:12`). `build:electron`, `build:mac`, and `release:mac` build the renderer and run `electron-builder` (`cometline/package.json:15-17`).

Electron-builder includes the sidecar as an extra resource (`cometline/package.json:89-93`) and packages macOS DMG/ZIP outputs with hardened runtime and notarization enabled (`cometline/package.json:66-78`).

## Invariants

| Invariant | Consequence If Broken |
|---|---|
| Renderer never imports Node APIs directly | Electron security boundary collapses |
| Native access goes through preload IPC | Main/renderer responsibilities blur |
| Chat/session data comes from CometMind REST/SSE | Renderer state becomes an alternate source of truth |
| Sidecar restart waits for process exit | Port 7700 or SQLite WAL lock can remain held |
| `app://bundle` must serve SPA fallback in production | Reloading `/session/{id}` in packaged app breaks |
| Stream event reducer must publish new item references | Svelte may not render live token updates |
| First message route handoff uses pending-message queue | New-session first send races with transcript load/navigation |
| Provider settings contain secrets and must remain `0600` | API keys become easier to leak |

# Where To Start For Common Changes

| Goal | Start Here | Also Check |
|---|---|---|
| Add a new backend model provider | `comet-sdk/provider`, `cometmind/internal/provider/factory.go`, `cometline` settings/model fetch | SDK tests, CometMind config tests, renderer settings UI |
| Add a new agent tool | `cometmind/internal/tools`, `registry.go` | Workspace sandbox, permission-gate design, transcript rendering |
| Change chat streaming UI | `cometline/src/lib/reducers/chat.ts`, `chat.svelte.ts`, `ChatThread.svelte` | CometMind event contract and reducer tests |
| Change persistence schema | `cometmind/internal/db/schema.sql`, `migrate.go` | `sqlc generate`, session service, server transcript tests |
| Add a REST endpoint | `cometmind/server/server.go`, `openapi.yaml` | Renderer client if UI needs it |
| Change provider settings UX | `cometline/src/lib/stores/settings.svelte.ts`, `SettingsPanel.svelte`, `electron/main.cjs` | Generated `config.toml`, sidecar restart behavior |
| Change packaging/release | `cometline/package.json`, `.github/workflows`, `electron/main.cjs` | Sidecar `extraResources`, update flow |
| Improve secrets storage | `electron/main.cjs` and future CometMind config endpoints | OS keychain design, renderer redaction |

# Verification Commands

Run commands from module directories where required. There is no root `go.work`.

```bash
cd comet-sdk && go test ./...
cd comet-sdk && go build ./...

cd cometmind && go test ./...
cd cometmind && go build ./...

cd cometline && pnpm run check
cd cometline && pnpm run test
cd cometline && pnpm run build

make check
make build
make dev
```

Live provider tests in `comet-sdk` require API keys and the `live` build tag.

# Load-Bearing Walls

The essential complexity of this project is the streaming, persisted, workspace-scoped agent loop behind a local desktop UI: provider-specific SSE becomes SDK events, SDK events become CometMind runtime events, runtime events are persisted and streamed over HTTP/SSE, and the renderer reduces those events into a live chat transcript. Any version of the project must preserve that chain. Everything else, including Electron, SvelteKit, Gin, SQLite, and individual provider implementations, is a swappable implementation choice if the contracts and invariants above stay intact.
