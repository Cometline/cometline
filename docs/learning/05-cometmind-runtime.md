# 05 — cometmind Runtime

> **Prerequisite:** [04-comet-sdk.md](./04-comet-sdk.md)  
> **Next:** [06-cometmind-features.md](./06-cometmind-features.md)

## Purpose

`cometmind` is the **local agent runtime and source of truth**. It owns reasoning orchestration, session/job persistence, workspace scoping, tool execution, memory, MCP, and the localhost API the desktop app consumes.

## Entry points

| Surface | Command / file | Role |
|---------|----------------|------|
| HTTP API | `cometmind serve` → `server/server.go` | Primary Cometline integration |
| CLI chat | `cometmind chat "message"` | Terminal testing |
| CLI init | `cometmind init` | Create config + DB + register workspace |
| Discord | `cometmind gateway run --platform discord` | Messaging gateway |
| Settings/process | `cometmind settings reload`, `cometmind process ...` | Long-running process control |
| Models | `cometmind model list/set` | Inspect or update enabled/default models |
| Library | `internal/runtime`, `internal/agent` | Shared by all surfaces |

All surfaces use the same `agent.Runner` and `session.Service` — no duplicate agent implementations.

## Runtime composition

`runtime.New()` in `internal/runtime/runtime.go` is the composition root:

```text
runtime.New()
  → config.Load() (JSON settings or legacy TOML)
  → store.OpenSQLite() (SQLite + pragmas + migration)
  → session.New(db)
  → jobs.NewService(db)
  → scheduler.NewService(db)
  → memory.NewService(...) if enabled
  → mcp.NewManager(...) and background connect
  → retention, jobs maintenance, scheduler, autonomy workers where enabled
```

`RunnerFor(session)` wires a session-specific provider, session service, and workspace-scoped tool registry into an `agent.Runner`.

## Agent Runner

`agent.Runner` in `internal/agent/runner.go` is the load-bearing brain.

### Dependencies (via interfaces)

```go
type TurnStore interface {
    BuildSDKMessages(ctx) ([]cometsdk.Message, error)
    AppendAssistantStep(...)
    AppendToolResults(...)
    // ...
}
```

The runner depends on `TurnStore`, not concrete SQLite — making the loop testable.

### Run loop (simplified)

```text
Run(ctx, turn, emit):
  defer emit(done)

  for step < MaxSteps:
    messages ← store.BuildSDKMessages()
    messages ← NormalizeHistoryForProvider(messages)
    emit(turn_status)
    memories ← memory.RetrieveForTurn(...)
    messages/context ← compact if needed
    req ← BuildRequest(system+memories+skills, tools, messages)

    stream ← llm.StreamMessage(provider, req)
    for event := range stream.Events():
      emit(translate(event))
    result ← stream.Result()

    store.AppendAssistantStep(result)
    store.SaveTokenUsage(result.Usage)

    if no tool calls or stop or max_tokens: break

    for each tool call:
      output ← registry.Execute(tool, input)
      store.AppendToolResult(...)
      emit(tool_result)
```

GitNexus `context Run -f runner.go` confirms outgoing calls to `StreamMessage`, all `event.*` emitters, `BuildRequest`, and `NormalizeHistoryForProvider`.

### Completion and compatibility details

A turn is more than a `done` event. The runtime persists assistant/tool output and token usage, keeps opaque provider continuation state only for the matching provider/model scope, and can persist assistant media separately from text. Post-turn memory extraction is asynchronous and globally bounded, so completion is not held hostage by a slow extraction. Provider/model capability failures also feed a compatibility policy, preventing the runtime from repeatedly requesting known-unsupported features.

### Related agent packages

| File | Role |
|------|------|
| `request.go` | `BuildRequest` — assembles comet-sdk Request |
| `normalize.go` | Provider-specific history cleanup |
| `job_progress_hook.go` | Background job progress during runs |
| `contextwindow.go`, `compaction.go`, `budget.go` | Context budget and compaction |

## Session service and data model

### SQLite schema (`internal/db/schema.sql`)

| Table | Purpose |
|-------|---------|
| `workspaces` | Registered absolute workspace paths |
| `sessions` | Conversations: model, provider, token usage JSON |
| `messages` | User, assistant, tool_result, system rows |
| `tool_calls` | Tool-call shells + execution output |
| `memories` | Semantic memory entries with embeddings |
| `memory_events` | Memory audit events |
| `memory_reembed_jobs` | Durable embedding migration/rebuild work |
| `assistant_provider_states` | Opaque continuation state scoped to the provider/model that produced it |
| `model_capability_negatives` | Learned compatibility exclusions for unsupported model capabilities |
| `inbox_messages` | Durable user-facing notifications and reply state |
| `gateway_sessions` | External chat thread/channel to session mappings |
| `jobs` | Durable job queue with status, leases, retry/archive/delete metadata |
| `scheduled_jobs` | One-shot and recurring schedule definitions |
| `job_events` | Audit log for job lifecycle changes |

Database path: `~/.cometmind/cometmind.db`

### Migrations

- Tracked via `PRAGMA user_version` / `schemaVersion` in `internal/db/migrate.go`
- Read `schemaVersion` in `migrate.go` for the current version; schema changes for existing users need an incremental `alterStatements` entry, not only a `schema.sql` edit
- **Never** edit generated sqlc files — run `sqlc generate` after schema/query changes

### session.Service responsibilities

| Operation | Method area |
|-----------|-------------|
| Register workspace | `EnsureWorkspace` |
| Create/list/delete sessions | CRUD methods |
| Append user message | `AppendUserMessageContent` |
| Persist assistant step | `AppendAssistantStep` |
| Persist tool results | `AppendToolResult` |
| Rebuild SDK history | `buildSDKMessagesFromRows` |
| UI transcript | `LoadTranscript` in `transcript.go` |
| Token usage snapshot | JSON in `sessions.token_usage` |

### Persisted formats

| Field | Format |
|-------|--------|
| `messages.reasoning_content` | JSON array of reasoning blocks |
| `messages.content` (tool_result) | JSON `{tool_call_id, content, is_error}` |
| `sessions.token_usage` | JSON `cometsdk.TokenUsage` |
| `sessions.context_summary` | Compacted conversation summary |

## HTTP/SSE server

Gin app in `server/server.go`, built via `server.New(deps)`.

### Critical handler: POST message

Implemented in `server/messages.go` as `handlePostMessage` (registered from `server/server.go`):

```text
handlePostMessage:
  1. Parse + validate JSON body
  2. Load session + workspace
  3. runtime.RunnerFor(session)
  4. runManager.Acquire(sessionID) — one active run
  5. Persist user message (+ auto-title if first)
  6. Set SSE headers (text/event-stream)
  7. goroutine: runner.Run(ctx, turn, writeSSE)
  8. Flush after each event
  9. runManager.Release on completion
```

### RunManager

`server/run_manager.go` enforces **one in-flight run per session**. Prevents interleaved tool results and corrupted transcripts when the user rapid-fires messages.

Cancel via `DELETE /api/v1/sessions/{id}/runs/current` → `RunManager.Cancel`.

### CORS

Allows Vite dev origins, localhost, `app://`, `file://`, and empty origin for packaged app.

## Tools

### Tool surfaces (`internal/tools/surface.go`)

Capability policy, not separate registries of hand-picked names:

| Surface | Used by | Capabilities |
|---------|---------|--------------|
| `ParentSurface` | Main agent | Full: read/edit/run, skills+drafts, spawn, jobs, memory, MCP, settings; `delegate_coding_task` only if ACP enabled |
| `ResearchSurface` | In-process general subagent | Read + skills |
| `CodingSurface` | In-process coding subagent | Read + edit + run + skills (no MCP / spawn / settings) |

### Registry (`internal/tools/registry.go`)

Built per workspace root via `newRegistryWithSurface`:

| Family | Tools |
|--------|-------|
| FS / shell / web | `read_file`, `edit_file`, `write_file`, `list_dir`, `glob`, `grep`, `run_command`, `web_fetch`, `web_search` |
| Skills | `load_skill`, `read_skill_file`, `write_skill`, draft tools (`write_skill_draft`, …) |
| Subagents / harness | `spawn_general_agent`, `wait_subagents`; `delegate_coding_task` when harness enabled |
| MCP | `list_mcp_servers`, `reconnect_mcp_server`, plus `mcp_{serverId}_{toolName}` |
| Jobs | `list_jobs`, `propose_job`, `create_job`, `claim_job`, `update_job`, `complete_job`, `release_job`, scheduled-job tools |
| Memory | `recall_task_outcome`, `list_memories`, `search_memories`, `create_memory`, `update_memory`, `delete_memory` |
| Settings | `list_settings`, `get_settings`, `patch_settings` (parent only; reject desktop keys) |

### Sandbox

`internal/tools/sandbox/pathcheck.go` prevents path escape outside workspace root. Every file tool goes through this check.

### Tool interface

```go
type Tool interface {
    Spec() ToolSpec   // name, description, JSON schema
    Execute(ctx, input) (output, error)
}
```

Register new tools in `registry.go` `init()` or `NewRegistry`.

## Config and provider factory

### Config loading (`internal/config/config.go`)

1. Read `~/.cometmind/cometline-settings.json` (preferred)
2. Fall back to `~/.cometmind/config.toml` if JSON missing
3. Overlay `COMETMIND_*` environment variables

### Provider factory (`internal/provider/factory.go`)

`NewForModel(providerID, modelID)`:
- Resolve provider entry from settings
- Resolve API key (settings → env → provider-specific vars) for key-based methods
- Wire `codex` and `xai` subscription/session providers
- Construct concrete `cometsdk.Provider`
- For `opencode-go`, dispatch by the model's resolved protocol from models.dev metadata: `@ai-sdk/openai` → OpenAI Responses (`openairesponses` provider), `@ai-sdk/anthropic` → Anthropic Messages, default (including offline catalog) → Chat Completions

`NewFor` (entry's primary model) and `NewMemoryLLM` (extraction model) delegate to `NewForModel`. The shared Responses wire protocol lives in `comet-sdk/internal/responsesproto` and is reused by both the Codex and OpenCode Go providers.

## Event layer

`internal/event/event.go` defines the CometMind-native event union and JSON wire format. The runner translates comet-sdk events into these before the server writes SSE frames.

Runtime-only events (no direct SDK equivalent) include:

- `turn_status`, `turn_recover`
- `memory_injected`, `memory_updated`, `memory_compaction_completed`
- `subagent_started`, `subagent_progress`, `subagent_finished`

This is a **second translation layer** — intentional separation so the OpenAPI contract can diverge slightly from SDK internals.

## CLI commands

| Command | Use |
|---------|-----|
| `go run . init --workspace /path` | Bootstrap config + DB |
| `go run . serve --port 7700` | Start API (what Electron spawns) |
| `go run . chat "hello"` | Quick terminal test |
| `go run . session list` | List sessions |
| `go run . gateway run --platform discord` | Start Discord bot |
| `go run . settings reload` | Ask running processes to reload safe settings in place |
| `go run . process status\|stop\|restart` | Inspect/control long-running CometMind processes |
| `go run . model list\|set` | Inspect or change model defaults in settings |

## Testing

```bash
cd cometmind
go test ./...                    # All tests
go test -run TestPostMessage ./server  # Specific handler test
```

Server tests use `httptest` + temporary SQLite databases.

## Invariants checklist

Before changing CometMind, verify:

- [ ] Sessions remain workspace-scoped
- [ ] One run per session enforced
- [ ] `done` always emitted
- [ ] Tool calls persisted before results
- [ ] `turn_status`/`done` events keep UI progress and termination coherent
- [ ] Workspace sandbox intact
- [ ] Schema changes have migrations
- [ ] OpenAPI updated if API changes
- [ ] Jobs changes update job events, leases, settings, and retention behavior together

## What's next

[06-cometmind-features.md](./06-cometmind-features.md) covers memory, MCP, coding-harness delegation, Discord, skills, and background jobs built on this runtime.
