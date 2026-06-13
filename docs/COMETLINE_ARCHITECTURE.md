# Cometline Architecture

Purpose: build a personal desktop AI app from the three local repos: `comet-sdk`, `cometmind`, and `cometline`.

## One-Sentence Purpose

Cometline exists so one person can run a local-first personal AI assistant with provider switching, persistent sessions, tool use, memory, and a desktop UI, while keeping the actual agent runtime outside the renderer and under a small, auditable local API.

## Repo Roles

```text
comet-sdk
  Provider-normalized LLM I/O.
  Owns streaming, provider conversion, tool-call delta assembly, retries, and token usage.

cometmind
  Local agent runtime.
  Owns the agent loop, session persistence, tool registry, SQLite database,
  local HTTP/SSE API, CLI, and TUI.

cometline
  Desktop shell.
  Owns Electron lifecycle, spawning the CometMind binary, native OS affordances,
  and Svelte UI that talks to CometMind over REST/SSE.
```

Dependency direction:

```text
cometline UI
  -> CometMind local API (REST + SSE)
    -> cometmind agent/runtime/storage/tools
      -> comet-sdk provider interface
        -> Anthropic / OpenAI / OpenAI-compatible APIs
```

The key rule: Cometline desktop is not the brain. CometMind is the brain. Comet SDK is not the brain either; it is only the model I/O boundary.

## Current Implementation Map

| architecture concept | Current Cometline owner | Current status |
| --- | --- | --- |
| Provider runtime | `comet-sdk` | Strong foundation: normalized provider interface, stream events, tool calls, reasoning events, token usage. |
| Agent runtime | `cometmind/internal/agent` | Present: multi-step tool loop, persistence, SSE events. |
| Local store | `cometmind/internal/db`, `internal/session` | Present: SQLite schema for workspaces, sessions, messages, tool calls. |
| Tool runtime | `cometmind/internal/tools` | Present but early: read/write/list/run command. Missing permission gates. |
| Workspace sandbox | `cometmind/internal/tools/sandbox` | Present but v1: path traversal protection, symlink evaluation intentionally skipped. |
| Local API | `cometmind/server`, `openapi.yaml` | Present for health, sessions, messages, streaming, abort. |
| Desktop runtime | `cometline/docs/HLD.md` | Designed: Electron spawns CometMind, uses REST/SSE for app data, IPC only for OS operations. |
| Renderer app | `cometline/docs/HLD.md` | Designed: Svelte 5 UI, sessions/chat/settings. |
| Memory system | none yet | Major gap. |
| Encrypted secrets | partial | Config/env exists; OS keychain or encrypted secret store missing. |
| Artifacts preview | none yet | Future layer. |
| MCP | none yet | Future layer. |
| Skills / prompt apps / plugins | none yet | Future layer. |
| Chrome Relay | none yet | Future layer. |

## Architecture Principles

- The core product is a local, permissioned orchestration runtime, not a chat UI.
- Provider normalization is load-bearing. Comet SDK owns the provider boundary with `Provider.Stream(ctx, *Request) (<-chan Event, error)`.
- CometMind is the trusted runtime boundary. It owns agent policy, persistence, tool execution, memory, provider calls, and the local API.
- Cometline desktop is a desktop shell and renderer. It owns native app lifecycle, process management, and UI only.
- Tool execution must be auditable. CometMind already stores `tool_calls`, duration, result, and exit code; it still needs explicit permission state.
- Workspace-scoped file access is essential. CometMind routes file tools through a workspace root and path sandbox.
- The UI should consume a stable stream contract. CometMind emits `text_delta`, `reasoning_start`, `reasoning_delta`, `tool_call`, `tool_result`, `step_finish`, `error`, and `done`.
- REST/SSE is the app behavior boundary between Cometline desktop and CometMind. Electron IPC is reserved for OS-native operations only.

## Cometline Design Decisions

| Decision | Project stance |
| --- | --- |
| Runtime language | Go for CometMind and Comet SDK. The runtime is packaged as a static binary. |
| Desktop shell | Electron for process management, packaging, tray/hotkeys, native dialogs, and self-contained distribution. |
| Renderer framework | Svelte 5 in Cometline desktop. |
| Runtime/UI boundary | Cometline desktop calls CometMind through REST/SSE for app behavior. |
| Native OS boundary | Cometline desktop uses Electron IPC only for app version, process status, restart, folder picker, and notifications. |
| Persistence owner | CometMind owns SQLite and schema migrations. |
| Provider owner | Comet SDK normalizes provider APIs; CometMind chooses providers and builds requests. |
| Extension order | Add permissions, secrets, provider/config APIs, and memory before larger extension systems. |

## Current Contracts

### Comet SDK Contract

`comet-sdk` exposes the right low-level abstraction:

```go
type Provider interface {
    ID() string
    Stream(ctx context.Context, req *Request) (<-chan Event, error)
}
```

Important normalized types:

- `Request`: model, messages, tools, system prompt, max tokens, temperature, provider-specific options.
- `Message`: role, content blocks, reasoning content.
- `Tool`: name, description, JSON Schema parameters.
- `Event`: text deltas, reasoning events, tool-call lifecycle, step finish, error, done.
- `TokenUsage`: input, output, cache read, cache write.

This should remain a pure LLM I/O package. Do not add memory, tools, storage, or agent policy to `comet-sdk`.

### CometMind API Contract

Current local API shape:

- `GET /api/v1/health`
- `POST /api/v1/sessions`
- `GET /api/v1/sessions`
- `GET /api/v1/sessions/{id}`
- `GET /api/v1/sessions/{id}/messages`
- `POST /api/v1/sessions/{id}/message` with SSE response
- `POST /api/v1/sessions/{id}/abort`

Current SSE event names:

- `text_delta`
- `reasoning_start`
- `reasoning_delta`
- `tool_call`
- `tool_result`
- `step_finish`
- `error`
- `done`

This API is the primary boundary between Cometline desktop and CometMind. Keep it stable and versioned.

### Cometline Desktop Boundary Contract

Cometline desktop should:

- Spawn the CometMind binary on app launch.
- Poll `GET /api/v1/health` until CometMind is ready.
- Load/render the Svelte UI.
- Use `fetch` and `ReadableStream` for CometMind REST/SSE calls.
- Use Electron IPC only for app version, process status, restart, native folder picker, notifications, and other OS-native operations.
- Keep `contextIsolation: true` and `nodeIntegration: false`.

Cometline desktop should not:

- Execute tools.
- Call LLM providers directly.
- Open or mutate the SQLite database directly in the normal app flow.
- Store provider API keys in renderer state or browser storage.

## Main Project Gaps

### 1. Permission Gates

Current state: CometMind executes tool calls directly once the model requests them.

Risk: `write_file` and `run_command` are impactful tools. Auto-executing them makes the local runtime unsafe once the app is used for real work.

Needed in `cometmind`:

- Tool metadata: read-only vs impactful.
- Tool-call state: pending, approved, denied, running, completed, failed.
- API/SSE event for permission request.
- API endpoint to approve or deny a pending tool call.
- Run loop pause/resume around pending permissions.

Needed in `cometline`:

- Permission prompt UI for pending tool calls.
- Approve, approve for session, deny controls.

### 2. Secret Storage

Current state: config file and environment variables are enough for development.

Risk: a personal desktop app will store long-lived API keys and OAuth tokens.

Needed:

- Move provider secrets out of plain config.
- Use OS keychain where possible.
- Store only secret references in SQLite/config.
- Ensure exports never include raw secrets.

Likely owner: CometMind, because CometMind makes provider calls.

### 3. Memory System

Current state: CometMind README says the runtime owns memory, but schema and implementation do not yet include memory tables or retrieval.

Needed in `cometmind`:

- `memories` table.
- Embedding model config.
- Embedding generation service.
- Vector search backend.
- Memory retrieval before model call.
- Memory extraction after conversation or explicit user request.
- Memory management API.

Needed in `cometline`:

- Memory settings screen.
- Memory list/search/edit/delete UI.
- UI indicator showing which memories were injected.

### 4. Provider And Config API Mismatch

Cometline desktop HLD references provider/config APIs such as `GET /api/v1/providers` and `POST /api/v1/config`.

CometMind OpenAPI currently only exposes the first session/message slice.

Needed:

- Reconcile Cometline desktop docs with CometMind OpenAPI before implementing UI.
- Add config/provider endpoints to CometMind or remove them from the Cometline desktop HLD until later.

Recommended CometMind endpoints:

- `GET /api/v1/config`
- `PATCH /api/v1/config`
- `GET /api/v1/providers`
- `POST /api/v1/providers/test`
- `GET /api/v1/models`

### 5. Workspace Semantics

Current state: `workspaces.path` is a real filesystem path.

For coding-agent use, this is correct.

For the personal assistant use case, you may also want logical spaces:

- `career`
- `health`
- `family`
- `finance`
- `learning`

Do not destroy filesystem workspaces. Instead, extend the model:

- Add `kind`: `filesystem` or `domain`.
- Add `metadata` JSON.
- Keep `path` nullable or introduce a separate table if needed.

### 6. Artifacts

Current state: not implemented.

The project does not need artifacts before memory and permissions. Add later for HTML, Mermaid, SVG, code previews, and generated mini apps.

### 7. MCP, Skills, Prompt Apps, Plugins, Chrome Relay

Current state: not implemented.

These are expansion layers, not the next priority. Build in this order:

1. MCP stdio tools.
2. Skills as filesystem prompt packs.
3. Prompt apps as saved prompt templates.
4. Plugin API if the app needs third-party extension.
5. Chrome Relay only after permissions and tool audit are mature.

## Roadmap Documents

Each repo now owns its own day-to-day phase plan:

- `comet-sdk/docs/PHASES.md` — provider-normalized model I/O milestones.
- `cometmind/docs/PHASES.md` — Go runtime, permissions, memory, skills, MCP, scheduler, browser, gateway.
- `cometline/docs/PHASES.md` — SvelteKit/Electron frontend milestones and UI responsibilities.

Use this architecture document for boundaries and sequencing; use the per-repo phase documents for daily execution.

## Recommended Build Order

### Phase 0: Stabilize Existing Contracts

Repo: `cometmind`, `cometline`

- Make OpenAPI match the Cometline desktop HLD or update the HLD to match OpenAPI.
- Decide the canonical API set for config, providers, models, sessions, messages, and abort.
- Generate or hand-maintain matching TypeScript API types for Cometline desktop.

### Phase 1: Permissioned Tool Loop

Repo: `cometmind`, `cometline`

- Add tool safety metadata.
- Add pending permission state to the run loop.
- Add approval/denial endpoints.
- Add Cometline desktop permission UI.
- Keep read-only tools auto-approved; gate `write_file` and `run_command`.

This is the highest-priority safety fix.

### Phase 2: Secrets And Provider Management

Repo: `cometmind`, `cometline`

- Add provider/config API.
- Add secret storage abstraction.
- Add provider test endpoint.
- Add model list endpoint.
- Add Cometline desktop settings UI for providers without storing secrets in renderer persistence.

### Phase 3: Memory MVP

Repo: `cometmind`, `cometline`

- Add memory tables.
- Add manual memory create/list/search/delete.
- Add retrieval before model requests.
- Add memory injection event or trace for UI visibility.
- Add auto-extraction only after manual memory works.

### Phase 4: Personal Domains

Repo: `cometmind`, `cometline`

- Extend workspace model into spaces/domains.
- Add per-space system prompt and memory scope.
- Keep filesystem workspaces for coding tasks.

### Phase 5: MCP And Skills

Repo: `cometmind`, `cometline`

- Add MCP stdio server management.
- Expose MCP tools through the same tool registry and permission system.
- Add skill directory scanning and skill selection.

### Phase 6: Artifacts And Browser

Repo: `cometmind`, `cometline`

- Add artifact detection and storage.
- Add sandboxed preview in Cometline desktop.
- Add built-in browser tools later.
- Add Chrome Relay only after browser automation permission UX is clear.

## Concrete Next Files To Touch

If building now, start here:

1. `cometmind/openapi.yaml`
   Define the real near-term API contract for config/providers/tool permissions.

2. `cometmind/internal/tools/tool.go`
   Add tool metadata such as safety class and permission requirement.

3. `cometmind/internal/agent/runner.go`
   Change automatic tool execution into a permission-aware loop.

4. `cometmind/internal/db/schema.sql`
   Add fields/tables for tool-call status and permission decisions.

5. `cometmind/server/server.go`
   Add permission approval/denial endpoints and SSE events.

6. `cometline/docs/HLD.md`
   Reconcile documented endpoints with CometMind OpenAPI.

7. `cometline` implementation files once they exist
   Add permission prompt UI and API client types.

## Load-Bearing Walls

For Cometline, the essential complexity is:

- `comet-sdk` normalizes provider streaming and tool calls.
- `cometmind` owns the trusted local agent runtime, persistence, tool execution, memory, and local API.
- `cometline` is a native shell and renderer, not an agent runtime.
- REST/SSE is the stable boundary between UI and brain.
- Tool execution must become permissioned before powerful tools expand.
- Secrets and memory must be local-first and auditable.

Everything else is swappable: Svelte vs another renderer, Electron vs another desktop shell, exact database extensions, exact provider SDKs, visual design, and future plugin systems.
