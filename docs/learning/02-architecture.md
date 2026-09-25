# 02 — Architecture and Boundaries

> **Prerequisite:** [01-nutshell.md](./01-nutshell.md)  
> **Next:** [03-data-flows.md](./03-data-flows.md)

This page uses plain English (about B2). Sentences are short. A new word is explained the first time it appears.

## One-sentence purpose

Cometline is a local desktop AI assistant. Sessions are saved, and each session belongs to one workspace. You can watch reasoning and tool activity as they stream in. It also has semantic memory, jobs, provider switching, and links to external tools. The trusted agent runtime stays **outside** the renderer.

A **renderer** is the window process that draws the user interface. A **runtime** is the program that runs the agent. **Semantic** means by meaning, not only by the same words.

## Repository topography

A monorepo is one git repository that holds several modules. I/O means input and output. CLI means command-line interface. The shell is the Electron app around the UI.

```text
cometline/          (monorepo root)
├── comet-sdk/      Go module: LLM I/O library
├── cometmind/      Go module: agent runtime, CLI, HTTP API
├── cometline/      SvelteKit + Electron desktop shell
├── Makefile        Root commands (install, check, build, dev)
├── ARCHITECTURE.md
├── ARCHITECTURE_GUIDE.md
└── docs/learning/  ← you are here
```

There is **no root `go.work`**. Run Go commands from `comet-sdk/` or `cometmind/`. Do not run them from the repo root.

## Module ownership matrix

**SSE** means Server-Sent Events. The server pushes events on one open connection. **Persistence** means saving data so it remains after a restart. A **sidecar** is a helper process. The desktop app starts it and stops it.

**MCP** means Model Context Protocol. It connects external tool servers. A harness is an external coding program with a fixed command profile. **ACP** is CometMind's support for that harness.

Windowing means creating and managing OS windows. Wire means the request format on the network. A quirk is a provider-specific difference in that format. Lifecycle means start, run, and stop. Delta assembly means joining partial tool-call pieces into one tool call.

| Module        | Owns                                                                                                            | Must NOT own                                       |
| ------------- | --------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| **comet-sdk** | Provider requests/responses, SSE parsing, tool-call delta assembly, retries, typed errors                       | Agent loops, persistence, UI, tool execution       |
| **cometmind** | Agent loop, SQLite, workspaces/sessions/jobs, built-in tools, memory, MCP/ACP/Discord, scheduler, localhost API | Windowing, renderer state, provider wire quirks    |
| **cometline** | Native shell, sidecar lifecycle, settings UI, chat/jobs/file rendering, mini routes, animations, auto-update    | Tool execution, provider requests, database writes |

## Dependency direction

**IPC** means inter-process communication. It is how Electron's processes talk to each other.

```text
Desktop user
  → cometline renderer (SvelteKit)
    → HTTP/SSE http://127.0.0.1:7700
      → cometmind (agent, sessions, tools)
        → comet-sdk Provider interface
          → Anthropic / OpenAI / Codex / xAI / compatible APIs

Electron main
  → spawns cometmind sidecar binary
  → persists cometline-settings.json + cometline-desktop.json (split on write)
  → exposes OS capabilities via preload IPC
```

Spawns means starts. Persists means saves. Exposes means makes available. A preload script runs before the page.

**Rule:** `cometline` may call CometMind over REST/SSE and Electron IPC. It must not become a second runtime.

## Tech stack by concern

A **contract** is a shared description of an API or data shape. Both sides must follow it. Swappable means you could replace that part if you keep the contract. A surface is the set of tools other code is allowed to use. Agent orchestration means the loop that runs model steps and tools. Gin is the HTTP library used by the API server. sqlc generates Go code from SQL.

| Concern              | Implementation                                             | Swappable?                               |
| -------------------- | ---------------------------------------------------------- | ---------------------------------------- |
| LLM I/O              | `comet-sdk` + provider packages                            | Yes, behind the `Provider` interface     |
| Streaming collection | `comet-sdk/llm.StreamMessage`                              | Yes, if event order stays the same       |
| Agent orchestration  | `cometmind/internal/agent.Runner`                          | No. The system depends on this part.     |
| Persistence          | SQLite (`modernc.org/sqlite`) + sqlc                       | Only behind the session service contract |
| HTTP API             | Gin (`cometmind/server`)                                   | Yes, if the OpenAPI contract stays the same |
| Desktop shell        | Electron                                                   | Yes, if the sidecar and the same IPC remain |
| Renderer             | SvelteKit 5 + TypeScript                                   | Yes, if REST and SSE contracts stay the same |
| Jobs/scheduler       | `internal/jobs`, `internal/scheduler`, `internal/autonomy` | No, once jobs are saved. The system depends on this part. |
| MCP client           | `internal/mcp` + MCP Go SDK                                | Yes, behind the tool registry surface    |

## Cross-module contracts

Three contracts connect the modules. If you change one, update every layer that uses it.

### 1. HTTP API (`cometmind/openapi.yaml`)

CometMind serves `/api/v1/*` on localhost. These are the core endpoints.

**Compaction** means turning a long history into a shorter summary. A toast is a small notice that appears for a short time. A transcript is the saved chat. In-flight means still running.

| Method         | Path                                  | Purpose                                    |
| -------------- | ------------------------------------- | ------------------------------------------ |
| `GET`          | `/api/v1/health`                      | Check that the sidecar is running          |
| `POST`         | `/api/v1/workspaces`                  | Register a workspace path                  |
| `POST`         | `/api/v1/sessions`                    | Create a session                           |
| `GET`          | `/api/v1/sessions?workspace_path=...` | List sessions                              |
| `GET`          | `/api/v1/sessions/{id}/messages`      | Load the transcript                        |
| `POST`         | `/api/v1/sessions/{id}/messages`      | Send a message and open an SSE stream      |
| `DELETE`       | `/api/v1/sessions/{id}/runs/current`  | Cancel the in-flight run                   |
| `GET` / `PUT`  | `/api/v1/workspaces/files/content`    | Preview or edit small workspace files      |
| `GET`          | `/api/v1/mcp/servers`                 | MCP connection status                      |
| `GET` / `POST` | `/api/v1/jobs`                        | Data for the jobs board                    |
| `GET` / `POST` | `/api/v1/scheduled-jobs`              | Jobs that run later, or on a schedule      |
| `GET`          | `/api/v1/events`                      | Runtime SSE (memory toasts, compaction, and other events) |
| `POST`         | `/api/v1/memories/compaction-runs`    | Start memory compaction on request         |

Renderer client: `cometline/src/lib/client/cometmind.ts`

### 2. SSE event contract

CometMind sends JSON frames. Each frame has a `type` field. That field names the event. The full list is OpenAPI `StreamEvent` and `event/event.go`.

A **subagent** is a child agent started by the main agent. A general subagent runs inside CometMind, not in an external program. A **token** is a small piece of text. A context window is the model's space limit for one request.

| Event                                                          | Meaning                                                      |
| -------------------------------------------------------------- | ------------------------------------------------------------ |
| `reasoning_start` / `reasoning_delta`                          | Thinking tokens                                              |
| `text_delta`                                                   | Visible assistant text                                       |
| `tool_call`                                                    | The model asked for a tool                                   |
| `tool_result`                                                  | The tool finished                                            |
| `step_finish`                                                  | One model step ended. The event includes usage.              |
| `subagent_started` / `subagent_progress` / `subagent_finished` | Coding harness or general subagents |
| `memory_injected` / `memory_updated`                           | Memories were loaded, or new memories were saved             |
| `memory_compaction_completed`                                  | Memory compaction finished. This often arrives on `/events`. |
| `context_budget`                                               | A measured report of context-window space                    |
| `inbox_message_created` / `inbox_message_archived`            | Notices for the whole runtime when an inbox message is created or archived |
| `assistant_image`                                              | Saved assistant media. The app loads it from a session-media URL. |
| `turn_status`                                                  | Status before the reply. Examples: loading memories, contacting the model, and others. |
| `turn_recover`                                                 | Recovers a partial stream after a failure in the middle of a turn |
| `error`                                                        | Failure                                                      |
| `done`                                                         | The stream has ended                                         |

Session chat streams come from `POST …/messages`. Notices that are not part of that chat also use `GET /api/v1/events`.

- Go source: `cometmind/internal/event/event.go`
- TS types: `cometline/src/lib/types.ts`
- Chat reducer: `cometline/src/lib/reducers/chat.ts`. A **reducer** turns events into UI state.
- Runtime toasts: `cometline/src/lib/stores/memory-toasts.svelte.ts`

### 3. Electron IPC (`window.electronAPI`)

`cometline/electron/src/preload.ts` exposes this API. Types are in `electron/src/shared/api.ts`. The domains set up by `electron/src/domains/runtime.ts` handle the calls.

**OAuth** is a sign-in flow for a protected remote server.

| Method                                         | Purpose                                |
| ---------------------------------------------- | -------------------------------------- |
| `getProviderSettings` / `saveProviderSettings` | Read and save settings. The save step merges data, then splits it into the two JSON files. |
| `fetchProviderModels`                          | Load the model list from the provider API |
| Codex / xAI auth helpers                       | Sign in with a subscription session    |
| `getWorkspacePath` / `setWorkspacePath`        | Get or set the workspace path          |
| `restartCometMind`                             | Full sidecar restart. This is rare.    |
| `checkForUpdates` / `installUpdate`            | Check for an update, or install one    |
| MCP OAuth                                      | CometMind `POST /api/v1/mcp/servers/{id}/oauth-flows` |
| `readCursorMcpConfig`                          | Import an MCP config in Cursor's format |
| `notifyJob`                                    | Desktop notices when jobs change       |

The renderer treats `electronAPI` as optional. Browser-only dev mode still works without it.

## cometmind internal package map

A composition root is the one place that builds the main parts.

```text
cometmind/
├── main.go, cmd/           CLI: init, serve, chat, session, gateway
├── server/                 Gin HTTP/SSE API, RunManager (messages in messages.go)
├── openapi.yaml            API contract (the main source)
└── internal/
    ├── runtime/            Composition root (config, DB, sessions, providers)
    ├── agent/              Multi-step LLM and tool runner
    ├── session/            Domain service over sqlc queries
    ├── db/                 Schema, migrations, generated sqlc
    ├── config/             JSON, TOML, and env config
    ├── provider/           Config → comet-sdk factory (builds the provider)
    ├── tools/              Built-in tool registry, surfaces, and sandbox
    ├── acp/                Fixed CLI profiles for delegate_coding_task
    ├── subagent/           Controls subagents inside this process
    ├── memory/             Semantic memory: retrieve, extract, compact
    ├── mcp/                MCP client manager and OAuth
    ├── skills/             Agent Skills discovery and drafts
    ├── jobs/               Saved jobs, leases (short worker locks), events, settings
    ├── scheduler/          One-shot (run once) and cron (repeated schedule) jobs
    ├── autonomy/           Job worker that can start work on its own
    ├── settingsapply/      Chooses reload, gateway, or full restart
    ├── processctl/         Long-running modes: serve and gateway
    ├── retention/          Storage cleanup and delete-by-age
    ├── gateway/            Discord gateway now, and other gateways later
    └── event/              SSE event types
```

`internal/runtime` is the composition root. The CLI and the HTTP server both call `runtime.New()`. They do not set up those parts a second time.

In `tools/`, that surface is the set of tool families one agent may use. A sandbox limits tool file access to the workspace.

## comet-sdk package map

```text
comet-sdk/
├── sdk.go              Public types: Request, Message, Block, Event, Provider
├── errors.go           Typed errors (auth, rate limit, server, stream)
├── llm/                StreamMessage, Collect, GenerateText
├── provider/
│   ├── anthropic/      Messages API adapter
│   ├── openai/         Chat Completions adapter
│   ├── codex/          ChatGPT Codex adapter
│   └── xai/            xAI Grok subscription adapter
└── internal/
    ├── providerbase/   Shared HTTP, errors, and options
    ├── retry/          Exponential backoff (each retry waits longer)
    └── sse/            SSE scanner
```

## cometline package map

ESM is the JavaScript module format used by the Electron main process. Pure means the reducer only computes the next state. Validation means checking values. Normalization means putting values into a standard form.

```text
cometline/
├── electron/
│   └── src/
│       ├── main.ts             ESM entrypoint → app.ts
│       ├── preload.ts          contextBridge → electronAPI
│       ├── shared/api.ts       Typed ElectronAPI contract
│       └── domains/
│           ├── runtime.ts      Main-process composition root
│           ├── runtime-ipc.ts  IPC handler composition
│           ├── settings.ts     Settings load and save
│           └── cometmind-lifecycle.ts  Sidecar start, run, and stop
└── src/
    ├── routes/         SvelteKit pages (/ , /session/[id], /jobs, /skill-drafts, /mini, /settings)
    ├── lib/
    │   ├── client/     cometmind.ts HTTP/SSE client
    │   ├── stores/     chat, session, settings, model, shell, runtime, memory-toasts
    │   ├── reducers/   chat.ts pure SSE → state
    │   ├── components/ ChatView, Composer, JobsPage, settings/* panels, …
    │   ├── jobs/       Job prompts, notifications, board helpers
    │   └── settings/   schema.ts validation and normalization
    └── app.html
```

## Load-bearing invariants

An **invariant** is a rule that must stay true. If you break these rules, the failures are easy to miss. They are also hard to find.

Workspace-scoped means each session belongs to one workspace. Emits means sends. Persisted means saved. Escape means leave the workspace folder.

**WAL** is SQLite's write-ahead log. If its lock is still held, other writers must wait.

| Invariant                                   | If broken…                                 |
| ------------------------------------------- | ------------------------------------------ |
| Sessions are workspace-scoped               | Chat history from one project appears in another |
| One in-flight run per session               | Streams mix together. Saved transcripts can be damaged. |
| Runner always emits `done`                  | The UI stays stuck, waiting for the stream to end |
| Tool calls persisted before results         | Tool results cannot be linked to the model calls |
| Tool paths cannot escape workspace          | The agent reads or writes files outside the project |
| Schema changes need migrations + sqlc regen | Databases already on user machines break   |
| Sidecar restart waits for process exit      | Port 7700 stays in use, or the SQLite WAL lock stays held |
| Reducer publishes new object references     | Svelte will not redraw live tokens         |
| Renderer never imports Node APIs            | The security boundary around Node APIs fails |

## Extension seams (where to plug in)

A seam is a place where you can add a feature. You start at the files in the right column.

| Change            | Start here                                                                                               |
| ----------------- | -------------------------------------------------------------------------------------------------------- |
| New LLM provider  | `comet-sdk/provider/<name>` → `cometmind/internal/provider/factory.go` → `SettingsProvidersPanel.svelte` |
| New built-in tool | `cometmind/internal/tools/*.go` → `registry.go` / `surface.go`                                           |
| New API endpoint  | `server/server.go` + `openapi.yaml` → `make generate`                                                    |
| New SSE event     | `event/event.go` + `openapi.yaml` → reducer and/or runtime toasts + contract tests                       |
| DB schema change  | `db/schema.sql` + `migrate.go` → `sqlc generate`                                                         |
| Settings field    | `settings/schema.ts` + settings panel module + Electron split path                                       |
| Jobs behavior     | `internal/jobs` / `scheduler` / `autonomy` + OpenAPI + `/jobs` UI                                        |
| MCP behavior      | `internal/mcp` + settings MCP panel + Electron OAuth IPC                                                 |
| Coding harness    | `internal/acp` + Settings → Coding task delegation                                                       |

## What's next

[03-data-flows.md](./03-data-flows.md) explains each major flow, step by step, with diagrams. The flows are startup, the first message, the agent loop, settings save, and packaging.
