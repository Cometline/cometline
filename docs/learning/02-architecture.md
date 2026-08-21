# 02 — Architecture and Boundaries

> **Prerequisite:** [01-nutshell.md](./01-nutshell.md)  
> **Next:** [03-data-flows.md](./03-data-flows.md)

## One-sentence purpose

Cometline lets you run a local desktop AI assistant with persistent workspace-scoped sessions, visible streaming reasoning/tool activity, semantic memory, jobs, provider switching, and external tool integrations — while keeping the trusted agent runtime **outside** the renderer.

## Repository topography

```text
cometline/          (monorepo root)
├── comet-sdk/      Go module — LLM I/O library
├── cometmind/      Go module — agent runtime, CLI, HTTP API
├── cometline/      SvelteKit + Electron desktop shell
├── Makefile        Root orchestration (install, check, build, dev)
├── ARCHITECTURE.md
├── ARCHITECTURE_GUIDE.md
└── docs/learning/  ← you are here
```

There is **no root `go.work`**. Run Go commands from `comet-sdk/` or `cometmind/`, not the repo root.

## Module ownership matrix

| Module        | Owns                                                                                                            | Must NOT own                                       |
| ------------- | --------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| **comet-sdk** | Provider requests/responses, SSE parsing, tool-call delta assembly, retries, typed errors                       | Agent loops, persistence, UI, tool execution       |
| **cometmind** | Agent loop, SQLite, workspaces/sessions/jobs, built-in tools, memory, MCP/ACP/Discord, scheduler, localhost API | Windowing, renderer state, provider wire quirks    |
| **cometline** | Native shell, sidecar lifecycle, settings UI, chat/jobs/file rendering, mini routes, animations, auto-update    | Tool execution, provider requests, database writes |

## Dependency direction

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

**Rule:** `cometline` may call CometMind over REST/SSE and Electron IPC, but it must not become a second runtime.

## Tech stack by concern

| Concern              | Implementation                                             | Swappable?                               |
| -------------------- | ---------------------------------------------------------- | ---------------------------------------- |
| LLM I/O              | `comet-sdk` + provider packages                            | Yes, behind `Provider` interface         |
| Streaming collection | `comet-sdk/llm.StreamMessage`                              | Yes, if event ordering preserved         |
| Agent orchestration  | `cometmind/internal/agent.Runner`                          | Load-bearing                             |
| Persistence          | SQLite (`modernc.org/sqlite`) + sqlc                       | Behind session service contract          |
| HTTP API             | Gin (`cometmind/server`)                                   | Yes, if OpenAPI contract preserved       |
| Desktop shell        | Electron                                                   | Yes, if sidecar + IPC equivalents remain |
| Renderer             | SvelteKit 5 + TypeScript                                   | Yes, if REST/SSE contracts preserved     |
| Jobs/scheduler       | `internal/jobs`, `internal/scheduler`, `internal/autonomy` | Load-bearing once jobs are persisted     |
| MCP client           | `internal/mcp` + MCP Go SDK                                | Yes, behind the tool registry surface    |

## Cross-module contracts

Three contracts glue the modules together. Changing any of them requires coordinated updates across layers.

### 1. HTTP API (`cometmind/openapi.yaml`)

CometMind serves `/api/v1/*` on localhost. Core endpoints:

| Method         | Path                                  | Purpose                                    |
| -------------- | ------------------------------------- | ------------------------------------------ |
| `GET`          | `/api/v1/health`                      | Sidecar liveness                           |
| `POST`         | `/api/v1/workspaces`                  | Register workspace path                    |
| `POST`         | `/api/v1/sessions`                    | Create session                             |
| `GET`          | `/api/v1/sessions?workspace_path=...` | List sessions                              |
| `GET`          | `/api/v1/sessions/{id}/messages`      | Load transcript                            |
| `POST`         | `/api/v1/sessions/{id}/messages`      | Send message → SSE stream                  |
| `DELETE`       | `/api/v1/sessions/{id}/runs/current`  | Cancel in-flight run                       |
| `GET` / `PUT`  | `/api/v1/workspaces/files/content`    | Preview or edit small workspace files      |
| `GET`          | `/api/v1/mcp/servers`                 | MCP connection status                      |
| `GET` / `POST` | `/api/v1/jobs`                        | Jobs board data                            |
| `GET` / `POST` | `/api/v1/scheduled-jobs`              | Deferred/recurring jobs                    |
| `GET`          | `/api/v1/events`                      | Runtime SSE (memory toasts, compaction, …) |
| `POST`         | `/api/v1/memories/compaction-runs`    | Manual memory compaction                   |

Renderer client: `cometline/src/lib/client/cometmind.ts`

### 2. SSE event contract

CometMind emits JSON frames with a `type` discriminator. Full catalog (OpenAPI `StreamEvent` + `event/event.go`):

| Event                                                          | Meaning                                                      |
| -------------------------------------------------------------- | ------------------------------------------------------------ |
| `reasoning_start` / `reasoning_delta`                          | Thinking tokens                                              |
| `text_delta`                                                   | Visible assistant text                                       |
| `tool_call`                                                    | Model requested a tool                                       |
| `tool_result`                                                  | Tool finished                                                |
| `step_finish`                                                  | One model step ended (includes usage)                        |
| `subagent_started` / `subagent_progress` / `subagent_finished` | Coding harness / general subagents                           |
| `memory_injected` / `memory_updated`                           | Memory retrieval and extraction                              |
| `memory_compaction_completed`                                  | Memory compaction finished (often via `/events`)             |
| `context_budget`                                               | Context-window budget telemetry                              |
| `inbox_message_created` / `inbox_message_archived`            | Runtime-wide inbox lifecycle notifications                   |
| `assistant_image`                                              | Persisted assistant media, fetched from a session-media URL   |
| `turn_status`                                                  | Pre-output status (retrieving memories, contacting model, …) |
| `turn_recover`                                                 | Partial stream recovery after mid-turn failure               |
| `error`                                                        | Failure                                                      |
| `done`                                                         | Stream terminal                                              |

Session chat streams come from `POST …/messages`. Non-chat runtime feedback also uses `GET /api/v1/events`.

- Go source: `cometmind/internal/event/event.go`
- TS types: `cometline/src/lib/types.ts`
- Chat reducer: `cometline/src/lib/reducers/chat.ts`
- Runtime toasts: `cometline/src/lib/stores/memory-toasts.svelte.ts`

### 3. Electron IPC (`window.electronAPI`)

Exposed via `cometline/electron/src/preload.ts`, typed in `electron/src/shared/api.ts`, and handled by the domains composed from `electron/src/domains/runtime.ts`:

| Method                                         | Purpose                                |
| ---------------------------------------------- | -------------------------------------- |
| `getProviderSettings` / `saveProviderSettings` | Settings merge/split persistence       |
| `fetchProviderModels`                          | Model list from provider API           |
| Codex / xAI auth helpers                       | Subscription session sign-in           |
| `getWorkspacePath` / `setWorkspacePath`        | Workspace management                   |
| `restartCometMind`                             | Full sidecar restart (rare)            |
| `checkForUpdates` / `installUpdate`            | Auto-update                            |
| MCP OAuth                                      | CometMind `POST /api/v1/mcp/servers/{id}/oauth-flows` |
| `readCursorMcpConfig`                          | Import Cursor-style MCP config         |
| `notifyJob`                                    | Desktop notifications for job changes  |

The renderer treats `electronAPI` as optional so browser-only dev mode still works.

## cometmind internal package map

```text
cometmind/
├── main.go, cmd/           CLI: init, serve, chat, session, gateway
├── server/                 Gin HTTP/SSE API, RunManager (messages in messages.go)
├── openapi.yaml            API contract (source of truth)
└── internal/
    ├── runtime/            Composition root (config, DB, sessions, providers)
    ├── agent/              Multi-step LLM/tool runner
    ├── session/            Domain service over sqlc queries
    ├── db/                 Schema, migrations, generated sqlc
    ├── config/             JSON/TOML/env config
    ├── provider/           Config → comet-sdk factory
    ├── tools/              Built-in tool registry + surfaces + sandbox
    ├── acp/                Coding-harness CLI profiles (delegate_coding_task)
    ├── subagent/           In-process subagent orchestration
    ├── memory/             Semantic memory (retrieve, extract, compact)
    ├── mcp/                MCP client manager + OAuth
    ├── skills/             Agent Skills discovery and drafts
    ├── jobs/               Durable jobs, leases, events, settings
    ├── scheduler/          One-shot and cron scheduled jobs
    ├── autonomy/           Autonomous job worker
    ├── settingsapply/      Reload vs gateway vs restart classify
    ├── processctl/         Long-running process modes (serve, gateway)
    ├── retention/          Storage cleanup / age purge
    ├── gateway/            Discord (and future) gateways
    └── event/              SSE event types
```

`internal/runtime` is the composition root — CLI and HTTP server both call `runtime.New()` rather than duplicating setup.

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
    ├── providerbase/   Shared HTTP/error/options
    ├── retry/          Exponential backoff
    └── sse/            SSE scanner
```

## cometline package map

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
│           ├── settings.ts     Settings persistence
│           └── cometmind-lifecycle.ts  Sidecar lifecycle
└── src/
    ├── routes/         SvelteKit pages (/ , /session/[id], /jobs, /skill-drafts, /mini, /settings)
    ├── lib/
    │   ├── client/     cometmind.ts HTTP/SSE client
    │   ├── stores/     chat, session, settings, model, shell, runtime, memory-toasts
    │   ├── reducers/   chat.ts pure SSE → state
    │   ├── components/ ChatView, Composer, JobsPage, settings/* panels, …
    │   ├── jobs/       Job prompts, notifications, board helpers
    │   └── settings/   schema.ts validation/normalization
    └── app.html
```

## Load-bearing invariants

Violating these causes subtle, hard-to-debug failures:

| Invariant                                   | If broken…                                 |
| ------------------------------------------- | ------------------------------------------ |
| Sessions are workspace-scoped               | Chat history leaks across projects         |
| One in-flight run per session               | Interleaved streams, corrupted transcripts |
| Runner always emits `done`                  | UI hangs waiting for stream end            |
| Tool calls persisted before results         | Tool results can't link to model calls     |
| Tool paths cannot escape workspace          | Agent reads/writes outside project         |
| Schema changes need migrations + sqlc regen | Existing user DBs break                    |
| Sidecar restart waits for process exit      | Port 7700 or SQLite WAL lock held          |
| Reducer publishes new object references     | Svelte won't re-render live tokens         |
| Renderer never imports Node APIs            | Security boundary collapses                |

## Extension seams (where to plug in)

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

[03-data-flows.md](./03-data-flows.md) walks through each major flow step by step with diagrams — startup, first message, agent loop, settings save, and packaging.
