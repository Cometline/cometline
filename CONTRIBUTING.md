# Contributing to Cometline

Thanks for helping. This page covers setup, the daily workflow, and what a pull request needs before review.

## Reading path

1. [README.md](./README.md): what Cometline is and how to install it.
2. This file: setup, checks, and the PR checklist.
3. [docs/learning/](./docs/learning/00-README.md): a guided tour of the three modules.
4. [ARCHITECTURE_GUIDE.md](./ARCHITECTURE_GUIDE.md): the system overview and the package and folder map.

[AGENTS.md](./AGENTS.md) holds the command reference and repository rules for AI agents and humans alike. `CLAUDE.md` is a symlink to it.

## Repository layout

| Module | What it is |
| --- | --- |
| `comet-sdk/` | Go library for provider-agnostic LLM I/O |
| `cometmind/` | Go agent runtime: agent loop, SQLite, HTTP/SSE API, Discord gateway |
| `cometline/` | SvelteKit + Electron desktop shell |

Dependencies point one way: `cometline` → `cometmind` → `comet-sdk`. There is no root `go.work`, so run Go commands from `comet-sdk/` or `cometmind/`, not from the repository root.

## Prerequisites

- macOS 13+ to run and package the desktop app. The Go modules and frontend checks also run on Linux (CI uses Ubuntu).
- Go 1.25, Node.js 22, and pnpm 11.3.0. With [mise](https://mise.jdx.dev), `mise install` picks these up from `mise.toml`.

## Setup

```bash
git clone https://github.com/Cometline/cometline.git
cd cometline
make install   # pnpm install in cometline/
make dev       # build the CometMind sidecar and launch the Electron dev app
```

Runtime settings live in `~/.cometmind/cometline-settings.json`. The desktop app creates the file on first launch, and you configure providers in Settings.

## Checks

`make check` is what CI runs. It must pass before a PR is merged.

```bash
make check                        # codegen freshness, SDK + CometMind tests, Svelte check/lint/test
cd comet-sdk && go test ./...     # SDK only
cd cometmind && go test ./...     # runtime only
cd cometline && pnpm run check && pnpm run lint && pnpm run test
```

SDK live tests call real providers and need API keys. They are behind the `live` build tag and never run in CI (see [AGENTS.md](./AGENTS.md#live-tests)).

## Generated code

Never edit generated files by hand. Change the source and regenerate.

| Source | Generated output | Command |
| --- | --- | --- |
| `cometmind/openapi.yaml` | `cometline/src/lib/generated/cometmind-api/`, `cometmind/internal/apigen/types.gen.go` | `make generate` |
| `cometmind/internal/db/schema.sql`, `queries/*.sql` | `cometmind/internal/db/*.sql.go`, `db.go`, `models.go` | `cd cometmind && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` |

sqlc is pinned to the version in the generated file headers. `go run` fetches it on demand, so you don't need to install it. sqlc 1.31.1 needs Go 1.26, and Go downloads that toolchain automatically unless you set `GOTOOLCHAIN=local`.

Schema changes for existing users also need an incremental migration in `cometmind/internal/db/migrate.go`.

## Commits

Use conventional commits with the module or package as the scope, and sign off with `git commit -s`:

```text
feat(cometline): add default model picker to settings
fix(cometmind): prevent tool execution path escape
docs(readme): document setup steps
```

Use `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `chore`, or `ci` as the type. The release notes (`cliff.toml`) group `feat`, `fix`, `perf`, and `refactor` and skip the rest. Keep each commit and each PR to one concern. A refactor should not change behavior; move code first, then change it in a follow-up.

## Pull requests

Open one PR in this repository, even when it spans several modules. The [PR template](./.github/PULL_REQUEST_TEMPLATE.md) asks you to confirm:

- `make check` passes.
- Docs that describe the changed behavior are updated in the same PR.
- `ARCHITECTURE_GUIDE.md` is updated if you added, moved, or renamed a package or feature folder.
- Generated code was regenerated, not hand-edited.
- Contract changes (OpenAPI, SSE events, Electron IPC) are updated on both sides, with tests.

## Reporting bugs and security issues

File bugs and feature requests with the [issue templates](https://github.com/Cometline/cometline/issues/new/choose). Report vulnerabilities privately as described in [SECURITY.md](./SECURITY.md), not in a public issue.

Everyone taking part is expected to follow the [Code of Conduct](./CODE_OF_CONDUCT.md).
