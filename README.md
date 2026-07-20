# Cometline

![Minako](./static/preview.png)

**An AI companion for your workspace.**

Cometline is a local-first AI companion with a native desktop chat UI and a powerful agent runtime behind it. It remembers what matters, keeps each project isolated, lets you switch personas, delegates coding work to specialized agents, and runs the same brain as a Discord bot — all on your machine.

## Personas

Pick the companion personality that fits your workflow in Settings → About. Switching personas updates the chat avatar, app icon, and the SOUL system prompt CometMind uses.

| Persona | Avatar | Description |
| --- | :---: | --- |
| **Minako** (default) | <img src="./static/minako.png" width="96" alt="Minako" /> | Warm, cute AI companion |
| **Souma** | <img src="./static/souma.png" width="96" alt="Souma" /> | Warm, humorous AI companion |

## Why Cometline?

- **Persona switch** — Choose between companion personas (e.g. Minako or Souma) in Settings; each persona has its own avatar, tone, and SOUL system prompt
- **Semantic memory** — Automatically retrieves and learns context across sessions so your companion remembers preferences, decisions, and project details
- **Jobs and scheduling** — Track work on a Kanban-style jobs board, let the runtime claim/complete jobs, and materialize scheduled jobs from one-shot or cron definitions
- **Coding agent delegation** — Hand off complex tasks to OpenCode, Claude Code, or Codex through fixed non-interactive CLI profiles, with progress streamed back to your chat
- **Workspace isolation** — Separate chat history, sessions, tools, and memories per project; file access stays sandboxed to the active workspace
- **Agent Skills** — Reusable prompt templates invoked with slash commands (`/tdd`, `/create-skill`, or custom skills in your workspace)
- **Discord bot** — Run the same agent runtime as a Discord bot with per-thread sessions, @mention gating, and skill invocation
- **Native chat UI** — SvelteKit + Electron desktop app with streaming responses, reasoning blocks, syntax highlighting, and smooth animations; includes a mini-window for quick access
- **Multi-provider** — Switch between Anthropic, OpenAI, OpenAI-compatible APIs, OpenCode Go, ChatGPT Codex, and xAI Grok (subscription auth)
- **MCP client support** — Connect to external Model Context Protocol servers over stdio, streamable HTTP, or SSE, including OAuth-protected remote servers

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

The app will open and prompt you to configure a provider. Add your API key, enable models, and choose default model roles in Settings → Providers. For a local no-API-key path, see [Running with Ollama](./cometline/docs/ollama-local.md).

## Features

### Semantic Memory

Cometline builds a persistent memory layer for your companion:

- **Auto-retrieve** — relevant memories are injected before each turn
- **Auto-extract** — new facts and preferences are captured after conversations
- **Workspace-scoped** — memories stay tied to the project they belong to
- **Manageable** — browse, edit, search, and compact memories in Settings

### Jobs And Scheduling

Cometline includes a lightweight work queue for tasks that outlive a single chat turn:

- **Jobs board** — create, edit, archive, unblock, and inspect job event history in the desktop UI
- **Runtime leases** — CometMind can claim a job for a session, heartbeat while work is active, release it, or mark it complete
- **Scheduled jobs** — define one-shot `run_at` jobs or recurring `cron_expr` schedules that materialize into the normal jobs queue
- **Notifications** — optional desktop notifications for claimed, completed, released, or blocked jobs

### Workspace Isolation

Every project is a first-class workspace with its own boundary:

- **Separate sessions** — chat history does not leak across projects
- **Scoped tools** — `read_file`, `write_file`, `list_dir`, and `run_command` operate only inside the active workspace
- **Per-workspace skills** — discover skills from `~/.cometmind/skills/`, `~/.agents/skills/`, `{workspace}/.agents/skills/`, `{workspace}/.claude/skills/`, OpenCode, and Claude Code skill roots

### Chat Interface

- **Streaming responses** with visible reasoning blocks and tool call activity
- **Multimodal input** — paste or drop images (PNG, JPEG, GIF, WebP) and text files
- **Rich markdown** — syntax highlighting, math (KaTeX), tables, and embedded link previews
- **Mini mode** — compact `/mini` routes for quick session access outside the full shell
- **File panel** — preview and edit small workspace files through CometMind's workspace-scoped file APIs

### Agent Delegation

Cometline can delegate coding tasks to external coding harnesses:

```
You: Help me refactor the auth module
CometMind: I'll delegate this to the selected coding harness...
[Coding harness spawns, streams progress back]
OpenCode: I've refactored auth.go to use middleware...
```

Choose OpenCode, Claude Code, or Codex in Settings → CometMind → Coding task delegation. The settings are persisted in `~/.cometmind/cometline-settings.json`; command paths and arguments are built into CometMind and are not user-editable.

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

CometMind includes tools for file operations, command execution, and web fetching:

- `read_file`, `write_file`, `list_dir` — workspace-scoped file access
- `run_command` — execute shell commands in the workspace
- `web_fetch` — retrieve and parse web content
- `load_skill`, `read_skill_file`, `write_skill` — manage Agent Skills

### Agent Skills

Skills are reusable prompt templates — built-in slash commands plus custom skills in `~/.cometmind/skills/`, global `~/.agents/skills/`, workspace-local `.agents/skills/` / `.claude/skills/`, and optional OpenCode or Claude Code skill roots:

```
/create-skill Build a skill for reviewing PRs
/tdd Help me implement the user auth feature
/my-skill Run my custom workflow
```

Skills can also be drafted before promotion. Drafts live in `~/.cometmind/skill-drafts/` and are managed through CometMind's skill draft APIs plus the Cometline `/skill-drafts` route.

### MCP Tools

MCP servers are configured in Settings → CometMind → MCP and persisted under `cometmind.mcp` in `~/.cometmind/cometline-settings.json`.

- Supported transports: `stdio`, streamable `http`, and legacy `sse`
- Tools are exposed to the main agent as provider-safe names like `mcp_{serverId}_{toolName}`
- Remote OAuth servers use CometMind-managed PRM/authorization-server discovery, Dynamic Client Registration, Authorization Code + PKCE, and headless refresh
- OAuth token files live under `~/.cometmind/mcp-oauth/`, not inside the settings JSON

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  cometline    Electron + SvelteKit desktop shell        │
│               Chat UI, settings, animations             │
├─────────────────────────────────────────────────────────┤
│  cometmind    Go agent runtime                          │
│               Agent loop, tools, memory, coding CLI,   │
│               MCP, jobs, scheduler, Discord gateway,    │
│               HTTP/SSE API                              │
├─────────────────────────────────────────────────────────┤
│  comet-sdk    Go LLM I/O library                        │
│               Anthropic + OpenAI + Codex + xAI + compat │
│               APIs                                      │
└─────────────────────────────────────────────────────────┘
```

- **cometline** — Desktop renderer that talks to CometMind over HTTP/SSE
- **cometmind** — Local agent runtime with SQLite persistence, serves the API on `127.0.0.1:7700`
- **comet-sdk** — Provider-agnostic streaming LLM library with retry logic, tool-call assembly, and Anthropic/OpenAI/Codex/xAI adapters

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed system design.

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

Cometline and CometMind share settings in `~/.cometmind/cometline-settings.json`. CometMind still reads legacy `~/.cometmind/config.toml` only when the JSON settings file is missing.

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
    "acp": { "command": "opencode", "args": ["acp"], "timeout": "30m" }
  }
}
```

Manage this file through the Settings UI unless you are intentionally hand-editing local configuration.

## License

Apache License 2.0. See [LICENSE](./LICENSE).

## Links

- [Documentation](./ARCHITECTURE.md)
- [Contributing](#contributing)
- [Issues](https://github.com/cometline/cometline/issues)
