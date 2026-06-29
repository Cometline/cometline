# Module Guide

This is the short operational guide for developers and agents working in the
monorepo after the v1 milestone.

For system walkthroughs, start with `../ARCHITECTURE.md` and
`../ARCHITECTURE_GUIDE.md`. This file focuses on ownership, design rules, and
how to change each module without leaking responsibilities across boundaries.

## Dependency Rule

The dependency direction is strict:

```text
cometline -> cometmind -> comet-sdk
```

Rules:

- `cometline` may call CometMind over REST/SSE and Electron IPC, but it must not become a second runtime.
- `cometmind` may depend on `comet-sdk`'s normalized provider interface, but not on frontend concerns.
- `comet-sdk` must stay provider-facing only. It should not grow persistence, agent-loop, or UI behavior.
- There is no root `go.work`; run Go commands from `comet-sdk/` or `cometmind/`.

## `comet-sdk`

### Purpose

`comet-sdk` is the provider-normalized LLM I/O boundary.

It owns:

- provider request/response adaptation
- streaming SSE parsing
- tool-call delta assembly
- normalized finish reasons and typed errors
- retry helpers and provider fixtures

It must not own:

- agent loops
- session persistence
- settings UI
- tool execution policy
- Electron or desktop behavior

### Key files

- `comet-sdk/sdk.go`
- `comet-sdk/errors.go`
- `comet-sdk/llm/stream.go`
- `comet-sdk/llm/collect.go`
- `comet-sdk/internal/providerbase/providerbase.go`
- `comet-sdk/internal/retry/retry.go`
- `comet-sdk/internal/sse/scanner.go`
- `comet-sdk/provider/openai/`
- `comet-sdk/provider/anthropic/`

### Design rules

- Normalize provider quirks here, not in CometMind or Cometline.
- If a provider streams partial reasoning or tool-call deltas, assemble and normalize them here.
- Keep public types provider-agnostic. Higher layers should not branch on raw wire fields.
- Use fixtures and parser tests when changing streaming behavior.

### When you are changing `comet-sdk`

Ask:

- Is this a provider transport/parsing concern or am I pushing runtime policy downward?
- Can CometMind consume this through existing normalized events?
- Do fixtures and tests cover the new streaming shape?

## `cometmind`

### Purpose

`cometmind` is the local agent runtime and source of truth.

It owns:

- agent orchestration
- workspace/session/message persistence
- SQLite schema and migrations
- tool registry and execution
- semantic memory and compaction
- provider factory and runtime config
- localhost REST/SSE API
- ACP, MCP, jobs, and Discord gateway integration

It must not own:

- renderer state
- Electron-native APIs
- frontend styling or route concerns
- provider wire-format special cases that belong in `comet-sdk`

### Key files

- `cometmind/internal/runtime/runtime.go`
- `cometmind/internal/agent/runner.go`
- `cometmind/internal/session/service.go`
- `cometmind/internal/tools/registry.go`
- `cometmind/internal/memory/service.go`
- `cometmind/internal/provider/factory.go`
- `cometmind/internal/event/event.go`
- `cometmind/server/server.go`
- `cometmind/openapi.yaml`
- `cometmind/internal/db/schema.sql`
- `cometmind/internal/db/migrate.go`

### Design rules

- Treat `openapi.yaml` as the HTTP contract source of truth.
- Treat `internal/event/event.go` as the SSE contract source of truth.
- Treat the SQLite schema and sqlc queries as load-bearing public internals. Schema changes need migration thinking, not just table edits.
- Tool execution must stay workspace-scoped and runtime-controlled.
- The runtime should be able to start with no configured provider and surface a usable settings-driven recovery path.

### When you are changing `cometmind`

Ask:

- Does this change affect the OpenAPI contract, SSE contract, database schema, or generated code?
- Does Cometline need reducer/store/UI updates for the new runtime behavior?
- Does this belong in runtime config, or is it a frontend-only preference?

## `cometline`

### Purpose

`cometline` is the desktop shell and renderer.

It owns:

- Electron main/preload behavior
- CometMind sidecar lifecycle
- settings UX and native persistence orchestration
- renderer API client and stream handling
- chat/session UI and transitions
- slash-command and skill UX
- updater, tray, workspace picker, web panel, and shortcuts

It must not own:

- transcript source of truth
- direct database writes
- tool execution
- provider request assembly
- runtime-only policy that belongs in CometMind

### Key files

- `cometline/electron/main.cjs`
- `cometline/electron/preload.cjs`
- `cometline/src/lib/client/cometmind.ts`
- `cometline/src/lib/reducers/chat.ts`
- `cometline/src/lib/stores/chat.svelte.ts`
- `cometline/src/lib/stores/model.svelte.ts`
- `cometline/src/lib/stores/settings.svelte.ts`
- `cometline/src/lib/settings/schema.ts`
- `cometline/src/routes/+page.svelte`
- `cometline/src/routes/session/[id]/+page.svelte`

### Design rules

- REST/SSE is for runtime data. Electron IPC is for OS/native capabilities.
- Renderer settings are a draft until explicitly persisted.
- Generated API code under `src/lib/generated/cometmind-api/` is not hand-edited.
- Keep styling token-first and aligned with `cometline/STYLING.md` and `cometline/docs/FRONTEND_PATTERNS.md`.
- Preserve the distinction between immediate settings actions and pending-save settings.

### When you are changing `cometline`

Ask:

- Is this renderer state or runtime state?
- Should this go through REST/SSE, Electron IPC, or both?
- Does this change affect sidecar restart rules or shared settings persistence?
- Am I keeping the settings modal draft semantics intact?

## Cross-Module Change Map

### OpenAPI changes

When you change `cometmind/openapi.yaml`, also update or regenerate:

- `cometmind/internal/apigen/types.gen.go`
- `cometline/src/lib/generated/cometmind-api/`
- renderer callsites that consume the changed API

Run `make generate` from the repo root.

### SSE event changes

When you change runtime events, also update:

- `cometmind/internal/event/event.go`
- `cometline/src/lib/types.ts`
- `cometline/src/lib/reducers/chat.ts`
- relevant contract tests and UI rendering paths

### Settings shape changes

When you change shared settings, inspect all of:

- `cometline/src/lib/settings/schema.ts`
- `cometline/src/lib/stores/settings.svelte.ts`
- `cometline/src/lib/settings/persist.ts`
- `cometline/src/lib/components/settings/`
- `cometline/electron/main.cjs`
- `cometmind/internal/config/`

Do not update only one layer.

### Memory changes

Memory spans both frontend and runtime. Be explicit about which category the change belongs to:

- memory UI draft and save flow in `cometline`
- live memory runtime config and handlers in `cometmind`
- embedding provider/model linkage inside shared settings

## Post-v1 Contributor Rules

- Prefer the smallest correct change that preserves current boundaries.
- Document design guardrails when a bug taught the team a non-obvious lesson.
- Fix doc drift when you find it, especially around settings and persistence behavior.
- Do not add new persistence paths casually. Reuse the existing source of truth unless there is a strong reason not to.
- When a change crosses modules, leave an obvious paper trail in docs, tests, and generated artifacts.
