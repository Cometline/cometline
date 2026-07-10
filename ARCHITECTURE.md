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
│  - Agent loop (up to 50 steps)                                  │
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
│  - Streaming LLM I/O (Anthropic, OpenAI, compatible APIs)       │
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
- Session navigation and workspace switching
- Multimodal input (images, file drops)
- Native desktop features (tray icon, keyboard shortcuts, auto-update)
- Jobs board, skill draft editor, settings route, mini-window routes, and workspace file preview/editor

**Must not own:**
- Tool execution
- Provider request construction
- Database writes
- Agent loop logic

**Key files:**
- `src/lib/components/Composer.svelte` — message input with model picker
- `src/lib/components/ChatView.svelte` — conversation rendering
- `src/lib/stores/` — reactive state (chat, session, model, settings)
- `src/lib/client/cometmind.ts` — HTTP/SSE client for CometMind API
- `electron/main.cjs` — sidecar lifecycle, settings persistence, IPC

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
- `internal/server/server.go` — HTTP/SSE API registration
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

**Core endpoints:**
- `GET /api/v1/health` — liveness check
- `POST /api/v1/sessions` — create workspace-scoped session
- `GET /api/v1/sessions?workspace_path=...` — list sessions
- `POST /api/v1/sessions/{id}/messages` — send message, receive SSE stream
- `DELETE /api/v1/sessions/{id}/runs/current` — cancel in-flight run
- `GET /api/v1/workspaces/files` — list previewable workspace files
- `GET /api/v1/workspaces/files/content` / `PUT /api/v1/workspaces/files/content` — read/write small workspace files
- `GET /api/v1/skill-drafts` and related draft endpoints — list, edit, promote, or reject skill drafts
- `GET /api/v1/mcp/servers` — MCP server connection status
- `GET /api/v1/mcp/tools` — registered MCP tools preview
- `POST /api/v1/mcp/servers/{id}/connection-tests` — test MCP connection
- `POST /api/v1/mcp/servers/{id}/reconnection-runs` — reconnect one MCP server
- `POST /api/v1/mcp/servers/{id}/oauth-flows` — start interactive MCP OAuth login
- `GET /api/v1/jobs` and `/api/v1/jobs/{id}` — jobs board and job detail APIs
- `PUT /api/v1/jobs/{id}/lease`, `PATCH /api/v1/jobs/{id}/lease`, `DELETE /api/v1/jobs/{id}/lease`, `PUT /api/v1/jobs/{id}/completion` — job worker lifecycle
- `GET /api/v1/scheduled-jobs` and `/api/v1/scheduled-jobs/{id}` — scheduled job management

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
| `subagent_progress` | Coding-harness or general-subagent progress update |
| `subagent_finished` | Coding-harness or general-subagent finished |
| `memory_injected` | Retrieved memories were injected into the turn |
| `memory_updated` | Post-turn memory extraction changed memory state |
| `turn_status` | Pre-output activity status such as retrieving memories or contacting the model |
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
- `getMcpOAuthStatus` / `startMcpOAuth` — native browser/loopback MCP OAuth flow
- `notifyJob` — desktop notifications for job state changes
- `openMiniWindow` / mini route support — compact chat window lifecycle

**Source:** `cometline/electron/main.cjs`  
**Preload:** `cometline/electron/preload.cjs`

## Extension Points

### Adding a new LLM provider

1. Implement `cometsdk.Provider` interface in `comet-sdk/provider/{name}/`
2. Add provider method constant to `comet-sdk/sdk.go`
3. Add provider config to `cometmind/internal/config/config.go`
4. Add provider UI to `cometline/src/lib/components/SettingsPanel.svelte`
5. Update `cometline/src/lib/types.ts` with new `ProviderMethod`

### Adding a new tool

1. Define tool in `cometmind/internal/tools/registry.go`
2. Implement `Tool` interface (Name, Description, Execute)
3. Register in `init()` function
4. Tool is automatically available to the agent loop

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

**CometMind config:** `~/.cometmind/config.toml`
```toml
provider = "anthropic"
model = "claude-sonnet-4-5"
max_tokens = 8192
max_steps = 50

[acp]
default_harness = "opencode" # opencode, claude, or codex

[gateway.discord]
enabled = false
bot_token_env = "DISCORD_BOT_TOKEN"
```

**Desktop settings:** `~/.cometmind/cometline-settings.json`
- Managed via Settings UI in Cometline
- Stores provider configs, active provider, appearance, shortcuts
- Synced to CometMind on save (triggers sidecar restart if needed)

## Further Reading

- [ARCHITECTURE_GUIDE.md](./ARCHITECTURE_GUIDE.md) — detailed contributor map with source references
- [AGENTS.md](./AGENTS.md) — development rules and commands
- [cometmind/openapi.yaml](./cometmind/openapi.yaml) — API contract source of truth
- [cometline/SOUL.md](./cometline/SOUL.md) — default system prompt
