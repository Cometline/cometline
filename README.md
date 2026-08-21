# Cometline

![Minako](./static/preview.png)

**An AI companion for your workspace.**

Cometline is a local-first AI companion with a native desktop chat UI and a powerful agent runtime behind it. It remembers what matters, keeps each project isolated, lets you switch personas, maintains a persistent knowledge wiki, leaves you inbox notes from background work, delegates coding to specialized agents, and runs the same brain as a Discord bot — all on your machine.

## Personas

Pick the companion personality that fits your workflow in Settings → About. Switching personas updates the chat avatar, app icon, and the SOUL system prompt CometMind uses.

| Persona | Avatar | Description |
| --- | :---: | --- |
| **Minako** (default) | <img src="./static/minako.png" width="96" alt="Minako" /> | Warm, cute AI companion |
| **Souma** | <img src="./static/souma.png" width="96" alt="Souma" /> | Warm, humorous AI companion |

## Why Cometline?

- **Persona switch** — Choose between companion personas (e.g. Minako or Souma) in Settings; each persona has its own avatar, tone, and SOUL system prompt
- **Semantic memory** — Automatically retrieves and learns context across sessions; compact, search, and re-embed when you change embedding models
- **LLM wiki** — Persistent Karpathy-style knowledge wiki at `~/.cometmind/wiki/` (`@runtime/wiki`), with backlinks, browse/edit in the workspace panel, and the built-in `/llm-wiki` skill
- **Inbox** — Agents leave short notes via `leave_inbox_message` (bell icon); reply later to internalize as memory, or dismiss
- **Jobs and scheduling** — Track work on a Kanban-style jobs board, claim/complete leases, and materialize one-shot or cron schedules (optional autonomous worker)
- **In-process coding + harness delegation** — Native `edit_file` / `run_command` tools with diffs in chat, spawn parallel subagents, or hand off to OpenCode / Claude Code / Codex
- **Session terminal** — Per-session terminal panel; select output and send it into chat as context
- **Workspace panel** — Browse the web, workspace file tree, and wiki side-by-side; capture page/file/terminal snippets as composer context chips (`@` mentions)
- **Local Ollama** — Optional private chat and embedding models via a user-installed Ollama daemon (setup wizard, health checks, model pull)
- **Workspace isolation** — Separate chat history, sessions, tools, and memories per project; file access stays sandboxed to the active workspace
- **Agent Skills** — Reusable prompt templates via slash commands, drafts before promotion, and overlap detection when synthesizing new skills
- **Discord bot** — Same agent runtime as a Discord bot with per-thread sessions, @mention gating, and skill invocation
- **Native chat UI** — SvelteKit + Electron with streaming, reasoning blocks, mini-window + session drawer, keyboard shortcuts, and auto-update
- **Multi-provider** — Anthropic, OpenAI, OpenAI-compatible, Ollama, OpenCode Go, ChatGPT Codex, and xAI Grok (subscription auth); OpenCode Go models auto-route to Chat Completions, Anthropic Messages, or OpenAI Responses per model
- **MCP client** — External Model Context Protocol servers over stdio, streamable HTTP, or SSE, including OAuth-protected remote servers
- **Backup and retention** — Zip backups of `~/.cometmind`, plus retention for aged tool-output / agent-tmp files

## Quick Start

### Prerequisites

- macOS 13+ (Apple Silicon or Intel)

### Install

Install with Homebrew after adding the trusted tap:

```bash
brew tap cometline/tap
brew install --cask cometline
```

Or in one command:

```bash
brew install --cask cometline/tap/cometline
```

Or download the latest signed release from [GitHub Releases](https://github.com/cometline/cometline/releases). The app is notarized and includes auto-update support.

The app will open and prompt you to configure a provider. Add your API key (or connect Ollama), enable models, and choose default model roles in Settings → Providers. For a local no-API-key path, see [Running with Ollama](./cometline/docs/ollama-local.md).

## Features

### Semantic Memory

Cometline builds a persistent memory layer for your companion:

- **Auto-retrieve** — relevant memories are injected before each turn
- **Auto-extract** — new facts and preferences are captured after conversations
- **Compaction** — merge/prune stale entries manually from Settings → Memory, or automatically on extract
- **Re-embed** — when you change the embedding model, a cancellable background job rebuilds vectors; retrieval keeps the previous index until the new one finishes
- **Workspace-scoped** — memories stay tied to the project they belong to
- **Agent tools** — `list_memories`, `search_memories`, `create_memory`, `update_memory`, `delete_memory`, `recall_task_outcome`
- **Manageable** — browse, edit, search, and compact memories in Settings

### LLM Wiki

A compounding personal knowledge base separate from atomic memory facts:

- **Persistent mount** — `@runtime/wiki/` maps to `~/.cometmind/wiki/` (not age-purged)
- **Built-in skill** — `/llm-wiki` for ingest, query, research, and lint (Karpathy LLM Wiki pattern)
- **Browse in UI** — list, preview, edit, and follow `[[wikilinks]]` / backlinks in the workspace panel and composer `@` search
- **Agent file tools** — read/write wiki pages through the same workspace-scoped file APIs via the runtime mount

### Inbox

Background work can leave notes without interrupting chat:

- **Bell inbox** — open messages from the chrome; badge shows unread count
- **Agent tool** — `leave_inbox_message` for scheduled/autonomy results worth a later glance
- **Reply or dismiss** — replies are internalized as memory by a background worker; dismiss archives the note
- **Job deep links** — optional `job_id` ties a note back to the jobs board

### Jobs And Scheduling

Cometline includes a lightweight work queue for tasks that outlive a single chat turn:

- **Jobs board** — create, edit, archive, unblock, and inspect job event history in the desktop UI
- **Runtime leases** — CometMind can claim a job for a session, heartbeat while work is active, release it, or mark it complete
- **Scheduled jobs** — define one-shot `run_at` jobs or recurring `cron_expr` schedules that materialize into the normal jobs queue
- **Agent CRUD** — tools for jobs and scheduled jobs (`create_job`, `list_scheduled_jobs`, …)
- **Autonomy** — optional worker that claims ready jobs and runs bounded agent sessions
- **Notifications** — optional desktop notifications and sidebar indicators for claimed, completed, released, or blocked jobs

### Workspace Isolation

Every project is a first-class workspace with its own boundary:

- **Separate sessions** — chat history does not leak across projects
- **Scoped tools** — file and shell tools operate only inside the active workspace (plus explicit runtime mounts like `@runtime/wiki`)
- **Per-workspace skills** — discover skills from `~/.cometmind/skills/`, `~/.agents/skills/`, `{workspace}/.agents/skills/`, `{workspace}/.claude/skills/`, OpenCode, and Claude Code skill roots

### Chat Interface

- **Streaming responses** with visible reasoning blocks, tool call activity, and edit diffs
- **Multimodal input** — paste or drop images (PNG, JPEG, GIF, WebP) and text files
- **Rich markdown** — syntax highlighting, math (KaTeX), tables, and embedded link previews
- **Context chips** — attach web page, file, directory, wiki, or terminal selections before send
- **Mini window** — compact `/mini` routes with a session drawer for quick access across workspaces
- **Session terminal** — integrated terminal per session; selection becomes chat context
- **Workspace panel** — browser + workspace/wiki file tree with preview snippets and history
- **Navigate history** — forward/back through panel and chat navigation
- **Keyboard shortcuts** — remappable bindings with live tooltips on chrome buttons

### Local Models (Ollama)

Run private chat and memory embeddings through a user-installed [Ollama](https://ollama.com) daemon — Cometline does not bundle or silently install it:

- Settings → Providers → **Ollama Local** (also available in the setup wizard)
- Health check against `http://127.0.0.1:11434`, catalog cards, and in-app model pull
- Assign roles independently (chat/title vs memory embeddings); no silent fallback to cloud
- Changing the embedding model triggers a cancellable re-embed job

See [Running with Ollama](./cometline/docs/ollama-local.md).

### Coding Agents

CometMind can code in-process or delegate to an external harness:

**Native (default)** — `edit_file`, `write_file`, windowed `read_file`, `glob`, `grep`, and `run_command` with timeout/output spill. Edits stream as pretty diffs in chat. Spawn parallel helpers with `spawn_general_agent` / `wait_subagents`.

**External harness (opt-in)** — Settings → CometMind → Coding task delegation enables `delegate_coding_task` for OpenCode, Claude Code, or Codex through fixed non-interactive CLI profiles. Command paths and arguments are built into CometMind and are not user-editable.

```
You: Help me refactor the auth module
CometMind: I'll handle this with native edit tools...
[edit_file diffs stream in chat]
```

### Discord Bot

Run CometMind as a Discord bot with the same agent runtime:

```bash
# Set your bot token
export DISCORD_BOT_TOKEN=your_token_here

# Start the gateway
cometmind gateway run --platform discord
```

Features:
- Per-thread sessions with persistent memory
- @mention gating (bot only responds when mentioned)
- Allowlisted users and channels
- Slash commands and skill invocation

### Built-in Tools

CometMind registers tools by surface (parent agent vs coding subagent). Highlights:

| Area | Tools |
| --- | --- |
| Files / shell | `read_file`, `edit_file`, `write_file`, `list_dir`, `glob`, `grep`, `run_command` |
| Web | `web_fetch`, `web_search` |
| Skills | `load_skill`, `read_skill_file`, `write_skill`, draft list/read/write/promote |
| Memory | `list_memories`, `search_memories`, `create_memory`, `update_memory`, `delete_memory`, `recall_task_outcome` |
| Jobs | `list_jobs`, `create_job`, `claim_job`, `update_job`, `complete_job`, `release_job`, `propose_job`, scheduled-job CRUD |
| Agents | `spawn_general_agent`, `wait_subagents`, optional `delegate_coding_task` |
| Inbox | `leave_inbox_message`, `get_job` |
| Settings / MCP | `list_settings`, `get_settings`, `patch_settings`, `list_mcp_servers`, `reconnect_mcp_server` |

Plus any tools exposed by connected MCP servers (`mcp_{serverId}_{toolName}`).

### Agent Skills

Skills are reusable prompt templates — built-in slash commands plus custom skills in `~/.cometmind/skills/`, global `~/.agents/skills/`, workspace-local `.agents/skills/` / `.claude/skills/`, and optional OpenCode or Claude Code skill roots:

```
/create-skill Build a skill for reviewing PRs
/llm-wiki ingest https://example.com/notes
/tdd Help me implement the user auth feature
/my-skill Run my custom workflow
```

Built-in composer commands include `/change`, `/create-skill`, `/model`, `/job`, `/list-jobs`, and `/clear`.

Skills can be drafted before promotion. Drafts live in `~/.cometmind/skill-drafts/` and are managed through CometMind's skill draft APIs plus the Cometline `/skills` route (Drafts tab). When synthesizing a new skill, CometMind detects overlap against managed live skills and drafts (similar name/description) so you can refine or reuse instead of duplicating.

### MCP Tools

MCP servers are configured in Settings → CometMind → MCP and persisted under `cometmind.mcp` in `~/.cometmind/cometline-settings.json`.

- Supported transports: `stdio`, streamable `http`, and legacy `sse`
- Tools are exposed to the main agent as provider-safe names like `mcp_{serverId}_{toolName}`
- Keepalive + auto-reconnect for flaky servers; Settings UI can import Cursor-style `mcp.json`
- Remote OAuth servers use CometMind-managed PRM/authorization-server discovery, Dynamic Client Registration, Authorization Code + PKCE, and headless refresh
- OAuth token files live under `~/.cometmind/mcp-oauth/`, not inside the settings JSON

### Backup And Retention

Settings → CometMind → Storage:

- **Zip backup** of the entire `~/.cometmind` data directory (wiki, database, settings, skills, …)
- **Retention** for aged `tool-output` and `agent-tmp` runtime files
- Session retention controls for old conversations
- Sidecar logs under `~/.cometmind/logs/`

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  cometline    Electron + SvelteKit desktop shell        │
│               Chat UI, settings, workspace panel, terminal,   │
│               mini window, inbox, jobs, animations      │
├─────────────────────────────────────────────────────────┤
│  cometmind    Go agent runtime                          │
│               Agent loop, tools, memory, wiki, inbox,   │
│               coding CLI / subagents, MCP, jobs,        │
│               scheduler, Discord gateway, HTTP/SSE API  │
├─────────────────────────────────────────────────────────┤
│  comet-sdk    Go LLM I/O library                        │
│               Anthropic + OpenAI + Codex + xAI + Ollama │
│               + compatible APIs                         │
└─────────────────────────────────────────────────────────┘
```

- **cometline** — Desktop renderer that talks to CometMind over HTTP/SSE
- **cometmind** — Local agent runtime with SQLite persistence, serves the API on `127.0.0.1:7700`
- **comet-sdk** — Provider-agnostic streaming LLM library with retry logic, tool-call assembly, and Anthropic/OpenAI/Codex/xAI adapters

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed system design, or [docs/learning/](./docs/learning/) for a newcomer path.

## Development

```bash
# Clone the single monorepo
git clone https://github.com/cometline/cometline.git
cd cometline

# Install frontend dependencies
make install

# Regenerate OpenAPI clients after API changes
make generate

# Run all checks (codegen freshness, Go tests, Svelte checks)
make check

# Run frontend tests
cd cometline && pnpm test

# Run backend tests
cd cometmind && go test ./...

# Build for production
make build

# Package macOS app
make package
```

All new development happens in this repository. The historical `comet-sdk`, `cometmind`, and `cometline` repos are archived for reference only.

See [AGENTS.md](./AGENTS.md) for development rules and commands.

## Contributing

- Fork `cometline`
- Clone your fork normally; no submodule bootstrap is required
- Run `make install` and `make dev` from the repository root
- Run `make check` before opening a PR
- Open a single PR here, even for changes that span `cometline/`, `cometmind/`, and `comet-sdk/`

## Configuration

Cometline and CometMind share settings in `~/.cometmind/cometline-settings.json`. Desktop UI state (appearance, shortcuts, persona) lives in `~/.cometmind/cometline-desktop.json`. CometMind still reads legacy `~/.cometmind/config.toml` only when the JSON settings file is missing.

```json
{
  "providers": [
    {
      "id": "openai",
      "name": "OpenAI",
      "method": "openai",
      "enabled": true,
      "baseURL": "https://api.openai.com/v1",
      "apiKey": "...",
      "selectedModel": "gpt-4o",
      "models": ["gpt-4o"],
      "enabledModels": ["gpt-4o"]
    }
  ],
  "defaultProviderId": "openai",
  "defaultModelId": "gpt-4o",
  "cometmind": {
    "maxTokens": 2048,
    "acp": { "enabled": false, "default_harness": "opencode" },
    "storage": {
      "retentionDays": 90,
      "backup": { "enabled": false }
    }
  }
}
```

Manage this file through the Settings UI unless you are intentionally hand-editing local configuration. Default model roles (chat, title, memory) are set under Settings → Providers → Model roles.

## License

Apache License 2.0. See [LICENSE](./LICENSE).

## Links

- [Documentation](./ARCHITECTURE.md)
- [Learning path](./docs/learning/00-README.md)
- [Ollama local setup](./cometline/docs/ollama-local.md)
- [Contributing](#contributing)
- [Issues](https://github.com/cometline/cometline/issues)
