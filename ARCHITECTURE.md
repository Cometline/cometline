# Architecture

Cometline is a three-layer system: a desktop chat UI, a local agent runtime, and a provider-agnostic LLM library. Each layer has clear responsibilities and communicates through well-defined contracts.

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Desktop User                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  cometline (Electron + SvelteKit)                               │
│  - Chat UI, settings, animations                                │
│  - Sidecar lifecycle management                                 │
│  - Native OS integration (tray, shortcuts, auto-update)         │
└─────────────────────────────────────────────────────────────────┘
                              │ HTTP/SSE (127.0.0.1:7700)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  cometmind (Go runtime)                                         │
│  - Agent loop (up to 100 work steps + final answer)             │
│  - SQLite persistence (sessions, messages, memories)            │
│  - Built-in tools (file ops, commands, web fetch)               │
│  - Coding-harness CLI delegation (OpenCode, Claude Code, Codex) │
│  - MCP client manager and OAuth token refresh                    │
│  - Jobs, scheduled jobs, and autonomous job worker               │
│  - Discord gateway                                              │
│  - Semantic memory (retrieval, extraction, compaction)          │
└─────────────────────────────────────────────────────────────────┘
                              │ Provider interface
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  comet-sdk (Go library)                                         │
│  - Streaming LLM I/O (Anthropic, OpenAI, Codex, xAI, compatible)│
│  - Retry logic with exponential backoff                         │
│  - Tool-call assembly and delta tracking                        │
│  - Typed errors (auth, rate limit, server, stream)              │
└─────────────────────────────────────────────────────────────────┘
```

## Module Responsibilities

### cometline (Desktop Shell)

**Owns:**
- Chat UI rendering (streaming text, reasoning blocks, tool calls)
- Settings management (providers, models, appearance, shortcuts)
- Session navigation, workspace switching, and the per-session workspace panel
- Multimodal input, persisted message-context chips, and assistant image rendering
- Native desktop features (tray icon, keyboard shortcuts, auto-update, screen-capture permission/bridge, and workspace file watching)
- Jobs board, skill draft editor, settings route, mini-window routes, and the multi-surface workspace panel (wiki, workspace files, Git changes and full diffs, web, and terminal)

**Must not own:**
- Tool execution
- Provider request construction
- Database writes
- Agent loop logic

**Key files:**
- `src/lib/components/composer/Composer.svelte` — message input with model picker
- `src/lib/components/ChatView.svelte` — conversation rendering
- `src/lib/stores/` — reactive state (chat, session, model, settings)
- `src/lib/client/cometmind.ts` — HTTP/SSE client for CometMind API
- `electron/src/domains/runtime.ts` — Electron composition root; wires sidecar lifecycle, settings, IPC, windows, and native domains
- `electron/src/domains/` — focused native integrations: screen capture, workspace watching, terminal, updater, personas, and window chrome

### cometmind (Agent Runtime)

**Owns:**
- Agent orchestration (model call → persist → execute tools → continue)
- Session and workspace management
- SQLite persistence (messages, tool calls, memories)
- Built-in tools (file operations, command execution, web fetch)
- Coding-harness CLI delegation to external coding agents
- MCP server lifecycle and remote tool execution
- Jobs, scheduled jobs, job leases, job maintenance, and autonomous job execution
- Discord messaging gateway
- Semantic memory (auto-retrieve before turns, auto-extract after)

**Must not own:**
- Desktop windowing or UI state
- Direct renderer transitions
- Provider-specific request formatting (delegated to comet-sdk)

**Key files:**
- `internal/agent/runner.go` — multi-step agent loop
- `server/server.go` — HTTP/SSE API registration
- `internal/event/event.go` — SSE event types and emission
- `internal/tools/registry.go` — built-in tool registration
- `internal/db/` — SQLite schema and queries (sqlc-generated)
- `internal/memory/` — semantic memory service
- `internal/mcp/` — MCP client manager, bindings, OAuth login, and token persistence
- `internal/jobs/`, `internal/scheduler/`, `internal/autonomy/` — durable jobs, schedule materialization, and background workers

### comet-sdk (LLM I/O Library)

**Owns:**
- Provider-normalized requests and responses
- Streaming event emission (text deltas, reasoning, tool calls)
- Retry logic (429, 5xx, Anthropic 529)
- Tool-call assembly from streaming deltas
- Token usage tracking
- Typed errors

**Must not own:**
- Agent loops or tool execution
- Session persistence
- UI concerns

**Key files:**
- `sdk.go` — public API entry points
- `llm/stream.go` — streaming message implementation
- `provider/anthropic/` — Anthropic provider
- `provider/openai/` — OpenAI and compatible providers

## Data Flow

### User sends a message

```
1. User types in Composer.svelte and hits Enter
   ↓
2. Cometline POSTs to /api/v1/sessions/{id}/messages
   ↓
3. CometMind receives request, persists user message to SQLite
   ↓
4. Agent loop starts:
    a. Retrieve relevant memories (semantic search)
    b. Compact context when the transcript exceeds the configured budget
    c. Build prompt with system prompt + transcript + memories + skill index
    d. Call comet-sdk StreamMessage with provider/model
    e. Stream SSE events back to Cometline:
       - turn_status (retrieval, compaction, model contact, tools)
       - reasoning_delta (thinking tokens)
       - text_delta (visible response)
       - tool_call (model requests a tool)
       - tool_result (tool execution complete)
       - step_finish (one model step done)
    f. If tool_call: execute tool, persist result, goto (a)
    g. If done: persist assistant message, emit done event
   ↓
5. Cometline renders streaming events in ChatView.svelte
   ↓
6. CometMind extracts memories from the turn for future retrieval
```

### Agent delegation (coding-harness CLI)

```
1. User asks CometMind to delegate a coding task
   ↓
2. CometMind calls delegate_coding_task tool
   ↓
3. Tool spawns the selected external harness using its fixed non-interactive CLI profile
   ↓
4. External agent streams JSON/JSONL progress back through stdout
   ↓
5. CometMind emits subagent_progress SSE events to Cometline
   ↓
6. Cometline renders progress in the subagent chat components
   ↓
7. External agent completes, returns result
   ↓
8. CometMind emits subagent_finished, continues agent loop
```

### MCP tool integration

```
1. User configures MCP servers in Cometline Settings → CometMind → MCP
   ↓
2. Settings saved to ~/.cometmind/cometline-settings.json (cometmind.mcp)
   ↓
3. Save or runtime reload → runtime.New()/Runtime.Reload starts or refreshes mcp.Manager
   ↓
4. Manager connects enabled servers (stdio / HTTP / SSE) via go-sdk
   ↓
5. tools.Registry merges MCP tools as provider-safe `mcp_{serverId}_{toolName}` tools
   ↓
6. Agent loop calls MCP tools through the same tool_call / tool_result SSE path
```

OAuth tokens for remote MCP servers live in `~/.cometmind/mcp-oauth/{serverId}.json`, with the registered client identity in `{serverId}.client.json`. CometMind drives the full OAuth flow itself (metadata discovery + dynamic client registration + Authorization Code/PKCE), owning the loopback callback listener and opening the system browser; tokens are auto-refreshed headlessly at connect time.

Management endpoints (`/api/v1/mcp/*`) expose connection status, tool previews, connection tests, reconnection runs, and interactive OAuth flows without editing settings.

### Jobs and scheduled jobs

```
1. User or agent creates a job through /api/v1/jobs
   ↓
2. Job is persisted with status todo / ongoing / done / blocked
   ↓
3. A session or autonomous worker claims the job lease
   ↓
4. Worker heartbeats progress, records job events, and either completes, releases, or blocks the job
   ↓
5. Cometline polls jobs for board updates and optional desktop notifications
```

Scheduled jobs are stored separately under `/api/v1/scheduled-jobs`. The scheduler materializes due one-shot or cron schedules into regular jobs so downstream execution uses the same lease/completion path.

### Discord gateway

```
1. User sends message in Discord channel
   ↓
2. Discord webhook delivers to cometmind gateway
   ↓
3. Gateway maps Discord thread → CometMind session
   ↓
4. Agent loop runs (same as above)
   ↓
5. Gateway streams response back to Discord channel
```

## Key Contracts

### HTTP API (CometMind)

CometMind serves a REST/SSE API on `127.0.0.1:7700`. The OpenAPI spec is `cometmind/openapi.yaml`.

**Endpoint families** (the OpenAPI file is the complete contract):
- **Runtime and models:** health, enabled models, and catalog lookups
- **Workspaces:** registration/pruning; previewable file read/write; Git status, diff, stage, unstage, discard, and commit; global wiki files and backlinks
- **Sessions:** create/list/update/delete, workspace changes and forks, transcripts, one active run, child sessions, and live-session media at `/sessions/{id}/media/{mediaId}`
- **Media:** gallery list/import/delete at `/media`; blob bytes at `/media/{id}/content` work after the owning session is deleted until detached-media retention expires
- **Streaming:** `POST /sessions/{id}/messages` returns the turn SSE stream; `GET /events` carries runtime-wide notifications
- **Skills and MCP:** managed skills, drafts, sync runs; MCP server/tool inspection, connection tests, reconnection, and OAuth flows
- **Memory and storage:** memory CRUD/search/settings, re-embedding, purge and compaction runs; retention and backup runs
- **Jobs and inbox:** durable jobs, leases, retries, scheduled jobs, inbox messages, replies, and dismissals

Use `cometmind/openapi.yaml` rather than this overview when implementing a client or changing a route.

**Client:** `cometline/src/lib/client/cometmind.ts`

### SSE Events

CometMind emits JSON SSE frames with a `type` discriminator:

| Event | Description |
|-------|-------------|
| `reasoning_start` | Reasoning block begins |
| `reasoning_delta` | Reasoning token chunk |
| `text_delta` | Assistant visible text chunk |
| `tool_call` | Model requested a tool |
| `tool_result` | Tool execution completed |
| `step_finish` | One model step ended |
| `subagent_started`, `subagent_progress`, `subagent_finished` | Coding-harness or general-subagent lifecycle |
| `memory_injected`, `memory_updated`, `memory_compaction_completed` | Retrieved, changed, or compacted memory state |
| `context_budget` | Context-window budget telemetry for the active turn |
| `inbox_message_created`, `inbox_message_archived` | Runtime-wide inbox lifecycle notification |
| `assistant_image` | Persisted assistant media; chat fetches the session-media URL while the session lives |
| `turn_status`, `turn_recover` | Pre-output activity status or resumed/recovered turn state |
| `error` | Runtime/model/tool error |
| `done` | Terminal stream event |

**Source:** `cometmind/internal/event/event.go`  
**Renderer types:** `cometline/src/lib/types.ts`  
**Reducer:** `cometline/src/lib/reducers/chat.ts`

### Electron IPC

Electron main process exposes native capabilities to the renderer via preload:

- `getProviderSettings` / `saveProviderSettings` — settings persistence
- `fetchProviderModels` — query provider APIs
- `setWorkspacePath` / `listRecentWorkspaces` — workspace management
- `setOpenAtLogin` — macOS login item
- `checkForUpdates` — auto-update
- MCP OAuth is owned by CometMind (`POST /api/v1/mcp/servers/{id}/oauth-flows`), not Electron IPC
- `notifyJob` — desktop notifications for job state changes
- `onWorkspaceChanged` / `watchWorkspace` — debounced external workspace-change refreshes
- screen-capture access and bridge methods — permission state, target listing, and local captures for runtime tools
- `openMiniWindow` / mini route support — compact chat window lifecycle

**Composition root:** `cometline/electron/src/domains/runtime.ts`

**IPC wiring:** `cometline/electron/src/domains/runtime-ipc.ts` and `ipc.ts`

**Preload:** `cometline/electron/src/preload.ts`

## Extension Points

### Adding a new LLM provider

1. Implement `cometsdk.Provider` interface in `comet-sdk/provider/{name}/`
2. Add provider method constant to `comet-sdk/sdk.go`
3. Add provider config to `cometmind/internal/config/config.go`
4. Add provider UI to `cometline/src/lib/components/settings/SettingsProvidersPanel.svelte`
5. Update `cometline/src/lib/types.ts` with new `ProviderMethod`

### Adding a new tool

1. Implement `Tool` (`Spec`, `Execute`) in a focused file under `cometmind/internal/tools/`
2. Register it for the appropriate capability surface in `registry.go` / `surface.go`
3. Add schema, execution, and workspace-sandbox tests
4. The selected surfaces expose it automatically through the agent loop

### Adding a new SSE event type

1. Extend `StreamEvent` in `cometmind/openapi.yaml`
2. Add event type to `cometmind/internal/event/event.go`
3. Add emitter function in same file
4. Run `make generate` to regenerate clients
5. Add reducer case in `cometline/src/lib/reducers/chat.ts`
6. Update contract tests in `cometmind/internal/contract/`

### Adding a new Agent Skill

1. Create `~/.cometmind/skills/{skill-name}/SKILL.md`
2. Write YAML frontmatter (name, description)
3. Write markdown body with trigger scenarios, workflow, examples
4. Skill is auto-discovered and invokable via `/{skill-name}`

## Persistence

CometMind uses SQLite (pure Go, no CGO) with sqlc for type-safe queries.

**Database:** `~/.cometmind/cometmind.db`  
**Schema:** `cometmind/internal/db/schema.sql`  
**Queries:** `cometmind/internal/db/queries/*.sql`  
**Generated code:** `cometmind/internal/db/*.sql.go` (do not edit)

**Core tables:**
- `workspaces` — registered workspace paths
- `sessions` — workspace-scoped chat sessions
- `messages` — user/assistant/tool messages
- `tool_calls` — tool execution records
- `memories` — semantic memory entries with embeddings

## Configuration

**Runtime settings:** `~/.cometmind/cometline-settings.json`
- Stores provider configuration and the `cometmind` runtime section
- Read directly by CometMind; legacy `config.toml` is used only when JSON is missing
- Most runtime changes apply through `Runtime.Reload`; process bind changes such as host/port restart the sidecar

**Desktop settings:** `~/.cometmind/cometline-desktop.json`
- Stores appearance, shortcuts, app, and persona state
- Electron merges runtime and desktop settings for the Settings UI, then splits them on save

## Further Reading

- [ARCHITECTURE_GUIDE.md](./ARCHITECTURE_GUIDE.md) — detailed contributor map with source references
- [AGENTS.md](./AGENTS.md) — development rules and commands
- [cometmind/openapi.yaml](./cometmind/openapi.yaml) — API contract source of truth
- [cometline/SOUL.md](./cometline/SOUL.md) — default system prompt
