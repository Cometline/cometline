# 01 — The Nutshell

> **Prerequisite:** none  
> **Next:** [02-architecture.md](./02-architecture.md)

## What is Cometline?

Cometline is a **local-first AI companion** for your workspace. You chat in a native desktop app; a Go agent runtime on your machine handles reasoning, tools, memory, jobs, MCP, and persistence; an LLM library talks to Anthropic, OpenAI, Codex, xAI Grok, or compatible APIs.

Everything runs on your machine. Chat history, memories, and settings live under `~/.cometmind/`. The API binds to `127.0.0.1:7700` only.

## The three modules

```
┌─────────────────────────────────────────┐
│  cometline   Desktop UI (Electron +     │
│              SvelteKit)                 │
├─────────────────────────────────────────┤
│  cometmind   Agent brain (Go)           │
│              loop, tools, memory, jobs, API│
├─────────────────────────────────────────┤
│  comet-sdk   LLM wire adapter (Go lib)  │
│              streaming, retries, tools    │
└─────────────────────────────────────────┘
```

**Dependency rule (one direction only):**

```
cometline  →  cometmind  →  comet-sdk
```

- `cometline` never executes tools or calls LLM APIs directly
- `cometmind` never renders UI or touches Electron
- `comet-sdk` never knows about sessions, SQLite, or chat bubbles

## What makes it special

| Feature | One-line explanation |
|---------|---------------------|
| **Workspace isolation** | Each project has its own sessions, memories, and sandboxed file tools |
| **Semantic memory** | Facts are retrieved before turns and extracted after — your companion remembers |
| **Agent Skills** | Reusable prompt templates invoked with `/skill-name` |
| **ACP coding harness** | Hand coding tasks to OpenCode, Claude Code, or Codex via fixed CLI profiles |
| **MCP client** | Connect external MCP servers; their tools join the main agent's toolbox |
| **Jobs and scheduling** | Durable work queue with leases, scheduled jobs, and optional autonomous workers |
| **Skill drafts** | Generated skills can be reviewed and promoted before becoming active |
| **Discord gateway** | Same runtime as a Discord bot with per-thread sessions |
| **Multi-provider** | Switch models/providers per session from Settings |

## One message, end to end

When you type a message and press Enter:

```
You (Composer.svelte)
  │
  ▼ POST /api/v1/sessions/{id}/messages
CometMind server
  │ persist user message to SQLite
  │ acquire single-run lock (one stream per session)
  ▼
Agent Runner (up to 50 steps)
  │ retrieve relevant memories and emit turn_status
  │ compact context if needed
  │ rebuild transcript + skills index → comet-sdk Request
  │ StreamMessage → provider API
  │ translate SDK events → CometMind SSE events
  │ if tool_call → execute tool → persist result → loop
  ▼
SSE stream (turn_status, text_delta, tool_call, tool_result, done, …)
  │
  ▼
cometline chat store + reducer
  │
  ▼
ChatView renders live bubbles
```

GitNexus confirms the load-bearing link: `Runner.Run` in `cometmind/internal/agent/runner.go` calls `StreamMessage` in `comet-sdk/llm/stream.go`, which emits events later translated to `TextDelta`, `ToolCall`, `ToolResult`, etc. in `cometmind/internal/event/event.go`.

## Where things live on disk

| Path | Contents |
|------|----------|
| `~/.cometmind/cometline-settings.json` | Runtime settings: providers, CometMind config (agent-editable) |
| `~/.cometmind/cometline-desktop.json` | Desktop UI only: appearance, shortcuts, app/persona |
| `~/.cometmind/cometline-workspace.json` | Selected workspace path |
| `~/.cometmind/cometmind.db` | Sessions, messages, tool calls, memories (SQLite) |
| `~/.cometmind/logs/cometline.log` | Main sidecar log |
| `~/.cometmind/skills/` | Global Agent Skills (`SKILL.md` files) |
| `~/.cometmind/skill-drafts/` | Draft Agent Skills awaiting review/promotion |
| `~/.cometmind/mcp-oauth/` | MCP OAuth tokens (not in settings JSON) |
| `~/.cometmind/tool-output/` | Spilled large tool output |
| `{workspace}/.agents/skills/` | Workspace-local skills |

## Mental model: three processes at runtime

1. **Electron main** (`cometline/electron/main.cjs`) — spawns CometMind sidecar, handles IPC, persists settings
2. **CometMind** (`cometmind serve`) — HTTP API on port 7700, agent loop, SQLite
3. **SvelteKit renderer** (`cometline/src/`) — chat UI; talks to CometMind via `fetch` + SSE

The renderer is sandboxed (`contextIsolation: true`, `nodeIntegration: false`). Native features reach it only through `window.electronAPI` from the preload script.

## The one invariant to remember

> Provider-specific streaming becomes SDK events, SDK events become CometMind events, CometMind events are persisted and streamed over HTTP/SSE, and the renderer reduces them into live chat rows.

Break any link in that chain and something fundamental stops working — live tokens, tool results, or transcript reload.

## Key files (bookmark these)

| Layer | File | Role |
|-------|------|------|
| UI input | `cometline/src/lib/components/Composer.svelte` | Message composer |
| UI render | `cometline/src/lib/components/ChatView.svelte` | Per-session chat orchestration |
| SSE client | `cometline/src/lib/client/cometmind.ts` | HTTP/SSE to CometMind |
| SSE reducer | `cometline/src/lib/reducers/chat.ts` | Stream events → UI state |
| API server | `cometmind/server/server.go` | REST/SSE registration |
| Agent loop | `cometmind/internal/agent/runner.go` | Multi-step LLM + tools |
| Jobs | `cometmind/internal/jobs/`, `scheduler/`, `autonomy/` | Durable work queue and scheduled work |
| Coding harness | `cometmind/internal/acp/` | Fixed CLI profiles for `delegate_coding_task` |
| MCP | `cometmind/internal/mcp/` | External MCP tools and OAuth |
| LLM stream | `comet-sdk/llm/stream.go` | `StreamMessage` |
| API contract | `cometmind/openapi.yaml` | OpenAPI source of truth |

## What's next

[02-architecture.md](./02-architecture.md) explains *why* these boundaries exist and which contracts are load-bearing.
