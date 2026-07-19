# 10 — Development Guide

> **Prerequisite:** [09-contracts-codegen.md](./09-contracts-codegen.md)  
> **You made it.** This is the practical reference for day-to-day work.

## Prerequisites

- macOS 13+ (primary target)
- Go 1.25+
- Node.js + pnpm
- Optional: `sqlc`, `golangci-lint` for extended workflows

## First-time setup

```bash
git clone https://github.com/cometline/cometline.git
cd cometline
make install    # pnpm install in cometline/
make dev        # build sidecar + launch Electron dev app
```

## Command reference

### Root Makefile

| Command | What it does |
|---------|--------------|
| `make install` | Frontend dependencies |
| `make generate` | Regenerate OpenAPI clients (TS + Go) |
| `make check` | Codegen freshness + all tests + Svelte checks |
| `make build` | SDK + CometMind + renderer production build |
| `make package` | macOS Electron package with sidecar |
| `make dev` | Dev sidecar + Electron |
| `make port` | Show process listening on `127.0.0.1:7700` |
| `make clean-log` | Remove `~/.cometmind/logs/cometline*.log` |

### comet-sdk

```bash
cd comet-sdk
make test              # CI-safe unit tests
make test-live         # Live provider tests (needs API keys)
make build             # Verify compilation
make lint              # golangci-lint
```

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

Re-index after major changes: `node .gitnexus/run.cjs analyze`

---

## Extension recipes

### Add a new LLM provider

1. `comet-sdk/provider/<name>/` — implement `Provider` interface
2. Add fixtures + `stream_test.go`
3. `cometmind/internal/provider/factory.go` — wire provider ID
4. `cometmind/internal/config/config.go` — method constant if needed
5. `cometline/src/lib/settings/schema.ts` — validation
6. `cometline/src/lib/types.ts` — `ProviderMethod` type
7. `SettingsProvidersPanel.svelte` — UI fields if non-standard
8. `electron/main.cjs` — model discovery and/or Codex/xAI auth if subscription-based
9. `cd comet-sdk && make test && cd ../cometmind && go test ./...`

### Enable coding-harness delegation

1. Settings → CometMind → **Coding task delegation** → enable + pick `default_harness` (`opencode` / `claude` / `codex`)
2. Ensure the harness CLI is on `PATH`
3. Do **not** edit command/args in settings — fixed in `cometmind/internal/acp/runner.go`
4. Agent uses `delegate_coding_task` when registered

### Add a built-in tool

1. Create `cometmind/internal/tools/<name>.go`
2. Implement `Tool` interface (`Spec()`, `Execute()`)
3. Register in `registry.go` via the appropriate `ToolSurface` flags in `surface.go`
4. Add unit tests for schema + execution
5. Consider workspace sandbox (`sandbox/pathcheck.go`)
6. `go test ./internal/tools/...`

### Add a REST endpoint

1. `cometmind/openapi.yaml`
2. `cometmind/server/server.go` — handler + route
3. `make generate`
4. `cometline/src/lib/client/cometmind.ts` — client function
5. Server test in `server/*_test.go`

### Add an SSE event type

1. `openapi.yaml` — `StreamEvent` schema
2. `internal/event/event.go` — struct + emitter
3. `make generate`
4. `cometline/src/lib/types.ts` — union member
5. `reducers/chat.ts` — case handler **and/or** runtime toast/layout consumer
6. UI component if new visual needed
7. `internal/contract/contract_test.go`

### Change database schema

1. `internal/db/schema.sql`
2. `internal/db/migrate.go` — incremental migration
3. `internal/db/queries/*.sql`
4. `sqlc generate`
5. `internal/session/service.go` — domain updates
6. `go test ./internal/session/... ./server/...`

### Add an Agent Skill

1. Create `~/.cometmind/skills/<name>/SKILL.md`
2. YAML frontmatter: `name`, `description`
3. Markdown body with workflow and examples
4. Invoke with `/<name>` in composer

### Add a settings field

1. `settings/schema.ts` — type + normalization
2. Decide desktop vs runtime: desktop keys go in `cometline-desktop.json` (`appearance` / `shortcuts` / `app`)
3. Settings panel module under `components/settings/` — UI control
4. `electron/main.cjs` — save/load split path if needed
5. `cometmind/internal/config/` + `settingsapply` — runtime consumption / classify
6. Decide: pending-save vs instant-save vs action-based
7. See [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md)

### Change jobs or scheduled jobs

1. `cometmind/internal/jobs/` or `cometmind/internal/scheduler/`
2. `cometmind/internal/db/schema.sql` / queries / migrations if persistence changes
3. `cometmind/openapi.yaml` and `make generate` if API shape changes
4. `cometline/src/lib/client/cometmind.ts`
5. `cometline/src/lib/components/jobs/` and `cometline/src/lib/jobs/`
6. Verify leases, events, retention, and notifications still make sense

### Change MCP behavior

1. `cometmind/internal/mcp/`
2. `cometmind/internal/tools/registry.go` if tool exposure changes
3. `cometmind/openapi.yaml` and generated clients for management API changes
4. `cometline/src/lib/components/settings/SettingsMCPPanel.svelte`
5. `cometline/electron/main.cjs` / `preload.cjs` for OAuth or import IPC

---

## Verification before PR

```bash
make check    # full gate from repo root
```

Minimum per-module:

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

---

## Commit conventions

```
type(scope): description

feat(cometline): add default model picker to settings
fix(cometmind): prevent tool execution path escape
refactor(comet-sdk): extract retry logic into helper
```

Scopes: `cometline`, `cometmind`, `comet-sdk`, or cross-cutting `docs`, `ci`.

---

## Runtime paths (debugging)

| Path | Contents |
|------|----------|
| `~/.cometmind/cometline-settings.json` | Runtime settings (providers, cometmind) |
| `~/.cometmind/cometline-desktop.json` | Desktop UI settings |
| `~/.cometmind/cometline-workspace.json` | Selected workspace |
| `~/.cometmind/cometmind.db` | SQLite |
| `~/.cometmind/logs/cometline.log` | Sidecar logs |
| `~/.cometmind/logs/cometline-gateway.log` | Discord gateway logs |
| `~/.cometmind/mcp-oauth/` | MCP tokens |
| `~/.cometmind/tool-output/` | Spilled tool output |
| `~/.cometmind/skills/` | Global skills |
| `~/.cometmind/skill-drafts/` | Draft skills |

Environment overrides:

```bash
COMETMIND_PROVIDER=anthropic
COMETMIND_API_KEY=...
COMETMIND_MODEL=claude-sonnet-4-5
ANTHROPIC_API_KEY=...
OPENAI_API_KEY=...
# Codex / xAI use local subscription sessions, not these keys
```

---

## Troubleshooting

| Problem | Check |
|---------|-------|
| Sidecar won't start | `~/.cometmind/logs/cometline.log`, `make port` |
| Settings not persisting | JSON valid? `jq . ~/.cometmind/cometline-settings.json` |
| Streaming frozen | Reducer immutability; check browser console |
| Go test fails | Running from correct module dir? |
| Codegen drift | `make generate` then `make check` |
| Stale GitNexus | `node .gitnexus/run.cjs analyze` |
| MCP OAuth broken | Check `~/.cometmind/mcp-oauth/` files and Settings → CometMind → MCP status |
| Job stuck ongoing | Check job lease expiry, `job_events`, and autonomous worker settings |

---

## Learning path recap

| # | Doc | You learned |
|---|-----|-------------|
| 01 | Nutshell | What Cometline is, one-message journey |
| 02 | Architecture | Boundaries, contracts, invariants |
| 03 | Data flows | Startup, agent loop, settings, packaging |
| 04 | comet-sdk | Provider interface, streaming, retries |
| 05 | cometmind runtime | Runner, sessions, server, tools |
| 06 | Features | Memory, MCP, coding harness, Discord, skills, jobs |
| 07 | Desktop | Electron, sidecar, IPC, settings split, native OAuth |
| 08 | Frontend | Routes, stores, reducer, components |
| 09 | Contracts | OpenAPI, sqlc, codegen workflow |
| 10 | Development | Commands, recipes, verification |

## Further reading

- [../../AGENTS.md](../../AGENTS.md) — agent/dev automation rules
- [../../ARCHITECTURE_GUIDE.md](../../ARCHITECTURE_GUIDE.md) — line-level contributor map
- [../MODULE_GUIDE.md](../MODULE_GUIDE.md) — ownership checklists
- [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md) — settings split and reload rules
- [cometline/docs/postmortem/](../../cometline/docs/postmortem/) — incident writeups
- [cometmind/openapi.yaml](../../cometmind/openapi.yaml) — API spec
