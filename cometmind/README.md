# CometMind

> A local, session-first **general AI agent runtime**. CometMind is the brain — it reasons, plans, remembers, and acts through a pluggable tool layer, and delegates coding work to specialized coding agents (OpenCode, Claude Code, Codex, etc.).

This directory is one module inside the `cometline` monorepo. The historical standalone `cometmind` repo is archived; current development, issues, and pull requests land in the monorepo root.

CometMind is the middle tier of the Cometline stack:

```
comet-sdk   →  provider-agnostic LLM I/O (Anthropic, OpenAI, Codex, xAI, compatible APIs)
cometmind   →  general agent brain: agent loop + tools + memory + persistence + HTTP/SSE + CLI
  └─ Coding harness ──→  OpenCode (CLI), Claude Code (CLI), or Codex (CLI)
cometline   →  Electron desktop shell (also starts CometMind as a sidecar)
```

## What it is

CometMind started as a coding-focused agent and is now a **general-purpose orchestrator**:

- The runtime owns **reasoning, semantic memory, skills, and tool orchestration**.
- **Coding tasks are delegated** to an external coding harness instead of being hardcoded into the runtime.
- The same agent loop powers the **desktop app**, the **CLI**, and the **Discord gateway**.
- Built-in workspace tools cover file I/O, shell commands, web fetch, and skill management.

CometMind still has a clear runtime boundary inside the product: it owns the CLI, config, database, and localhost HTTP/SSE API. But in practice it is now tightly coupled to Cometline's product workflow and monorepo development model. It should be read as an internal first-party runtime, not as a separately evolving project.

## Architecture

```
main.go              entry point → cmd.Execute()
cmd/                 Cobra commands (init, serve, chat, session, skills, gateway, settings, process, model)
server/
  server.go          Gin engine; /api/v1 handlers; SSE encoding
  memory_handlers.go memory CRUD, search, compaction
  job_handlers.go    jobs, leases, events, completion, settings
  scheduled_job_handlers.go deferred and recurring job definitions
  mcp_handlers.go    MCP status, tools, reconnect, OAuth start
  run_manager.go     per-session single in-flight run control
internal/
  runtime/           shared composition root (config · DB · services · runner factory)
  agent/runner.go    core agent loop (multi-step tool iteration, max 50 steps)
  agent/request.go   builds cometsdk.Request from session history + memory + skills
  agent/contextwindow.go context budget and transcript compaction helpers
  session/           domain types + Service (workspaces, sessions, messages, delegation)
  memory/            semantic memory (embed, retrieve, extract, compact)
  jobs/              durable jobs, leases, events, lifecycle settings
  scheduler/         one-shot and cron scheduled job materialization
  autonomy/          autonomous job worker loop
  tools/             ToolSpec + Workspace + registry + built-in implementations
  tools/sandbox/     pathcheck — prevents path escape out of the workspace
  skills/            Agent Skills discovery, sync, export, write
  acp/               Coding-harness CLI runner for delegate_coding_task (OpenCode by default)
  mcp/               stdio/http/sse MCP manager plus OAuth login/token refresh
  gateway/           messaging adapters (Discord today)
  provider/          builds a comet-sdk provider from config/session (Anthropic, OpenAI-compatible, Codex, xAI)
  config/            cometline-settings.json loading + legacy TOML migration + COMETMIND_* env
  db/                sqlc-generated querier + schema.sql + queries/*.sql
  event/event.go     CometMind-native event union (shared by SSE/CLI/gateway)
  store/open.go      opens SQLite (pure-Go modernc.org/sqlite)
openapi.yaml         OpenAPI 3.1 spec for the local serve API
```

### Agent loop

The runner iterates up to `max_steps` work rounds (default 100), then makes one
tool-free request for a best-effort final answer if the work budget was exhausted:

1. **Retrieve memories** (when enabled) → inject into system prompt → emit `memory_injected` and `turn_status`.
2. Compact the context window when needed, then rebuild the conversation from SQLite → call the provider via `llm.StreamMessage`.
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
| `glob` | Find files by glob pattern (`**` supported); gitignore-aware, capped at 100 |
| `grep` | Search file contents (ripgrep when available); gitignore-aware |
| `run_command` | Shell in workspace cwd (120s timeout, denylist for dangerous commands) |
| `web_fetch` | HTTP(S) fetch with HTML→text; SSRF protection |
| `web_search` | Public web search via DuckDuckGo, with Google bridge and protected web-fetch fallbacks |
| `present_image` | Show a local image file inline in chat (and Discord when the gateway is active) |
| `capture_screenshot` | Capture a screen or app window via Cometline’s Electron bridge and present it inline |
| `list_capture_targets` | List screens and open windows for `capture_screenshot` |
| `load_skill` | Load full `SKILL.md` for a discovered skill |
| `read_skill_file` | Read auxiliary files inside a skill directory |
| `write_skill` | Create or update a skill under `~/.cometmind/skills/{name}/` |
| `list_skill_drafts` | List pending skill drafts |
| `write_skill_draft` | Create or update a draft skill for review before promotion |
| `read_skill_draft` | Read a draft skill |
| `promote_skill_draft` | Promote a draft into the managed skills directory |
| `delegate_coding_task` | Spawn a coding-harness child session (sync or async) |
| `spawn_general_agent` / `wait_subagents` | Launch and wait for general subagents |
| `list_jobs`, `create_job`, `claim_job`, `update_job`, `complete_job`, `release_job`, `propose_job` | Interact with durable immediate jobs |
| `list_scheduled_jobs`, `create_scheduled_job`, `update_scheduled_job`, `delete_scheduled_job` | Create/list/edit/delete deferred or recurring schedules (`cron_expr` or `run_at`/`run_at_iso`); do not put schedules in a normal job's DoD |
| `recall_task_outcome` | Retrieve remembered task outcomes |

File tools are workspace-scoped through `internal/tools/sandbox/pathcheck.go`.

### Coding-harness delegation

When the model calls `delegate_coding_task`, CometMind spawns the selected external coding harness and streams progress back through the same SSE pipeline. OpenCode, Claude Code, and Codex all run through their non-interactive CLI modes; CometMind normalizes each harness's JSON/JSONL output into the same subagent events.

The runtime owns the invocation profiles:

- OpenCode: `opencode run --format json --auto`
- Claude Code: `claude -p --output-format stream-json --verbose --dangerously-skip-permissions`
- Codex: `codex exec --json --dangerously-bypass-approvals-and-sandbox`

- Child sessions are persisted with `parent_session_id` and delegation status. The legacy ACP session-ID field remains in the wire model for compatibility, but CLI delegations do not populate it.
- Harness-specific progress is normalized into the same subagent events.
- Harnesses inherit the user's local authentication and configuration. Only enable unattended delegation for trusted workspaces.

Settings expose one choice—OpenCode, Claude Code, or Codex—in Settings → CometMind → Coding task delegation. Settings are persisted in `~/.cometmind/cometline-settings.json` (legacy `config.toml` is read only when JSON settings are missing):

```toml
[acp]
default_harness = "opencode" # opencode, claude, or codex
```

The old command, argument, and timeout fields are ignored during settings migration. Users cannot replace the executable or alter the built-in arguments; they only select the harness. If the selected harness is not installed, CometMind leaves `delegate_coding_task` out of the main agent's tool list until the harness is installed or the selection changes.

### Semantic memory

CometMind stores durable facts, preferences, and project notes in SQLite with embedding-based retrieval:

- **Auto-retrieve** before each turn (top-k by cosine similarity).
- **Auto-extract** after each turn via a structured LLM JSON pass.
- **Compaction** decays stale memories, forgets low-weight entries, and merges clusters when over `max_memories`.
- Embedding uses an OpenAI-compatible endpoint (default model: `text-embedding-3-small`).

Memory is configured under `[memory]` in config and exposed through REST (see API below). Cometline renders injected memories in the chat UI and provides a full memory settings panel.

### MCP client

CometMind can connect external MCP servers and merge their tools into the main agent's registry. CometMind's MCP tools are not injected into coding-harness child processes.

Supported transports:

- `stdio` subprocess servers
- streamable `http` servers
- legacy `sse` servers

Remote OAuth servers are handled by CometMind itself: Protected Resource Metadata discovery, Authorization Server Metadata discovery, Dynamic Client Registration, Authorization Code + PKCE, a loopback callback listener, token persistence, and headless refresh. Access/refresh tokens live in `~/.cometmind/mcp-oauth/{serverId}.json`; registered client metadata lives in `{serverId}.client.json`.

Each server has its own handshake/list-tools budget (stdio 15s/10s, HTTP/SSE 45s/20s). Auth success is not session success: `POST /api/v1/mcp/servers/{id}/oauth-flows` returns 200 once the token is stored (`ok: true, connected: false, error_code, error_hint` if the handshake failed). `oauth_connected` is token-file only. Changing the server URL treats a saved grant as stale. Tool calls go through `Manager.CallTool` so a reconnect is visible on the next turn.

### Jobs, scheduler, and autonomy

Jobs are durable work items with status `todo`, `ongoing`, `done`, or `blocked`. They can be created by users, agents, Discord, or scheduled job materialization.

- `internal/jobs` owns job CRUD, leases, events, retry/blocking, archive, purge, and settings.
- `internal/scheduler` owns scheduled jobs with either `run_at` or `cron_expr`, materializing due schedules into normal jobs.
- `internal/autonomy` can claim ready jobs and run them through a bounded agent session when enabled.
- `internal/gateway` can propose jobs from Discord and notify channels about job progress.
- Cometline renders the `/jobs` board and polls for optional desktop notifications.

### Storage & retention

Session, memory, deleted-job, inbox, and runtime-file cleanup runs once when CometMind starts and then on the configured retention interval. Both `serve` and standalone Discord gateway processes start this maintenance loop. Configure it under `cometmind.storage` in `cometline-settings.json` (or legacy `[storage]` in `config.toml` / `COMETMIND_STORAGE_*` env overrides):

| Field | Default | Meaning |
|---|---|---|
| `retentionDays` | 90 | Delete sessions with no activity for N days; `0` disables |
| `maxSessionsPerWorkspace` | 0 | Keep only the M most recently updated sessions per workspace; `0` disables |
| `archivedMemoryPurgeDays` | 90 | Hard-delete archived memories older than N days; `0` disables |
| `vacuumAfterPurge` | `true` | Run SQLite `VACUUM` after deletions to reclaim disk |

Deleting a session also removes its Discord channel mapping (`gateway_sessions`); the next message in that channel starts a fresh session. Gallery media is not deleted with the session, transcript clear, or an empty workspace — remove it from the Gallery page.

### Agent Skills

CometMind discovers skills from standard install locations and injects a compact index into the system prompt:

- `~/.cometmind/skills`
- `~/.agents/skills`
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

Discord configuration is managed through Cometline Settings → CometMind → Discord or the shared `cometmind.gateway.discord` JSON settings subtree.

## Local serve API

Localhost-only HTTP + SSE, versioned under `/api/v1` (default `http://127.0.0.1:7700`). See `openapi.yaml` for the full spec.

### Health & workspaces

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/health` | Liveness (`{status:ok}`) |
| `GET /api/v1/workspaces` | List registered workspaces |
| `POST /api/v1/workspaces` | Register a workspace by absolute path |
| `POST /api/v1/workspaces/prune-runs` | Remove registrations whose directories no longer exist |
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
| `POST /api/v1/sessions/{id}/forks` | Copy a session into another workspace |
| `DELETE /api/v1/sessions/{id}` | Delete session and cascade messages; gallery media stays |
| `GET /api/v1/sessions/{id}/messages` | Transcript (user/reasoning/assistant/tool) |
| `POST /api/v1/sessions/{id}/messages` | Send text + up to 6 images (4 MiB each) → SSE |
| `DELETE /api/v1/sessions/{id}/messages` | Clear transcript; gallery media stays |
| `GET /api/v1/sessions/{id}/media/{mediaId}` | Fetch assistant media while the session exists |
| `GET /api/v1/sessions/{id}/children` | Delegated child sessions |
| `DELETE /api/v1/sessions/{id}/runs/current` | Abort in-flight run (202, or 409 if none) |

### Media

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/media` | List ready gallery items |
| `GET /api/v1/media/{id}/content` | Fetch bytes after the session is gone |
| `POST /api/v1/media/{id}/imports` | Copy an item into another session |
| `DELETE /api/v1/media/{id}` | Tombstone the catalog row and delete the file |

### MCP

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/mcp/servers` | List configured MCP server status |
| `GET /api/v1/mcp/tools` | Preview registered MCP tools |
| `POST /api/v1/mcp/servers/{id}/connection-tests` | Test one server connection |
| `POST /api/v1/mcp/servers/{id}/reconnection-runs` | Reconnect one server |
| `POST /api/v1/mcp/servers/{id}/oauth-flows` | Start interactive OAuth login |

### Skills

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/skills` | List discovered skills |
| `POST /api/v1/skills/sync-runs` | Symlink discovered skills into `~/.cometmind/skills` |
| `DELETE /api/v1/skills/{name}` | Delete a managed skill |
| `GET /api/v1/skills/{name}/archive` | Download skill as zip |
| `GET /api/v1/skill-drafts` | List draft skills |
| `GET /api/v1/skill-drafts/{name}` | Read one draft skill |
| `PUT /api/v1/skill-drafts/{name}` | Update one draft skill |
| `POST /api/v1/skill-drafts/{name}/promote` | Promote a draft into managed skills |
| `DELETE /api/v1/skill-drafts/{name}` | Reject/delete a draft |

### Memory

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/memories` | List active memories |
| `POST /api/v1/memories` | Create a memory manually |
| `DELETE /api/v1/memories/{id}` | Delete a memory |
| `POST /api/v1/memories/searches` | Semantic search |
| `GET /api/v1/memories/settings` | Read memory configuration |
| `PUT /api/v1/memories/settings` | Update memory configuration |
| `POST /api/v1/memories/purge-runs` | Hard-delete archived memories older than a threshold |
| `POST /api/v1/memories/compaction-runs` | Run compaction |
| `GET /api/v1/memories/compaction-preview` | Preview compaction candidates |

### SSE event names

`text_delta`, `reasoning_start`, `reasoning_delta`, `tool_call`, `tool_result`, `step_finish`, `subagent_started`, `subagent_progress`, `subagent_finished`, `memory_injected`, `memory_updated`, `memory_compaction_completed`, `turn_status`, `turn_recover`, `error`, `done`

Only one run is allowed per session at a time (`409 session_running` on duplicate POST).

### Jobs and scheduled jobs

| Method & Path | Purpose |
|---|---|
| `GET /api/v1/jobs` | List jobs with filters |
| `POST /api/v1/jobs` | Create a job |
| `GET /api/v1/jobs/settings` | Read job runtime settings |
| `PUT /api/v1/jobs/settings` | Update job runtime settings |
| `GET /api/v1/jobs/{id}` | Fetch one job |
| `PATCH /api/v1/jobs/{id}` | Update an editable job |
| `DELETE /api/v1/jobs/{id}` | Soft-delete a job |
| `PUT /api/v1/jobs/{id}/archive` / `DELETE /api/v1/jobs/{id}/archive` | Archive or unarchive a job |
| `POST /api/v1/jobs/{id}/retry-runs` | Unblock a failed/blocked job for retry |
| `GET /api/v1/jobs/{id}/events` | List job event history |
| `PUT /api/v1/jobs/{id}/lease` | Claim a job for a session |
| `PATCH /api/v1/jobs/{id}/lease` | Heartbeat a claimed job |
| `DELETE /api/v1/jobs/{id}/lease` | Release a job claim |
| `PUT /api/v1/jobs/{id}/completion` | Mark a job completed |
| `GET /api/v1/scheduled-jobs` / `POST /api/v1/scheduled-jobs` | List or create scheduled jobs |
| `GET /api/v1/scheduled-jobs/{id}` / `PATCH /api/v1/scheduled-jobs/{id}` / `DELETE /api/v1/scheduled-jobs/{id}` | Read, update, or delete one schedule |

## CLI

| Command | Purpose |
|---|---|
| `cometmind init` | Create config + database; register current workspace |
| `cometmind serve` | Start the HTTP/SSE server (`--port`, `--watch-parent` for Electron sidecar) |
| `cometmind chat "message"` | One agent turn to stdout (`--session`, `--model`, `--provider`) |
| `cometmind session list` | List sessions (`--all`, `--json`, `--workspace-id`; honors `-w`) |
| `cometmind session delete <id>` | Delete a session |
| `cometmind session rename <id> --name <title>` | Rename a session |
| `cometmind session set-model <id> --model <m> --provider <p>` | Switch a session's model |
| `cometmind model list` | List enabled models from settings |
| `cometmind model set <provider> <model>` | Set default model in settings |
| `cometmind skills list\|show\|sync\|delete\|export` | Manage Agent Skills |
| `cometmind gateway run --platform discord` | Start the Discord messaging gateway |
| `cometmind settings reload` | Ask running `serve` and gateway processes to reload settings in place |
| `cometmind process status|stop|restart` | Inspect or control long-lived CometMind processes |

Persistent flag: `--workspace` / `-w` (defaults to current directory).

Session list examples:

```bash
cometmind session list                              # current workspace
cometmind session list -w /path/to/repo             # explicit workspace path
cometmind session list --workspace-id <uuid>        # by workspace id
cometmind session list --all                        # all workspaces (sidebar-equivalent)
cometmind session list --all --json                 # machine-readable output
```

## Configuration

Settings live at `~/.cometmind/cometline-settings.json` (shared with Cometline). The SQLite database is at `~/.cometmind/cometmind.db`.

Set `COMETMIND_DATA_DIR` to relocate the settings file, database, MCP OAuth tokens, and process metadata into a different directory for container or managed-service deployments.

Runtime apply semantics:

- `cometmind settings reload` re-reads the settings file and applies safe in-process changes for new work.
- Memory settings, memory provider swaps, storage cleanup interval changes, job reconcile interval changes, bind host or port changes, Discord token changes, and fresh environment variable values still require restart.
- `cometmind process restart` stops the target process and relaunches it using its recorded command arguments. It waits up to 10 seconds for a clean exit before force-killing, then re-execs the same binary with the same flags.

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
  "defaultProviderId": "my-gateway",
  "defaultModelId": "gpt-4o",
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
max_steps = 100
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

SQLite schema (see `schemaVersion` in `internal/db/migrate.go`) includes:

| Table | Purpose |
|---|---|
| `workspaces` | Registered workspace roots |
| `sessions` | Conversations with model/provider, token usage, delegation fields |
| `messages` | User, assistant, tool result, and system rows (multimodal content) |
| `tool_calls` | Tool-call shells plus execution output and timing |
| `session_media` | Gallery catalog; session/workspace FKs SET NULL on delete |
| `gateway_sessions` | Maps external chat surfaces to CometMind sessions |
| `memories` | Semantic memories with embeddings and lifecycle metadata |
| `memory_events` | Audit log for memory changes |
| `jobs` | Durable work items, leases, retry metadata, archive/delete state |
| `scheduled_jobs` | One-shot and recurring job definitions |
| `job_events` | Audit log for job lifecycle changes |

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

Register this repo as the workspace (`go run . init` from the monorepo root or open it in Cometline), then ask CometMind to improve Cometline. It can call `delegate_coding_task` to hand coding to the selected harness, review test output in the parent session, and iterate.

Example verify command: `cd cometmind && go test ./...`

## License

See repository for license details.
