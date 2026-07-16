# Cometline Learning Guide

A progressive reading path through the Cometline monorepo. Start at the top and work downward — each doc assumes you have read the ones above it.

These guides are built from the **GitNexus knowledge graph** for `cometline-release` (10,327 symbols, 28,474 relationships, 300 execution flows) plus the canonical architecture docs (`ARCHITECTURE.md`, `ARCHITECTURE_GUIDE.md`).

## Who this is for

- New contributors onboarding to the monorepo
- Developers who know one layer (frontend or Go) but need the full picture
- Anyone who wants to understand *why* the boundaries exist, not just where files live

## Reading order

| # | Doc | Time | What you'll learn |
|---|-----|------|-------------------|
| 1 | [01-nutshell.md](./01-nutshell.md) | ~10 min | What Cometline is, the three modules, one message end-to-end |
| 2 | [02-architecture.md](./02-architecture.md) | ~20 min | Ownership rules, dependency direction, load-bearing contracts |
| 3 | [03-data-flows.md](./03-data-flows.md) | ~25 min | Startup, first message, agent loop, settings save/reload, MCP, jobs, packaging |
| 4 | [04-comet-sdk.md](./04-comet-sdk.md) | ~30 min | Provider interface, streaming, retries, tool-call assembly |
| 5 | [05-cometmind-runtime.md](./05-cometmind-runtime.md) | ~35 min | Agent runner, sessions, SQLite, HTTP/SSE server |
| 6 | [06-cometmind-features.md](./06-cometmind-features.md) | ~30 min | Memory, MCP, coding harness, Discord, skills, jobs |
| 7 | [07-cometline-desktop.md](./07-cometline-desktop.md) | ~25 min | Electron main, sidecar lifecycle, IPC, settings persistence |
| 8 | [08-cometline-frontend.md](./08-cometline-frontend.md) | ~30 min | SvelteKit routes, stores, SSE reducer, chat/jobs/settings UI |
| 9 | [09-contracts-codegen.md](./09-contracts-codegen.md) | ~20 min | OpenAPI, SSE events, sqlc, generated clients |
| 10 | [10-development-guide.md](./10-development-guide.md) | ~20 min | Build/test commands, extension recipes, verification |

**Total:** ~4 hours for a thorough first pass. Skim 01–03 if you only need orientation.

## Quick reference by goal

| I want to… | Start here |
|------------|------------|
| Understand the whole system in 10 minutes | [01-nutshell.md](./01-nutshell.md) |
| Trace what happens when I send a chat message | [03-data-flows.md](./03-data-flows.md) → Flow 2 & 3 |
| Add a new LLM provider | [04-comet-sdk.md](./04-comet-sdk.md) → [10-development-guide.md](./10-development-guide.md) |
| Add a built-in tool | [05-cometmind-runtime.md](./05-cometmind-runtime.md) → Tools section |
| Fix streaming UI bugs | [08-cometline-frontend.md](./08-cometline-frontend.md) |
| Change the API or SSE contract | [09-contracts-codegen.md](./09-contracts-codegen.md) |
| Understand MCP OAuth | [06-cometmind-features.md](./06-cometmind-features.md) → MCP section |
| Understand coding-harness delegation | [06-cometmind-features.md](./06-cometmind-features.md) → Coding task delegation |
| Understand jobs/scheduled jobs | [06-cometmind-features.md](./06-cometmind-features.md) → Jobs section |
| Understand settings persistence | [07-cometline-desktop.md](./07-cometline-desktop.md) → Settings persistence + [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md) |

## Companion docs (outside this series)

These existing docs go deeper on specific topics:

- [../MODULE_GUIDE.md](../MODULE_GUIDE.md) — module ownership checklists for agents
- [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md) — settings modal draft/save rules
- [../FRONTEND_DESIGN_SYSTEM.md](../FRONTEND_DESIGN_SYSTEM.md) — visual tokens and styling
- [../../ARCHITECTURE.md](../../ARCHITECTURE.md) — system overview (source of truth)
- [../../ARCHITECTURE_GUIDE.md](../../ARCHITECTURE_GUIDE.md) — contributor map with line references
- [../../AGENTS.md](../../AGENTS.md) — dev commands and codegen rules

## GitNexus tips while reading

Use these CLI commands from the repo root to explore beyond the docs:

```bash
node .gitnexus/run.cjs query "your concept"     # find execution flows
node .gitnexus/run.cjs context "SymbolName" -f path/to/file.go  # callers/callees
node .gitnexus/run.cjs status                   # check index freshness
```

The central chain every doc revolves around:

```
Provider SSE → comet-sdk events → CometMind events → HTTP/SSE → renderer reducer → ChatItem rows
```

Preserve that chain and the system stays coherent.
