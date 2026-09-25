# 05 — cometmind Runtime

> **Prerequisite:** [04-comet-sdk.md](./04-comet-sdk.md)  
> **Next:** [06-cometmind-features.md](./06-cometmind-features.md)

## Purpose

`cometmind` is the local agent runtime. A **runtime** is the program that runs the agent. Other parts of the app trust the data it stores.

It owns these parts:

- coordinating reasoning, one step at a time
- saving sessions and jobs
- keeping each workspace separate
- running tools
- memory
- MCP
- the localhost API that the desktop app calls

A **workspace** is one project folder. **MCP** means Model Context Protocol. It connects outside tool servers. An **API** is the HTTP interface other programs call. **Localhost** means the API listens only on this computer.

## Entry points

Each row is one **surface**. A surface is one way to enter the same runtime.

| Surface | Command / file | Role |
|---------|----------------|------|
| HTTP API | `cometmind serve` → `server/server.go` | Main path for Cometline |
| CLI chat | `cometmind chat "message"` | Test from the terminal |
| CLI init | `cometmind init` | Create config, the database, and register a workspace |
| Discord | `cometmind gateway run --platform discord` | Messaging gateway |
| Settings/process | `cometmind settings reload`, `cometmind process ...` | Control a long-running process |
| Models | `cometmind model list/set` | Inspect or update enabled and default models |
| Library | `internal/runtime`, `internal/agent` | Shared by all surfaces |

A **gateway** receives messages from another app, such as Discord. The **CLI** is the set of terminal commands.

All surfaces use the same `agent.Runner` and `session.Service`. There is no second agent implementation.

## Runtime composition

`runtime.New()` in `internal/runtime/runtime.go` builds the runtime. This function is the **composition root**. That is the place where the parts are created and connected.

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

**JSON** and **TOML** are settings file formats. TOML is the older one. **Legacy** means that older format. **SQLite** is the local database. **Pragmas** are SQLite settings applied when the file opens. A **migration** updates a database that already exists. **Retention** is the rule for deleting old data. **Autonomy** means a worker can start jobs with no new user message.

`RunnerFor(session)` builds an `agent.Runner` for one session. It connects a provider, the session service, and a tool registry. The registry is limited to that workspace.

A **provider** is the code that talks to one model service. A **registry** is the list of tools the agent may call.

## Agent Runner

`agent.Runner` in `internal/agent/runner.go` runs each chat turn. A **turn** is one user message plus the work to answer it. A **step** is one model call inside that turn.

### Dependencies (via interfaces)

```go
type TurnStore interface {
    BuildSDKMessages(ctx) ([]cometsdk.Message, error)
    AppendAssistantStep(...)
    AppendToolResults(...)
    // ...
}
```

The runner depends on `TurnStore`, not on SQLite itself. An **interface** is a list of methods. Tests can pass a fake store. That makes the loop easier to test.

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
    req.MaxTokens ← min(this model's output limit, 32,000)

    stream ← llm.StreamMessage(provider, req)
    for event := range stream.Events():
      emit(translate(event))
    result ← stream.Result()

    store.AppendAssistantStep(result)
    store.SaveTokenUsage(result.Usage)

    if no tool calls or stop: break
    if max_tokens: continue a few times, then break

    for each tool call:
      output ← registry.Execute(tool, input)
      store.AppendToolResult(...)
      emit(tool_result)
```

A **token** is a small piece of text that the model counts. `MaxTokens` is the output limit for one step.

GitNexus is the code index for this repository. `context Run -f runner.go` shows outgoing calls to `StreamMessage`, all `event.*` emitters, `BuildRequest`, and `NormalizeHistoryForProvider`.

### Completion and compatibility details

A turn is more than a `done` event. The runtime saves assistant text, tool output, and token usage.

It also keeps provider continuation state. That state is **opaque**. Opaque means CometMind stores it but does not read inside it. The state stays with the provider and model that made it. A different provider or model does not reuse it.

The runtime can save assistant media separately from text. **Media** here means images and similar files.

After the turn, memory extraction runs in the background. This work is **asynchronous**. The chat does not wait for it. It is also **globally bounded**. The runtime limits how many extractions can run at once. A slow extraction does not delay the end of the turn.

If a provider or model rejects a feature, the runtime records that failure. A **compatibility policy** uses those records. The runtime then stops requesting features that this provider or model does not support.

### Related agent packages

| File | Role |
|------|------|
| `request.go` | `BuildRequest` assembles the comet-sdk Request |
| `normalize.go` | Cleans history for the current provider |
| `job_progress_hook.go` | Reports background job progress during runs |
| `contextwindow.go`, `compaction.go`, `budget.go` | Step output limit, context reserve, and compaction |

The **context reserve** is space kept for the reply. The step output limit is explained in [05a-output-limit.md](./05a-output-limit.md). Each step sets `MaxTokens` to `min(this model's output limit, 32,000)`. `cometmind.maxTokens` in the settings file is not that limit.

## Session service and data model

### SQLite schema (`internal/db/schema.sql`)

A **schema** is the list of tables and columns. An **embedding** is a list of numbers that stands for text. Search uses it to find similar memories. An **audit** row records what changed. A **lease** is a time-limited claim that one worker owns a job. **Durable** means the data stays after the process exits. **Semantic** memory stores facts by meaning, not only by exact words.

| Table | Purpose |
|-------|---------|
| `workspaces` | Registered absolute workspace paths |
| `sessions` | Conversations: model, provider, and token usage JSON |
| `messages` | User, assistant, tool_result, and system rows |
| `tool_calls` | Tool-call records plus execution output |
| `memories` | Semantic memory rows with embeddings |
| `memory_events` | Memory audit events |
| `memory_reembed_jobs` | Durable work to migrate or rebuild embeddings |
| `assistant_provider_states` | Opaque continuation state for the provider and model that made it |
| `model_capability_negatives` | Saved exclusions for model features that failed |
| `inbox_messages` | Durable notices for the user, plus reply state |
| `gateway_sessions` | Maps an outside chat thread or channel to a session |
| `jobs` | Durable job queue: status, leases, retry, archive, and delete data |
| `scheduled_jobs` | One-shot and recurring schedule definitions |
| `job_events` | Audit log for job status changes |

**One-shot** means the schedule runs once. **Recurring** means it repeats.

Database path: `~/.cometmind/cometmind.db`

### Migrations

- Tracked with `PRAGMA user_version` and `schemaVersion` in `internal/db/migrate.go`.
- Read `schemaVersion` in `migrate.go` for the current version.
- For existing users, add an incremental `alterStatements` entry. A `schema.sql` edit alone is not enough.
- Never edit generated sqlc files. After a schema or query change, run `sqlc generate`.

**Incremental** means the change updates the old database in small steps. **sqlc** generates Go code from SQL. Do not edit that generated code by hand.

### session.Service responsibilities

| Operation | Method area |
|-----------|-------------|
| Register workspace | `EnsureWorkspace` |
| Create, list, or delete sessions | CRUD methods |
| Append user message | `AppendUserMessageContent` |
| Save assistant step | `AppendAssistantStep` |
| Save tool results | `AppendToolResult` |
| Rebuild SDK history | `buildSDKMessagesFromRows` |
| UI transcript | `LoadTranscript` in `transcript.go` |
| Token usage snapshot | JSON in `sessions.token_usage` |

**CRUD** means create, read, update, and delete. A **transcript** is the chat text shown in the UI. A **snapshot** is the saved totals at one moment.

### Persisted formats

**Persisted** means saved in the database.

| Field | Format |
|-------|--------|
| `messages.reasoning_content` | JSON array of reasoning blocks |
| `messages.content` (tool_result) | JSON `{tool_call_id, content, is_error}` |
| `sessions.token_usage` | JSON `cometsdk.TokenUsage` |
| `sessions.context_summary` | Short summary after context compaction |

## HTTP/SSE server

The HTTP server is a Gin app in `server/server.go`. It is built with `server.New(deps)`. **Gin** is the Go HTTP library. **SSE** means Server-Sent Events. The server pushes events to the client on one open connection.

### Critical handler: POST message

`handlePostMessage` lives in `server/messages.go`. It is registered from `server/server.go`.

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

`server/run_manager.go` allows one in-flight run per session. **In-flight** means the run has started and has not finished. This stops tool results from mixing. It also protects the transcript if the user sends many messages quickly. Mixed results would damage the transcript.

Cancel with `DELETE /api/v1/sessions/{id}/runs/current`. That calls `RunManager.Cancel`.

### CORS

**CORS** is a browser rule. It decides which sites may call the API. CometMind allows Vite dev origins, localhost, `app://`, `file://`, and an empty origin. **Vite** is the frontend dev server. An **origin** is the site address the browser checks. The empty origin is for the packaged app.

## Tools

### Tool surfaces (`internal/tools/surface.go`)

A tool **surface** is a capability policy. It is not a separate registry of chosen tool names. A **capability** is a kind of action the agent may take.

| Surface | Used by | Capabilities |
|---------|---------|--------------|
| `ParentSurface` | Main agent | Full: read, edit, run, skills and drafts, spawn, jobs, memory, MCP, settings. `delegate_coding_task` only if ACP is enabled |
| `ResearchSurface` | In-process general subagent | Read and skills |
| `CodingSurface` | In-process coding subagent | Read, edit, run, and skills. No MCP, spawn, or settings |

**ACP** is the coding-harness feature. A **subagent** is a smaller agent started by the main agent. **In-process** means it runs inside CometMind, not as a separate program. A **harness** is an outside coding program.

### Registry (`internal/tools/registry.go`)

Each workspace root gets a registry from `newRegistryWithSurface`.

| Family | Tools |
|--------|-------|
| FS / shell / web | `read_file`, `edit_file`, `write_file`, `list_dir`, `glob`, `grep`, `run_command`, `web_fetch`, `web_search` |
| Skills | `load_skill`, `read_skill_file`, `write_skill`, draft tools (`write_skill_draft`, …) |
| Subagents / harness | `spawn_general_agent`, `wait_subagents`; `delegate_coding_task` when harness enabled |
| MCP | `list_mcp_servers`, `reconnect_mcp_server`, plus `mcp_{serverId}_{toolName}` |
| Jobs | `list_jobs`, `propose_job`, `create_job`, `claim_job`, `update_job`, `complete_job`, `release_job`, scheduled-job tools |
| Memory | `recall_task_outcome`, `list_memories`, `search_memories`, `create_memory`, `update_memory`, `delete_memory` |
| Settings | `list_settings`, `get_settings`, `patch_settings` (parent only; reject desktop keys) |

**FS** means the file system. The **shell** runs terminal commands.

### Sandbox

A **sandbox** limits where tools may act. `internal/tools/sandbox/pathcheck.go` blocks a path escape. A **path escape** is a file path that leaves the workspace root. Every file tool uses this check.

### Tool interface

```go
type Tool interface {
    Spec() ToolSpec   // name, description, JSON schema
    Execute(ctx, input) (output, error)
}
```

Register new tools in `registry.go`, in `init()` or `NewRegistry`.

## Config and provider factory

### Config loading (`internal/config/config.go`)

1. Read `~/.cometmind/cometline-settings.json` first.
2. If that JSON file is missing, read `~/.cometmind/config.toml`.
3. Then apply `COMETMIND_*` environment variables over the file values.

An **environment variable** is a value set outside the program. These variables replace matching values from the file.

### Provider factory (`internal/provider/factory.go`)

`NewForModel(providerID, modelID)` does this:

- Find the provider entry in settings.
- For key-based methods, find the API key. The order is settings, then the environment, then provider-specific variables.
- Connect the `codex` and `xai` subscription or session providers.
- Build the concrete `cometsdk.Provider`.

**Concrete** means the real provider type, not only an interface.

For `opencode-go`, the factory chooses the wire protocol from models.dev metadata. A **wire protocol** is the exact HTTP message format. **Metadata** here is the model record from models.dev.

- `@ai-sdk/openai` uses OpenAI Responses. That provider is `openairesponses`.
- `@ai-sdk/anthropic` uses Anthropic Messages.
- The default uses Chat Completions. This includes the offline catalog.

`NewFor` uses the entry's primary model. `NewMemoryLLM` uses the extraction model. Both call `NewForModel`.

The shared Responses wire protocol lives in `comet-sdk/internal/responsesproto`. Codex and OpenCode Go both use it.

## Event layer

`internal/event/event.go` defines CometMind event types and their JSON format. This set is an **event union**. One JSON object is one of those event types. The runner translates comet-sdk events into these events. Then the server writes SSE frames. An SSE **frame** is one event message on that connection.

Some events exist only in the runtime. They have no matching SDK event:

- `turn_status`, `turn_recover`
- `memory_injected`, `memory_updated`, `memory_compaction_completed`
- `subagent_started`, `subagent_progress`, `subagent_finished`

This is a second translation layer. The split is intentional. The OpenAPI contract can differ a little from SDK internals. **OpenAPI** is the written description of the HTTP API.

## CLI commands

| Command | Use |
|---------|-----|
| `go run . init --workspace /path` | Create config and the database |
| `go run . serve --port 7700` | Start the API. This is what Electron starts. |
| `go run . chat "hello"` | Quick test in the terminal |
| `go run . session list` | List sessions |
| `go run . gateway run --platform discord` | Start the Discord bot |
| `go run . settings reload` | Ask running processes to reload safe settings in place |
| `go run . process status\|stop\|restart` | Inspect or control long-running CometMind processes |
| `go run . model list\|set` | Inspect or change model defaults in settings |

**In place** means the running process reloads settings. It does not exit and start again.

## Testing

```bash
cd cometmind
go test ./...                    # All tests
go test -run TestPostMessage ./server  # Specific handler test
```

Server tests use `httptest` and a temporary SQLite database.

## Invariants checklist

An **invariant** is a rule that must stay true.

Before you change CometMind, check these:

- [ ] Sessions stay workspace-scoped. Each session belongs to one workspace.
- [ ] One run per session is enforced
- [ ] `done` is always sent
- [ ] Tool calls are saved before their results
- [ ] `turn_status` and `done` keep UI progress and the end of the turn consistent
- [ ] The workspace sandbox still blocks path escape
- [ ] Schema changes have migrations
- [ ] OpenAPI is updated if the API changes
- [ ] Job changes update job events, leases, settings, and retention together

## What's next

[06-cometmind-features.md](./06-cometmind-features.md) covers memory, MCP, coding-harness delegation, Discord, skills, and background jobs. Those features are built on this runtime.
