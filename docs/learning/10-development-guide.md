# 10 - Development Guide

> **Prerequisite:** [09-contracts-codegen.md](./09-contracts-codegen.md)  
> This is the practical reference for daily work.

## Prerequisites

- macOS 13+ (primary target)
- Go 1.25+
- Node.js + pnpm
- Optional: `sqlc` and `golangci-lint`, for workflows beyond the basic commands

The primary target is the main system this app is built for. A workflow here is a set of commands for one kind of task.

## First-time setup

```bash
git clone https://github.com/cometline/cometline.git
cd cometline
make install    # pnpm install in cometline/
make dev        # build sidecar + launch Electron dev app
```

A sidecar is the CometMind process that runs next to the desktop app.

## Command reference

### Root Makefile

Codegen means code generation. Freshness means the generated files still match their sources.

| Command          | What it does                                  |
| ---------------- | --------------------------------------------- |
| `make install`   | Frontend dependencies                         |
| `make generate`  | Regenerate OpenAPI clients (TS + Go)          |
| `make check`     | Codegen freshness + all tests + Svelte checks |
| `make build`     | SDK + CometMind + renderer production build   |
| `make package`   | macOS Electron package with sidecar           |
| `make dev`       | Dev sidecar + Electron                        |
| `make port`      | Show process listening on `127.0.0.1:7700`    |
| `make clean-log` | Remove `~/.cometmind/logs/cometline*.log`     |

A renderer is the part of the app that draws the UI. OpenAPI is a file format that describes an HTTP API.

### comet-sdk

```bash
cd comet-sdk
make test              # CI-safe unit tests
make test-live         # Live provider tests (needs API keys)
make build             # Verify compilation
make lint              # golangci-lint
```

CI-safe means the tests do not call a live provider. CI is the automated check that runs on a proposed change. Live tests call a real API and need API keys.

### cometmind

```bash
cd cometmind
go test ./...                                    # All tests
go test -run TestPostMessageStreamsSSE ./server  # Specific test
go build ./...                                   # Verify compile
sqlc generate                                    # After schema/query changes
```

CLI during development:

```bash
cd cometmind
go run . init --workspace /path/to/project
go run . serve --port 7700
go run . chat "hello"
```

### cometline

```bash
cd cometline
pnpm run dev       # Vite + Electron
pnpm run check     # Type checking
pnpm run test      # Vitest
pnpm run lint      # ESLint
pnpm run storybook # Storybook
pnpm run build     # Production SvelteKit build
```

## GitNexus for exploration

The current GitNexus repo name is `cometline-release`.

```bash
node .gitnexus/run.cjs status
node .gitnexus/run.cjs query "concept you're exploring"
node .gitnexus/run.cjs context "SymbolName" -f path/to/file.go
node .gitnexus/run.cjs impact "SymbolName"    # before editing
```

Run the impact command before you edit a symbol. It shows what else uses that name. A symbol is a named function, type, or method.

Re-index after large changes with `node .gitnexus/run.cjs analyze`.

---

## Extension recipes

A recipe is a short step list for one kind of change.

### Add a new LLM provider

1. In `comet-sdk/provider/<name>/`, implement the `Provider` interface.
2. Add fixtures and `stream_test.go`. A fixture is a saved sample used by a test.
3. In `cometmind/internal/provider/factory.go`, connect the provider ID.
4. In `cometmind/internal/config/config.go`, add a method constant if one is needed.
5. In `cometline/src/lib/settings/schema.ts`, add validation.
6. In `cometline/src/lib/types.ts`, add the `ProviderMethod` type.
7. In `SettingsProvidersPanel.svelte`, add UI fields if the provider is not standard.
8. In `electron/src/domains/provider-auth.ts`, add model discovery, Codex or xAI auth, or both, if the provider is subscription-based.
9. Run `cd comet-sdk && make test && cd ../cometmind && go test ./...`.

An LLM is a large language model. Subscription-based means the provider uses a sign-in session, not an API key.

### Enable coding-harness delegation

1. Open Settings, then CometMind, then **Coding task delegation**. Enable it, and pick `default_harness` (`opencode`, `claude`, or `codex`).
2. Make sure the harness CLI is on `PATH`. `PATH` is the list of folders the shell searches for programs.
3. Do not edit the command or the args in settings. They are fixed in `cometmind/internal/acp/runner.go`.
4. The agent uses `delegate_coding_task` when that tool is registered.

A harness is an external coding program. Delegation means the agent hands that coding task to the harness.

### Add a built-in tool

1. Create `cometmind/internal/tools/<name>.go`.
2. Implement the `Tool` interface with `Spec()` and `Execute()`.
3. Register it in `registry.go` with the right `ToolSurface` flags in `surface.go`.
4. Add unit tests for the schema and for execution.
5. Consider the workspace sandbox in `sandbox/pathcheck.go`. A sandbox limits which files a tool may use.
6. Run `go test ./internal/tools/...`.

### Add a REST endpoint

1. Edit `cometmind/openapi.yaml`.
2. In `cometmind/server/server.go`, add the handler and the route.
3. Run `make generate`.
4. In `cometline/src/lib/client/cometmind.ts`, add the client function.
5. Add a server test in `server/*_test.go`.

REST is a request-and-response web API. An endpoint is one API path.

### Add an SSE event type

1. In `openapi.yaml`, extend the `StreamEvent` schema.
2. In `internal/event/event.go`, add the struct and the emitter.
3. Run `make generate`.
4. In `cometline/src/lib/types.ts`, add the union member.
5. Add a case in `reducers/chat.ts`, in the runtime toast or layout consumer, or in both.
6. Add a UI component if a new visual is needed.
7. Update `internal/contract/contract_test.go`.

SSE means Server-Sent Events: a live stream of events from the server. An emitter is a function that sends one event. A union member is one shape inside a type that can be several shapes.

### Change database schema

1. Edit `internal/db/schema.sql`.
2. In `internal/db/migrate.go`, add an incremental migration. Incremental means one small step from the old version to the new one.
3. Edit `internal/db/queries/*.sql`.
4. Run `sqlc generate`.
5. In `internal/session/service.go`, update the domain logic.
6. Run `go test ./internal/session/... ./server/...`.

### Add an Agent Skill

1. Create `~/.cometmind/skills/<name>/SKILL.md`.
2. Add YAML frontmatter with `name` and `description`. Frontmatter is the YAML block at the top of the file.
3. Write a Markdown body with the workflow and examples.
4. Invoke it with `/<name>` in the composer.

### Add a settings field

1. In `settings/schema.ts`, add the type and the normalization. Normalize means rewrite the value into the expected form.
2. Decide whether the key is desktop or runtime. Desktop keys go in `cometline-desktop.json`, under `appearance`, `shortcuts`, or `app`.
3. Add a UI control in a settings panel under `components/settings/`.
4. Update `electron/src/domains/settings.ts` and `settings-domain.ts` if the save or load split must change.
5. Update `cometmind/internal/config/` and `settingsapply` so the runtime can read the field and classify the change. Classify means choose reload, gateway recycle, or full restart.
6. Choose pending-save, instant-save, or action-based.
7. See [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md).

### Change jobs or scheduled jobs

1. Edit `cometmind/internal/jobs/` or `cometmind/internal/scheduler/`.
2. If saved data changes, update `cometmind/internal/db/schema.sql`, the queries, and the migrations.
3. If the API shape changes, update `cometmind/openapi.yaml` and run `make generate`.
4. Update `cometline/src/lib/client/cometmind.ts`.
5. Update `cometline/src/lib/components/jobs/` and `cometline/src/lib/jobs/`.
6. Check that leases, events, retention, and notifications are still correct.

A lease is a time-limited claim on a job. Retention means how long finished data is kept.

### Change MCP behavior

1. Edit `cometmind/internal/mcp/`.
2. If tool exposure changes, update `cometmind/internal/tools/registry.go`.
3. For management API changes, update `cometmind/openapi.yaml` and the generated clients.
4. Update `cometline/src/lib/components/settings/SettingsMCPPanel.svelte`.
5. For OAuth or import IPC, update `cometline/electron/src/domains/provider-auth.ts`, `runtime-ipc.ts`, `preload.ts`, and `shared/api.ts`.

MCP is a protocol for external tools. OAuth is a standard login flow. IPC means messages between Electron processes.

---

## Verification before PR

```bash
make check    # full gate from repo root
```

A PR is a pull request: a proposed set of changes. A gate here means the full set of checks.

Minimum per module:

```bash
cd comet-sdk && go test ./...
cd cometmind && go test ./...
cd cometline && pnpm run check && pnpm run test
```

Manual smoke test:

```bash
make dev
# Send a message, verify streaming
# Change provider in Settings, verify in-place reload (not full restart)
# Only host/port changes should full-restart the sidecar
# Configure/test an MCP server if touching MCP
# Create/claim/complete a job if touching jobs
# Run a tool (e.g. list_dir)
```

A smoke test is a short manual check that the main path still works.

---

## Commit conventions

```
type(scope): description

feat(cometline): add default model picker to settings
fix(cometmind): prevent tool execution path escape
refactor(comet-sdk): extract retry logic into helper
```

Scopes are `cometline`, `cometmind`, `comet-sdk`, or the cross-cutting scopes `docs` and `ci`. Cross-cutting means the change is not limited to one module.

---

## Runtime paths (debugging)

| Path                                      | Contents                                |
| ----------------------------------------- | --------------------------------------- |
| `~/.cometmind/cometline-settings.json`    | Runtime settings (providers, cometmind) |
| `~/.cometmind/cometline-desktop.json`     | Desktop UI settings                     |
| `~/.cometmind/cometline-workspace.json`   | Selected workspace                      |
| `~/.cometmind/cometmind.db`               | SQLite                                  |
| `~/.cometmind/logs/cometline.log`         | Sidecar logs                            |
| `~/.cometmind/logs/cometline-gateway.log` | Discord gateway logs                    |
| `~/.cometmind/mcp-oauth/`                 | MCP tokens                              |
| `~/.cometmind/tool-output/`               | Spilled tool output                     |
| `~/.cometmind/skills/`                    | Global skills                           |
| `~/.cometmind/skill-drafts/`              | Draft skills                            |

An MCP token is a saved login value for an MCP server. Spilled tool output is tool output saved in this folder. A draft skill is a skill that is not final yet.

Environment overrides:

```bash
COMETMIND_PROVIDER=anthropic
COMETMIND_API_KEY=...
COMETMIND_MODEL=claude-sonnet-4-5
ANTHROPIC_API_KEY=...
OPENAI_API_KEY=...
# Codex / xAI use local subscription sessions, not these keys
```

A subscription session is stored on this machine. It is not one of these API keys.

---

## Troubleshooting

| Problem                 | Check                                                                       |
| ----------------------- | --------------------------------------------------------------------------- |
| Sidecar will not start  | `~/.cometmind/logs/cometline.log`, `make port`                              |
| Settings are not saved  | Is the JSON valid? Run `jq . ~/.cometmind/cometline-settings.json`          |
| Streaming has stopped updating | Reducer immutability; check the browser console                       |
| A Go test fails         | Are you in the correct module directory?                                    |
| Generated files no longer match | Run `make generate`, then `make check`                               |
| The GitNexus index is old | `node .gitnexus/run.cjs analyze`                                          |
| MCP OAuth is broken     | Check `~/.cometmind/mcp-oauth/` and Settings → CometMind → MCP status       |
| A job stays ongoing     | Check job lease expiry, `job_events`, and autonomous worker settings        |

Immutability means the reducer must not edit the old array in place. Ongoing means the job is still marked as running. Autonomous means the worker can continue without a new user message.

---

## Learning path recap

| #   | Doc               | You learned                                          |
| --- | ----------------- | ---------------------------------------------------- |
| 01  | Nutshell          | What Cometline is, and the path of one message       |
| 02  | Architecture      | Boundaries, contracts, and invariants                |
| 03  | Data flows        | Startup, the agent loop, settings, and packaging     |
| 04  | comet-sdk         | The provider interface, streaming, and retries       |
| 05  | cometmind runtime | Runner, sessions, server, and tools                  |
| 06  | Features          | Memory, MCP, the coding harness, Discord, skills, and jobs |
| 07  | Desktop           | Electron, the sidecar, IPC, the settings split, and native OAuth |
| 08  | Frontend          | Routes, stores, the reducer, and components          |
| 09  | Contracts         | OpenAPI, sqlc, and the codegen workflow              |
| 10  | Development       | Commands, recipes, and verification                  |

An invariant is a rule that must stay true. Streaming means the reply arrives in small pieces while the model is still writing.

## Further reading

- [../../AGENTS.md](../../AGENTS.md): rules for agents and for development automation
- [../../ARCHITECTURE_GUIDE.md](../../ARCHITECTURE_GUIDE.md): a contributor map that points at specific lines
- [../MODULE_GUIDE.md](../MODULE_GUIDE.md): ownership checklists
- [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md): the settings split and reload rules
- [cometmind/openapi.yaml](../../cometmind/openapi.yaml): the API spec
