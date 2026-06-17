# Cometline

**A desktop AI assistant that brings coding agents to your workspace.**

Cometline combines a beautiful chat interface with a powerful agent runtime, letting you interact with LLMs while delegating complex coding tasks to specialized agents like OpenCode. Run it locally, connect it to Discord, and let AI help you build.

## Why Cometline?

- **Chat UI that feels native** — SvelteKit + Electron desktop app with streaming responses, syntax highlighting, and smooth animations
- **Agent delegation** — Hand off coding tasks to OpenCode or Claude Code via ACP (Agent Communication Protocol), with progress streamed back to your chat
- **Discord integration** — Run the same agent runtime as a Discord bot, with per-thread sessions and @mention gating
- **Local-first** — SQLite persistence, workspace-scoped sessions, and semantic memory that learns from your projects
- **Multi-provider** — Switch between Anthropic, OpenAI, and any OpenAI-compatible API

## Quick Start

### Prerequisites

- macOS 13+ (Apple Silicon or Intel)
- [Node.js](https://nodejs.org/) 22+
- [Go](https://go.dev/) 1.25+
- [pnpm](https://pnpm.io/) 11+

### Install

```bash
# Clone with submodules
git clone --recurse-submodules https://github.com/cometline/cometline-release.git
cd cometline-release

# Install dependencies
make install

# Build and launch
make dev
```

The app will open and prompt you to configure a provider. Add your Anthropic or OpenAI API key in Settings → Providers.

### Download Pre-built App

For macOS users, download the latest signed release from [GitHub Releases](https://github.com/cometline/cometline-release/releases). The app is notarized and includes auto-update support.

## Features

### Chat Interface

- **Streaming responses** with visible reasoning blocks and tool call activity
- **Multimodal input** — paste or drop images (PNG, JPEG, GIF, WebP) and text files
- **Rich markdown** — syntax highlighting, math (KaTeX), tables, and embedded link previews
- **Workspace-scoped sessions** — each project gets its own chat history
- **Semantic memory** — the agent remembers context across sessions with automatic retrieval

### Agent Delegation

Cometline can delegate coding tasks to external agents via ACP:

```
You: Help me refactor the auth module
CometMind: I'll delegate this to OpenCode...
[OpenCode agent spawns, streams progress back]
OpenCode: I've refactored auth.go to use middleware...
```

Configure in `~/.cometmind/config.toml`:

```toml
[acp]
command = "opencode"
args = ["acp"]
timeout = "30m"
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

CometMind includes tools for file operations, command execution, and web fetching:

- `read_file`, `write_file`, `list_dir` — workspace-scoped file access
- `run_command` — execute shell commands in the workspace
- `web_fetch` — retrieve and parse web content
- `load_skill`, `write_skill` — manage Agent Skills

### Agent Skills

Skills are reusable prompt templates stored in `~/.cometmind/skills/` or your workspace's `.agents/skills/` directory. Invoke them with slash commands:

```
/create-skill Build a skill for reviewing PRs
/tdd Help me implement the user auth feature
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  cometline    Electron + SvelteKit desktop shell        │
│               Chat UI, settings, animations             │
├─────────────────────────────────────────────────────────┤
│  cometmind    Go agent runtime                          │
│               Agent loop, tools, memory, ACP,           │
│               Discord gateway, HTTP/SSE API             │
├─────────────────────────────────────────────────────────┤
│  comet-sdk    Go LLM I/O library                        │
│               Anthropic + OpenAI + compatible APIs      │
└─────────────────────────────────────────────────────────┘
```

- **cometline** — Desktop renderer that talks to CometMind over HTTP/SSE
- **cometmind** — Local agent runtime with SQLite persistence, serves the API on `127.0.0.1:7700`
- **comet-sdk** — Provider-agnostic streaming LLM library with retry logic and tool-call assembly

See [ARCHITECTURE.md](./ARCHITECTURE.md) for detailed system design.

## Development

```bash
# Run all checks (typecheck, tests, build)
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

See [AGENTS.md](./AGENTS.md) for development rules and commands.

## Configuration

CometMind stores config in `~/.cometmind/config.toml`:

```toml
provider = "anthropic"
model = "claude-sonnet-4-5"
max_tokens = 8192
max_steps = 50

[acp]
command = "opencode"
args = ["acp"]
timeout = "30m"

[gateway.discord]
enabled = false
bot_token_env = "DISCORD_BOT_TOKEN"
require_mention = true
```

Desktop settings are stored in `~/.cometmind/cometline-settings.json` and managed via the Settings UI.

## License

MIT

## Links

- [Documentation](./ARCHITECTURE.md)
- [Contributing](./AGENTS.md)
- [Issues](https://github.com/cometline/cometline-release/issues)
