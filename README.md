# CometMind

> A local, session-first **general AI agent runtime**. CometMind is the brain — it reasons, plans, and acts through a pluggable tool layer, and delegates coding work to specialized coding agents (e.g. opencode) over **ACP**.

CometMind is the middle tier of the Cometline stack:

```
comet-sdk   →  provider-agnostic LLM I/O layer (streaming, tool-call assembly, retries)
cometmind   →  general agent brain: agent loop + tool registry + persistence + local HTTP/SSE server + TUI
  └─ ACP ──→  opencode / claude-code (coding specialist, invoked only for coding tasks)
cometline   →  Electron desktop shell
```

## What changed

CometMind started life as `cometcode`, a coding-specific agent. It has been **repositioned as a general-purpose agent orchestrator**. The core agent loop was always general; what changes is the framing and the tool surface:

- The runtime owns **reasoning, planning, memory, and orchestration**.
- **Coding tasks are delegated** to an external coding agent (opencode) via the Agent Communication Protocol (ACP), instead of being hardcoded into the runtime.
- The built-in tools (`read_file`, `write_file`, `list_dir`, `run_command`) remain available for lightweight local operations, and the tool registry is designed to grow toward general capabilities (web, memory, messaging, etc.).

## Architecture

```
main.go              entry point → cmd.Execute()
cmd/                 Cobra commands: root / serve / chat / tui / init / session
server/
  server.go          Gin engine; /api/v1 handlers; SSE encoding
  run_manager.go     per-session single in-flight run control (start/cancel/finish)
internal/
  agent/runner.go    core agent loop (multi-step tool iteration, max 50 steps)
  agent/request.go   builds cometsdk.Request from session history
  session/service.go persistence coordination (workspace/session/message/tool_call CRUD)
  tools/             read_file / write_file / list_dir / run_command + registry
  tools/sandbox/     pathcheck — prevents path escape out of the workspace
  provider/factory.go builds a comet-sdk provider from config (anthropic/openai)
  config/            config.toml loading + API key resolution
  db/                sqlc-generated querier + schema.sql + queries/*.sql
  event/event.go     CometMind-native event union (shared by SSE/TUI/CLI)
  store/open.go      opens SQLite (pure-Go modernc.org/sqlite — static-compile friendly)
tui/                 Bubble Tea: session list ↔ chat screens
openapi.yaml         OpenAPI 3.1 spec for the local serve API
```

### Agent loop

The runner iterates up to `MaxSteps` (default 50):

1. Rebuild the conversation from SQLite → call the provider via `llm.StreamMessage`.
2. Stream SDK events → translate to CometMind `event.Event` and push to the caller.
3. Persist token usage and the assistant step (reasoning + tool-call shells).
4. If there are tool calls: execute each via `Registry.Execute`, record duration/exit, persist the tool result, and emit a `tool_result` event.
5. Stop when `finish_reason` is `stop`/`max_tokens`, or when there are no tool calls.

### Delegating coding tasks via ACP

When a task is identified as coding work, CometMind does not write code itself. It hands the task to an ACP-speaking coding agent (opencode / claude-code), streams the agent's progress back through the same event pipeline, and persists the result. This keeps the general runtime lean while reusing best-in-class coding agents.

## Local serve API

Localhost-only HTTP + SSE surface, versioned under `/api/v1` (default `http://127.0.0.1:7700`). See `openapi.yaml` for the full spec.

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/health` | Liveness (`{status:ok}`) — used by Cometline desktop on startup |
| `POST /api/v1/sessions` | Create a session (`workspace_id` or `workspace_path`) |
| `GET /api/v1/sessions` | List sessions for a workspace |
| `GET /api/v1/sessions/{id}` | Fetch a single session |
| `GET /api/v1/sessions/{id}/messages` | Transcript-style messages (user/reasoning/assistant/tool) |
| `POST /api/v1/sessions/{id}/message` | Send text, returns `text/event-stream` (SSE) |
| `POST /api/v1/sessions/{id}/abort` | Abort an in-flight run (202, or 409 if none running) |

SSE event names: `text_delta`, `reasoning_start`, `reasoning_delta`, `tool_call`, `tool_result`, `step_finish`, `error`, `done`.

## Configuration

Config lives at `~/.cometmind/config.toml`, overridable via `COMETMIND_*` environment variables. The SQLite database is stored at `~/.cometmind/cometmind.db`.

## Build & run

```bash
# Build everything
go build ./...

# Initialize config
go run . init

# Start the local serve API
go run . serve

# Or use the interactive TUI
go run . tui
```

Requires Go 1.25+. `comet-sdk` is consumed via a local `replace => ../comet-sdk` directive in `go.mod`.

## License

See repository for license details.
