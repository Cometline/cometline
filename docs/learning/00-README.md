# Cometline Learning Guide

This is a reading path for the Cometline monorepo. A **monorepo** is one git repository that holds several modules. Start at the top and read down. Each doc expects you to have read the docs above it.

These guides are built from the **GitNexus knowledge graph** for `cometline-release`. They also use the main architecture docs (`ARCHITECTURE.md`, `ARCHITECTURE_GUIDE.md`). A knowledge graph is a stored map of code names and the links between them. Use GitNexus status when you need current index counts. Do not copy counts that change over time into these docs.

The tables below use a few technical words.

- **SSE** means Server-Sent Events. The server sends events to the app on one open connection.
- A **renderer** is the window process that draws the user interface.
- A **sidecar** is a helper process. The desktop app starts it and stops it.
- A **contract** is a shared description of an API or event. Both sides must follow it.
- **IPC** means inter-process communication. It is how two processes on one machine talk.
- **Persistence** means saving data so it remains after a restart.
- A **reducer** turns stream events into UI state.
- **MCP** means Model Context Protocol. It connects external tool servers.
- **OAuth** is a sign-in flow for a protected remote server.
- A **coding harness** is an external coding program. **Delegation** means sending a task to that program.

## Who this is for

- New contributors joining the monorepo
- Developers who know one layer (frontend or Go) but need the whole system
- Anyone who wants to know *why* the boundaries exist, not only where the files are

## Reading order

| # | Doc | Time | What you'll learn |
|---|-----|------|-------------------|
| 1 | [01-nutshell.md](./01-nutshell.md) | ~10 min | What Cometline is, the three modules, one message from start to finish |
| 2 | [02-architecture.md](./02-architecture.md) | ~20 min | Ownership rules, dependency direction, contracts the system depends on |
| 3 | [03-data-flows.md](./03-data-flows.md) | ~25 min | Startup, first message, agent loop, settings save/reload, MCP, jobs, packaging |
| 4 | [04-comet-sdk.md](./04-comet-sdk.md) | ~30 min | Provider interface, streaming, retries, tool-call assembly (joining partial tool calls) |
| 5 | [05-cometmind-runtime.md](./05-cometmind-runtime.md) | ~35 min | Agent runner, sessions, SQLite, HTTP/SSE server |
| 5a | [05a-output-limit.md](./05a-output-limit.md) | ~15 min | Step output limit, thinking, and context reserve (space kept for the reply) |
| 6 | [06-cometmind-features.md](./06-cometmind-features.md) | ~30 min | Memory, MCP, coding harness, Discord, skills, jobs |
| 7 | [07-cometline-desktop.md](./07-cometline-desktop.md) | ~25 min | Electron main, sidecar lifecycle (start, run, and stop), IPC, settings persistence |
| 8 | [08-cometline-frontend.md](./08-cometline-frontend.md) | ~30 min | SvelteKit routes, stores, SSE reducer, chat/jobs/settings UI |
| 9 | [09-contracts-codegen.md](./09-contracts-codegen.md) | ~20 min | OpenAPI, SSE events, sqlc (generates Go from SQL), generated clients |
| 10 | [10-development-guide.md](./10-development-guide.md) | ~20 min | Build/test commands, extension steps, and how to check them |

**Total:** about 4 hours for a careful first read. Read 01 to 03 quickly if you only need a first look.

## Quick reference by goal

| I want to… | Start here |
|------------|------------|
| Understand the whole system in 10 minutes | [01-nutshell.md](./01-nutshell.md) |
| Follow what happens when I send a chat message | [03-data-flows.md](./03-data-flows.md) → Flow 2 & 3 |
| Understand the reply size limit | [05a-output-limit.md](./05a-output-limit.md) |
| Add a new LLM provider | [04-comet-sdk.md](./04-comet-sdk.md) → [10-development-guide.md](./10-development-guide.md) |
| Add a built-in tool | [05-cometmind-runtime.md](./05-cometmind-runtime.md) → Tools section |
| Fix streaming UI bugs | [08-cometline-frontend.md](./08-cometline-frontend.md) |
| Change the API or SSE contract | [09-contracts-codegen.md](./09-contracts-codegen.md) |
| Understand MCP OAuth | [06-cometmind-features.md](./06-cometmind-features.md) → MCP section |
| Understand coding-harness delegation | [06-cometmind-features.md](./06-cometmind-features.md) → Coding task delegation |
| Understand jobs and scheduled jobs | [06-cometmind-features.md](./06-cometmind-features.md) → Jobs section |
| Understand settings persistence | [07-cometline-desktop.md](./07-cometline-desktop.md) → Settings persistence + [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md) |

## Companion docs (outside this series)

These docs cover one topic in more detail:

- [../MODULE_GUIDE.md](../MODULE_GUIDE.md): module ownership checklists for agents
- [../SETTINGS_AND_PERSISTENCE.md](../SETTINGS_AND_PERSISTENCE.md): draft and save rules for the settings modal. A modal is a dialog on top of the page.
- [../FRONTEND_DESIGN_SYSTEM.md](../FRONTEND_DESIGN_SYSTEM.md): visual tokens and styling. A visual token is a named style value.
- [../../ARCHITECTURE.md](../../ARCHITECTURE.md): system overview. This is the main source.
- [../../ARCHITECTURE_GUIDE.md](../../ARCHITECTURE_GUIDE.md): contributor map with line references
- [../../AGENTS.md](../../AGENTS.md): dev commands and rules for generated code

## GitNexus tips while reading

CLI means command-line interface. Run these commands from the repo root. They search the code index, not only these docs.

```bash
node .gitnexus/run.cjs query "your concept"     # find execution flows
node .gitnexus/run.cjs context "SymbolName" -f path/to/file.go  # callers/callees
node .gitnexus/run.cjs status                   # check index freshness
```

The first command finds execution flows. A flow is one path through the code. The second command lists callers and callees. A caller is code that uses the symbol. A callee is code the symbol uses. The third command checks whether the index is up to date.

Every doc in this series follows this path:

```
Provider SSE → comet-sdk events → CometMind events → HTTP/SSE → renderer reducer → ChatItem rows
```

Keep every step in this path. That is how the system stays consistent.
