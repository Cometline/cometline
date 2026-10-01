# Conventions

One way to do each thing. This page is the short form. Controller, error, and testing details stay in [cometline/docs/FRONTEND_PATTERNS.md](../cometline/docs/FRONTEND_PATTERNS.md). Colors, type, and motion stay in [cometline/STYLING.md](../cometline/STYLING.md) and [FRONTEND_DESIGN_SYSTEM.md](./FRONTEND_DESIGN_SYSTEM.md).

## Go

Modules are `github.com/Cometline/cometline/comet-sdk` and `github.com/Cometline/cometline/cometmind`, on Go 1.26. There is no root `go.work`. The HTTP server is `cometmind/internal/server`. SQLite is opened in `cometmind/internal/sqlite`.

Files stay around 500 lines and functions around 80 lines (`funlen`). Cyclomatic complexity stays at or under 15 (`gocyclo`). `funlen` and `gocyclo` still have named exceptions in [scripts/readability-allowlist.txt](../scripts/readability-allowlist.txt), so they remain in the non-blocking `.golangci.budget.yml` configs. `scripts/readability-report.sh --check` fails when a new offender is not listed, and when a listed function is back under budget. Size and fan-out budgets have no exceptions: Go files over 500 lines, Go functions over 80 body lines, Svelte components over 400, scoped CSS over 200, `.svelte.ts` stores over 500, and cometmind packages over 10 internal imports outside the composition roots (`cmd`, `internal/server`, `internal/runtime`, `internal/tools`).

`internal/tools` is the tool-registry composition root. It may exceed the fan-out budget. depguard does not exempt it, and depguard does not exempt `cmd/session.go` or `internal/session/wire.go`.

Put interfaces next to the code that calls them, not next to the implementation.

Wrap errors with `fmt.Errorf("context: %w", err)` so callers can use `errors.Is` and `errors.As`. Sentinels are package-level `var Err... = errors.New(...)`.

### HTTP handlers

Routes come from the generated Gin strict server in `cometmind/internal/apigen` (`server.gen.go`). Add an operation to `openapi.yaml`, implement the strict-server method, and run `make generate`.

These operations stay hand-registered in `internal/server/routes.go` because the strict generator cannot express them: `postSessionMessage`, `streamSessionEvents`, `streamRuntimeEvents`, `getSessionMedia`, `getMediaContent`, `exportSkill`.

The spec documents two error shapes, and both remain: `{error:{code,message}}` (`ErrorResponse`) and `{"error":"string"}` (`SimpleErrorResponse`).

`internal/apigen` is imported only by `internal/server`. `internal/db` is not imported by `internal/server` or `internal/gateway`.

### Migrations

A fresh database is built from `cometmind/internal/db/schema.sql`. Upgrades are embedded files `cometmind/internal/db/migrations/NNNN_description.sql` (0002 through 0037). The highest file number is the current version. `TestMigrationsFromV1MatchFreshSchema` fails if `schema.sql` and the migrations drift. A file that contains `DROP TABLE` runs as a transactional rebuild. The only Go-side step is `skipIfApplied` in `internal/db/migrate.go`.

### Tools

`internal/tools` is the registry. Families live in `fsops`, `web`, `media`, `jobs`, `mcp`, `memory`, `settings`, `subagent`, `skills`, and `inbox`, plus shared `toolkit` and `fs`. A family must not import the parent `tools` package. Implement `toolkit.Tool` in the family and register it from `registry.go`.

## Frontend

Feature code lives in `cometline/src/lib/features/{chat,composer,gallery,inbox,jobs,onboarding,settings,shell,sidebar,skills,usage,workspace}`. `src/lib/components/` is shared primitives only.

Components stay at or under 400 lines, scoped CSS at or under 200, and `.svelte.ts` stores at or under 500. Generated code is excluded. The same readability check enforces this.

The renderer calls CometMind only through `$lib/client`. Electron IPC channel names live only in `cometline/electron/src/shared/ipc-channels.ts`. Colors, spacing, and motion use design tokens (`var(--*)` in `app.css`); do not add raw hex in components.
