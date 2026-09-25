# 01 — The Nutshell

> **Prerequisite:** none  
> **Next:** [02-architecture.md](./02-architecture.md)

## What is Cometline?

Cometline is a **local-first AI companion** for your workspace. Local-first means it runs on your computer. A companion is the assistant you chat with.

You chat in a native desktop app. Native means the app is installed on your computer. A Go agent runtime on your machine does the work. A **runtime** is the program that runs the agent. It handles reasoning, tools, memory, jobs, MCP, and persistence. MCP means Model Context Protocol. It connects external tool servers to the agent. Persistence means saving data so it remains after a restart.

An LLM library talks to Anthropic, OpenAI, Codex, xAI Grok, or compatible APIs. LLM means large language model.

Everything runs on your machine. Chat history, memories, and settings are stored under `~/.cometmind/`. The API binds to `127.0.0.1:7700` only. Binds means it accepts connections on that address and no other.

## The three modules

```
┌─────────────────────────────────────────┐
│  cometline   Desktop UI (Electron +     │
│              SvelteKit)                 │
├─────────────────────────────────────────┤
│  cometmind   Agent runtime (Go)         │
│              loop, tools, memory, jobs, API│
├─────────────────────────────────────────┤
│  comet-sdk   LLM API adapter (Go lib)   │
│              streaming, retries, tools  │
└─────────────────────────────────────────┘
```

**Dependency rule (one direction only):**

```
cometline  →  cometmind  →  comet-sdk
```

A dependency is a module that another module needs. Calls go only in the arrow direction.

- `cometline` never runs tools, and it never calls LLM APIs directly.
- `cometmind` never draws the UI, and it never uses Electron.
- `comet-sdk` never knows about sessions, SQLite, or the chat UI.

## What makes it special

| Feature                 | One-line explanation                                                            |
| ----------------------- | ------------------------------------------------------------------------------- |
| **Workspace isolation** | Each project has its own sessions and memories. File tools are sandboxed. Sandboxed means they cannot leave that project. |
| **Semantic memory**     | Semantic means by meaning, not only by exact words. The app loads related facts before a turn. It saves new facts after a turn. The companion can remember them. |
| **Agent Skills**        | Reusable prompt templates. You start one with `/skill-name`. |
| **ACP coding harness**  | A harness is an external coding program. CometMind sends coding tasks to OpenCode, Claude Code, or Codex. Each one uses a fixed CLI profile. CLI means command-line interface. |
| **MCP client**          | You can connect external MCP servers. Their tools are added to the main agent's tool list. |
| **Jobs and scheduling** | Jobs are stored in a queue, so they survive a restart. A lease is a short lock: only one worker may hold a job. There are also scheduled jobs. Optional workers can start work on their own. |
| **Skill drafts**        | The app can generate a skill as a draft. You can review it and promote it before it becomes active. Promote means you accept the draft. |
| **Discord gateway**     | A gateway connects this runtime to another chat service. The same runtime can run as a Discord bot. Each thread has its own session. |
| **Multi-provider**      | You can switch the model or provider for one session from Settings. |
| **Context and media**   | The app saves file, web, and terminal context references. It shows assistant images in a safe way. |
| **Live workspace panel** | The panel watches file and Git changes made outside the app. It can refresh previews and keep unsaved drafts. |

## One message, end to end

When you type a message and press Enter, this is the path.

A few words used below:

- Persist means save.
- Acquire means take.
- Emit means send.
- Compact means shorten older context so it still fits. That is called compaction.
- A transcript is the saved conversation.
- **SSE** means Server-Sent Events. The server pushes events on one open connection.
- A reducer turns stream events into UI state.

```
You (Composer.svelte)
  │
  ▼ POST /api/v1/sessions/{id}/messages
CometMind server
  │ persist user message to SQLite
  │ acquire single-run lock (one stream per session)
  ▼
Agent Runner (up to 100 work steps + final answer)
  │ retrieve relevant memories and emit turn_status
  │ compact context if needed
  │ rebuild transcript + skills index → comet-sdk Request
  │ step output limit = min(current model output limit, 32,000)
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
ChatView renders live chat rows
```

Each step uses `min(current model output limit, 32,000)`. `min` means the smaller of the two numbers. The current model is the model used on that step.

GitNexus confirms the main call path. `Runner.Run` is in `cometmind/internal/agent/runner.go`. It calls `StreamMessage` in `comet-sdk/llm/stream.go`. That call emits events. CometMind later turns them into `TextDelta`, `ToolCall`, `ToolResult`, and other events. Those types are in `cometmind/internal/event/event.go`.

## Where things live on disk

| Path                                    | Contents                                                       |
| --------------------------------------- | -------------------------------------------------------------- |
| `~/.cometmind/cometline-settings.json`  | Runtime settings: providers and CometMind config. The agent may edit this file. |
| `~/.cometmind/cometline-desktop.json`   | Desktop UI only: appearance, shortcuts, and app/persona |
| `~/.cometmind/cometline-workspace.json` | The selected workspace path |
| `~/.cometmind/cometmind.db`             | Sessions, messages, tool calls, and memories (SQLite) |
| `~/.cometmind/logs/cometline.log`       | Main sidecar log. A sidecar is the helper process the desktop app starts. |
| `~/.cometmind/skills/`                  | Global Agent Skills (`SKILL.md` files) |
| `~/.cometmind/skill-drafts/`            | Draft Agent Skills waiting for review or promotion |
| `~/.cometmind/mcp-oauth/`               | MCP OAuth tokens. OAuth is a sign-in flow for a remote server. These tokens are not in the settings JSON. |
| `~/.cometmind/tool-output/`             | Large tool output saved outside the chat message |
| `{workspace}/.agents/skills/`           | Skills stored in one workspace |

## Mental model: three processes at runtime

**IPC** means inter-process communication. It is how Electron's processes talk.

1. **Electron main** (`cometline/electron/src/main.ts` -> `domains/runtime.ts`). It starts the CometMind sidecar. It sets up the IPC domains. It saves settings.
2. **CometMind** (`cometmind serve`). It serves the HTTP API on port 7700. It runs the agent loop and uses SQLite.
3. **SvelteKit renderer** (`cometline/src/`). A **renderer** is the window process that draws the UI. This renderer is the chat UI. It talks to CometMind with `fetch` and SSE.

The renderer is sandboxed. `contextIsolation` is `true`. `nodeIntegration` is `false`. Sandboxed means page code stays separate from Node.js. Native features reach the page only through `window.electronAPI`. That API comes from the preload script. The preload script runs before the page.

## The one invariant to remember

An **invariant** is a rule that must stay true.

> Provider-specific streaming becomes SDK events. SDK events become CometMind events. CometMind saves those events and sends them over HTTP/SSE. The renderer turns them into live chat rows.

A token is a small piece of text. If any step in that rule breaks, a basic feature stops. That may be live tokens, tool results, or loading the transcript again.

## Key files (bookmark these)

A **contract** is a shared description both sides must follow.

| Layer                 | File                                                  | Role                                                 |
| --------------------- | ----------------------------------------------------- | ---------------------------------------------------- |
| UI input              | `cometline/src/lib/components/composer/Composer.svelte` | Message composer                                   |
| UI render             | `cometline/src/lib/components/ChatView.svelte`        | Coordinates the chat UI for one session              |
| SSE client            | `cometline/src/lib/client/cometmind.ts`               | HTTP and SSE calls to CometMind                      |
| SSE reducer           | `cometline/src/lib/reducers/chat.ts`                  | Turns stream events into UI state                    |
| Electron entrypoint   | `cometline/electron/src/main.ts`                      | Entry file for the Electron main process (ESM)       |
| Electron runtime      | `cometline/electron/src/domains/runtime.ts`           | Sets up the sidecar, settings, windows, and IPC domains |
| Electron IPC contract | `cometline/electron/src/shared/api.ts`                | Typed methods on `window.electronAPI`                |
| API server            | `cometmind/server/server.go`                          | Registers REST and SSE routes                        |
| Agent loop            | `cometmind/internal/agent/runner.go`                  | Multi-step LLM calls and tools                       |
| Jobs                  | `cometmind/internal/jobs/`, `scheduler/`, `autonomy/` | Saved work queue and scheduled work                  |
| Coding harness        | `cometmind/internal/acp/`                             | Fixed CLI profiles for `delegate_coding_task`        |
| MCP                   | `cometmind/internal/mcp/`                             | External MCP tools and OAuth                         |
| LLM stream            | `comet-sdk/llm/stream.go`                             | `StreamMessage`                                      |
| API contract          | `cometmind/openapi.yaml`                              | OpenAPI file that the other API files must match     |

## What's next

[02-architecture.md](./02-architecture.md) explains why these boundaries exist. It also names the contracts the system depends on.
