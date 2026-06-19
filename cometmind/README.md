# CometMind

> A local, session-first **general AI agent runtime**. CometMind is the brain — it reasons, plans, remembers, and acts through a pluggable tool layer, and delegates coding work to specialized coding agents (OpenCode, Claude Code, etc.) over **ACP**.

This directory is one module inside the `cometline-release` monorepo. The historical standalone `cometmind` repo is archived; current development, issues, and pull requests land in the monorepo root.

CometMind is the middle tier of the Cometline stack:

```
comet-sdk   →  provider-agnostic LLM I/O (Anthropic, OpenAI, Codex, compatible APIs)
cometmind   →  general agent brain: agent loop + tools + memory + persistence + HTTP/SSE + CLI
  └─ ACP ──→  opencode / claude-code (coding specialist, invoked via delegate_coding_task)
cometline   →  Electron desktop shell (also starts CometMind as a sidecar)
```

## What it is

CometMind started as a coding-focused agent and is now a **general-purpose orchestrator**:

- The runtime owns **reasoning, planning, semantic memory, skills, and tool orchestration**.
- **Coding tasks are delegated** to an external ACP-speaking agent instead of being hardcoded into the runtime.
- The same agent loop powers the **desktop app**, the **CLI**, and the **Discord gateway**.
- Built-in workspace tools cover file I/O, shell commands, web fetch, and skill management.

CometMind still has a clear runtime boundary inside the product: it owns the CLI, config, database, and localhost HTTP/SSE API. But in practice it is now tightly coupled to Cometline's product workflow and monorepo development model. It should be read as an internal first-party runtime, not as a separately evolving project.

## Architecture

```
main.go              entry point → cmd.Execute()
cmd/                 Cobra commands (init, serve, chat, session, skills, gateway)
server/
  server.go          Gin engine; /api/v1 handlers; SSE encoding
  memory_handlers.go memory CRUD, search, compaction
  run_manager.go     per-session single in-flight run control
internal/
  runtime/           shared composition root (config · DB · services · runner factory)
  agent/runner.go    core agent loop (multi-step tool iteration, max 50 steps)
  agent/request.go   builds cometsdk.Request from session history + memory + skills
  session/           domain types + Service (workspaces, sessions, messages, delegation)
  memory/            semantic memory (embed, retrieve, extract, compact)
  tools/             ToolSpec + Workspace + registry + built-in implementations
  tools/sandbox/     pathcheck — prevents path escape out of the workspace
  skills/            Agent Skills discovery, sync, export, write
  acp/               ACP client for delegate_coding_task (OpenCode by default)
  gateway/           messaging adapters (Discord today)
  provider/          builds a comet-sdk provider from config/session (Anthropic, OpenAI-compatible, Codex)
  config/            cometline-settings.json loading + legacy TOML migration + COMETMIND_* env
  db/                sqlc-generated querier + schema.sql + queries/*.sql
  event/event.go     CometMind-native event union (shared by SSE/CLI/gateway)
  store/open.go      opens SQLite (pure-Go modernc.org/sqlite)
openapi.yaml         OpenAPI 3.1 spec for the local serve API
```

### Agent loop

The runner iterates up to `max_steps` (default 50):

1. **Retrieve memories** (when enabled) → inject into system prompt → emit `memory_injected`.
2. Rebuild the conversation from SQLite → call the provider via `llm.StreamMessage`.
3. Stream SDK events → translate to CometMind `event.Event` and push to the caller.
4. Persist token usage and the assistant step (reasoning + tool-call shells).
5. If there are tool calls: execute each via `Registry.Execute`, record duration/exit, persist the tool result, and emit `tool_result`.
6. Stop when `finish_reason` is `stop`/`max_tokens`, or when there are no tool calls.
7. **Extract memories** after the turn (when enabled) → emit `memory_updated`.

### Built-in tools

Registered per workspace in `internal/tools/registry.go`:

| Tool | Purpose |
|---|---|
| `read_file` | Read UTF-8 text under the workspace root |
| `write_file` | Create or overwrite a file; mkdir parents |
| `list_dir` | Non-recursive directory listing |
| `run_command` | Shell in workspace cwd (120s timeout, denylist for dangerous commands) |
| `web_fetch` | HTTP(S) fetch with HTML→text; SSRF protection |
| `load_skill` | Load full `SKILL.md` for a discovered skill |
| `read_skill_file` | Read auxiliary files inside a skill directory |
| `write_skill` | Create or update a skill under `~/.cometmind/skills/{name}/` |
| `delegate_coding_task` | Spawn or resume an ACP child session (sync or async) |

File tools are workspace-scoped through `internal/tools/sandbox/pathcheck.go`.

### ACP delegation

When the model calls `delegate_coding_task`, CometMind spawns an external coding agent (default: `opencode acp`) and streams progress back through the same SSE pipeline:

- Child sessions are persisted with `parent_session_id`, delegation status, and ACP session ID.
- Interactive mode can pause for user questions or permission prompts (`subagent_awaiting_input`).
- The desktop can reply to awaiting children via `POST /api/v1/sessions/{id}/respond`.

Configure in Settings → CometMind → ACP, persisted in `~/.cometmind/cometline-settings.json` (legacy `config.toml` is read only when JSON settings are missing):

```toml
[acp]
command = "opencode"
args = ["acp"]
timeout = "30m"
interactive = true
```

### Semantic memory

CometMind stores durable facts, preferences, and project notes in SQLite with embedding-based retrieval:

- **Auto-retrieve** before each turn (top-k by cosine similarity).
- **Auto-extract** after each turn via a structured LLM JSON pass.
- **Compaction** decays stale memories, forgets low-weight entries, and merges clusters when over `max_memories`.
- Embedding uses an OpenAI-compatible endpoint (default model: `text-embedding-3-small`).

Memory is configured under `[memory]` in config and exposed through REST (see API below). Cometline renders injected memories in the chat UI and provides a full memory settings panel.

### Storage & retention

Session and archived-memory cleanup runs once when CometMind starts (sidecar restart after saving General settings in Cometline). Configure under `cometmind.storage` in `cometline-settings.json` (or legacy `[storage]` in `config.toml` / `COMETMIND_STORAGE_*` env overrides):

| Field | Default | Meaning |
|---|---|---|
| `retentionDays` | 90 | Delete sessions with no activity for N days; `0` disables |
| `maxSessionsPerWorkspace` | 0 | Keep only the M most recently updated sessions per workspace; `0` disables |
| `archivedMemoryPurgeDays` | 90 | Hard-delete archived memories older than N days; `0` disables |
| `vacuumAfterPurge` | `true` | Run SQLite `VACUUM` after deletions to reclaim disk |

Deleting a session also removes its Discord channel mapping (`gateway_sessions`); the next message in that channel starts a fresh session.

### Agent Skills

CometMind discovers skills from standard install locations and injects a compact index into the system prompt:

- `~/.cometmind/skills`
- `<workspace>/.agents/skills`
- `<workspace>/.claude/skills`
- `~/.config/opencode/skills`
- `~/.claude/skills`

The model loads full instructions on demand via `load_skill` / `read_skill_file`. Cometline and Discord expose `/create-skill` to author new skills.

```bash
npx skills add vercel-labs/agent-skills -g -a opencode -a claude-code
go run . skills list
go run . skills sync
```

```toml
[skills]
enabled = true
roots = []
include_opencode = true
include_claude = true
mirror_to_cometmind = false
```

### Discord gateway

CometMind can run as a Hermes-style messaging gateway. Discord is the first supported platform.

```bash
go run . gateway run --platform discord
```

Features: allowlisted users/channels, `@mention` gating, per-thread sessions, typing indicators, reply chunking, `/thread` and `/create-skill` slash commands.

See [`docs/GATEWAY.md`](docs/GATEWAY.md) for setup.

## Local serve API

Localhost-only HTTP + SSE, versioned under `/api/v1` (default `http://127.0.0.1:7700`). See `openapi.yaml` for the full spec.

### Health & workspaces

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/health` | Liveness (`{status:ok}`) |
| `GET /api/v1/workspaces` | List registered workspaces |
| `POST /api/v1/workspaces` | Register a workspace by absolute path |
| `POST /api/v1/workspaces/prune` | Remove registrations whose directories no longer exist |
| `GET /api/v1/workspaces/files` | List previewable workspace files |
| `GET /api/v1/workspaces/files/content` | Read a previewable text/image file |
| `PUT /api/v1/workspaces/files/content` | Write a small UTF-8 text file from the preview editor |

### Sessions

| Method & Path | Purpose |
|---|---|
| `POST /api/v1/sessions` | Create a session (`workspace_id` or `workspace_path`) |
| `GET /api/v1/sessions` | List sessions for one workspace |
| `GET /api/v1/sessions/{id}` | Fetch a session |
| `PATCH /api/v1/sessions/{id}` | Update model/provider for later turns |
| `PATCH /api/v1/sessions/{id}/workspace` | Move an existing session to another workspace |
| `POST /api/v1/sessions/{id}/fork` | Copy a session into another workspace |
| `DELETE /api/v1/sessions/{id}` | Delete session and cascade messages |
| `GET /api/v1/sessions/{id}/messages` | Transcript (user/reasoning/assistant/tool) |
| `GET /api/v1/sessions/{id}/children` | Delegated child sessions |
| `POST /api/v1/sessions/{id}/message` | Send text + up to 6 images (4 MiB each) → SSE |
| `POST /api/v1/sessions/{id}/abort` | Abort in-flight run (202, or 409 if none) |
| `POST /api/v1/sessions/{id}/respond` | Reply to an awaiting ACP child → SSE |

### Skills

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/skills` | List discovered skills |
| `GET /api/v1/skills/{name}` | Read one skill's `SKILL.md` |
| `POST /api/v1/skills/sync` | Symlink discovered skills into `~/.cometmind/skills` |
| `DELETE /api/v1/skills/{name}` | Delete a managed skill |
| `GET /api/v1/skills/{name}/export` | Download skill as zip |

### Memory

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/memories` | List active memories |
| `POST /api/v1/memories` | Create a memory manually |
| `PATCH /api/v1/memories/{id}` | Update a memory |
| `DELETE /api/v1/memories/{id}` | Delete a memory |
| `POST /api/v1/memories/search` | Semantic search |
| `GET /api/v1/memory/settings` | Read memory configuration |
| `PUT /api/v1/memory/settings` | Update memory configuration |
| `POST /api/v1/memory/purge` | Hard-delete archived memories older than a threshold |
| `POST /api/v1/memory/compact` | Run compaction |
| `GET /api/v1/memory/compact/preview` | Preview compaction candidates |

### SSE event names

`text_delta`, `reasoning_start`, `reasoning_delta`, `tool_call`, `tool_result`, `step_finish`, `subagent_started`, `subagent_progress`, `subagent_awaiting_input`, `subagent_finished`, `memory_injected`, `memory_updated`, `error`, `done`

Only one run is allowed per session at a time (`409 session_running` on duplicate POST).

## CLI

| Command | Purpose |
|---|---|
| `cometmind init` | Create config + database; register current workspace |
| `cometmind serve` | Start the HTTP/SSE server (`--port`, `--watch-parent` for Electron sidecar) |
| `cometmind chat "message"` | One agent turn to stdout (`--session` to resume) |
| `cometmind session list` | List sessions for the current workspace |
| `cometmind skills list\|show\|sync\|delete\|export` | Manage Agent Skills |
| `cometmind gateway run --platform discord` | Start the Discord messaging gateway |

Persistent flag: `--workspace` / `-w` (defaults to current directory).

## Configuration

Settings live at `~/.cometmind/cometline-settings.json` (shared with Cometline). The SQLite database is at `~/.cometmind/cometmind.db`.

If `cometline-settings.json` is missing but legacy `config.toml` exists, CometMind loads the TOML once and logs a migration hint. New installs get a minimal JSON template from `cometmind init` / first `Load()`.

Environment overrides use the `COMETMIND_` prefix (dots become underscores). Provider API keys fall back to `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `COMETMIND_API_KEY`.

Example JSON shape (Cometline writes the full file from Settings):

```json
{
  "providers": [
    {
      "id": "my-gateway",
      "name": "Company Gateway",
      "method": "openai-compatible",
      "enabled": true,
      "baseURL": "https://gateway.example.com/v1",
      "apiKey": "...",
      "enabledModels": ["gpt-4o"],
      "models": ["gpt-4o"],
      "selectedModel": "gpt-4o"
    }
  ],
  "activeProviderId": "my-gateway",
  "cometmind": {
    "systemPromptPath": "/path/to/SOUL.md",
    "memory": { "embedding": { "providerId": "", "model": "" } },
    "storage": {
      "retentionDays": 90,
      "maxSessionsPerWorkspace": 0,
      "archivedMemoryPurgeDays": 90,
      "vacuumAfterPurge": true
    }
  }
}
```

Legacy `config.toml` (still read for migration):

```toml
provider = "anthropic"
model = "claude-sonnet-4-5"
base_url = ""
max_tokens = 8192
max_steps = 50
system_prompt_path = ""

[[providers]]
id = "my-gateway"
name = "Company Gateway"
method = "openai-compatible"
base_url = "https://gateway.example.com/v1"
api_key = "..."
model = "gpt-4o"

[memory]
enabled = true
auto_extract = true
auto_retrieve = true

[gateway.discord]
enabled = false
bot_token_env = "DISCORD_BOT_TOKEN"
allowed_users = []
allowed_channels = []
require_mention = true
workspace_path = "/path/to/workspace"
```

When Cometline is running, Settings writes `~/.cometmind/cometline-settings.json`; CometMind reads that same file on startup.

## Database

SQLite schema (version 7) includes:

| Table | Purpose |
|---|---|
| `workspaces` | Registered workspace roots |
| `sessions` | Conversations with model/provider, token usage, delegation fields |
| `messages` | User, assistant, tool result, and system rows (multimodal content) |
| `tool_calls` | Tool-call shells plus execution output and timing |
| `gateway_sessions` | Maps external chat surfaces to CometMind sessions |
| `memories` | Semantic memories with embeddings and lifecycle metadata |
| `memory_events` | Audit log for memory changes |

After schema or query changes, run `sqlc generate` and add incremental migrations in `internal/db/migrate.go`.

## Build & run

```bash
# From cometmind/
go build ./...
go test ./...

go run . init
go run . serve --port 7700

# One CLI turn scoped to the current workspace
go run . chat "hello"
```

From the monorepo root:

```bash
make dev      # build CometMind + launch Cometline Electron app
make check    # SDK tests + CometMind tests + Svelte checks
make package  # build sidecar + package Electron app
```

Requires Go 1.25+. `comet-sdk` is consumed via `replace github.com/cometline/comet-sdk => ../comet-sdk`.

CometMind is not versioned or released independently today, and the current documentation should assume monorepo-first development rather than future standalone distribution.

## Closed-loop self-improvement

Register this repo as the workspace (`go run . init` from the monorepo root or open it in Cometline), then ask CometMind to improve Cometline. It can call `delegate_coding_task` to hand coding to OpenCode, review test output in the parent session, and iterate.

Example verify command: `cd cometmind && go test ./...`

## License

See repository for license details.
