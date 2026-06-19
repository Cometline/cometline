# OpenAPI-driven Go ↔ TypeScript API contract codegen (ISSUE-5)

**Date:** 2026-06-16  
**Components:** `cometmind/openapi.yaml`, `cometmind/internal/event/event.go`, `cometmind/internal/apigen/`, `cometmind/internal/contract/`, `cometline/src/lib/generated/cometmind-api/`, `cometline/src/lib/client/cometmind.ts`, `cometline/src/lib/types.ts`, root `Makefile`, `AGENTS.md`

## Problem

Cometline streams chat over HTTP SSE (`POST /api/v1/sessions/{id}/message`), not Electron IPC. Each `data:` frame is JSON discriminated by `type` (`text_delta`, `tool_call`, `memory_injected`, …).

Before this work the **wire contract** lived in four hand-maintained places:

| Layer | File | Role |
| ----- | ---- | ---- |
| Go emitter | `cometmind/internal/event/event.go` | `MarshalJSON` defines on-the-wire SSE JSON |
| REST spec | `cometmind/openapi.yaml` | REST only; SSE was `type: string` + prose examples |
| TypeScript | `cometline/src/lib/types.ts` | Hand-written `StreamEvent` union |
| Client | `cometline/src/lib/client/cometmind.ts` | Hand-written `fetch` + `normalizeMemorySettings()` PascalCase shims |

Drift was easy: new `subagent_*` or `memory_*` events required coordinated edits across Go, TS types, reducer, and tests. Memory settings responses also needed runtime alias shims because the client did not trust snake_case from the API.

## What we built

`cometmind/openapi.yaml` is now the **single schema source of truth** for REST JSON and SSE frame shapes. Consumers are generated; CI fails if artifacts are stale.

```text
openapi.yaml
    ├── @hey-api/openapi-ts  →  cometline/src/lib/generated/cometmind-api/
    ├── oapi-codegen         →  cometmind/internal/apigen/types.gen.go
    └── kin-openapi (tests)  →  cometmind/internal/contract/
```

### TypeScript (Cometline)

- **`pnpm run generate:api`** (also `make generate`) emits typed client + types under `src/lib/generated/cometmind-api/`.
- **`cometmind.ts`** delegates REST to the generated SDK; keeps thin **SSE wrappers** (`streamMessage`, `respondToSubagent`) because `text/event-stream` cannot use the JSON fetch client.
- **`types.ts`** re-exports API types (`StreamEvent`, `Session`, …); UI-only types (`ChatItem`, `ProviderSettings`) stay hand-written.
- **`normalizeMemorySettings()` removed**; `resolveMemorySettings()` only fills optional API fields with UI defaults (no PascalCase branches).

### Go (CometMind)

- **`go generate ./internal/apigen`** produces REST DTO types only (no Gin stubs).
- **`internal/event/event.go` unchanged** as the runtime emitter — the unified `Event` struct + custom `MarshalJSON` stays load-bearing for the agent loop.
- **`internal/contract`** validates every `event.MarshalJSON` output and server SSE frames against the OpenAPI `StreamEvent` schema.
- **`handlePutMemorySettings`** now returns `EffectiveMemoryConfig()` so PUT matches GET wire JSON.

### Tooling

| Command | Effect |
| ------- | ------ |
| `make generate` | Regen TS client + Go `apigen` types |
| `make check` | Runs `check-generated` (git diff gate) + all tests |

`predev:electron` / `prebuild:sidecar` run `generate:api` so local dev stays aligned.

## What stayed hand-written (on purpose)

- `event.Event` + `MarshalJSON` in Go
- SSE streaming loops in `cometmind.ts`
- Chat reducer semantics in `reducers/chat.ts` (only the **type** is codegen'd)
- Electron IPC and Cometline settings schemas

Not done yet (optional follow-ups): runtime SSE validation in `parser.ts` (e.g. `ajv`), adopting `apigen` types inside HTTP handlers.

---

## Mindset: changing the wire contract

### Add a new SSE event type

1. **`cometmind/openapi.yaml`** — add per-kind schema + `StreamEvent` `oneOf` / discriminator entry. Field names must match `MarshalJSON` exactly (`text_delta` uses `delta`, not `text`; `subagent_awaiting_input` uses `kind` for awaiting kind).
2. **`cometmind/internal/event/event.go`** — add `Kind`, fields, `MarshalJSON` branch, constructor.
3. **`make generate`** — commit `cometline/src/lib/generated/cometmind-api/` and `cometmind/internal/apigen/types.gen.go`.
4. **`cometline/src/lib/reducers/chat.ts`** — handle the new `type` in reducer logic.
5. **`cometmind/internal/contract/contract_test.go`** — add a table row (constructor → marshal → schema validate). Server SSE tests can rely on `contract.ValidateStreamEventJSON` for integration coverage.

If OpenAPI and Go diverge, `go test ./internal/contract/...` or `TestPostMessageStreamsSSEAndPersistsUserTurn` fails.

### Add or change a REST endpoint

1. Extend **`openapi.yaml`** paths + `components/schemas`.
2. **`make generate`**.
3. Wire Cometline via generated SDK in **`cometmind.ts`** (or re-export if the generated name differs).
4. Implement handler in **`cometmind/server/`**; add server test.

### Memory settings

- Wire shape is `MemorySettings` in OpenAPI (snake_case).
- Cometline uses **`MemorySettings`** with required UI fields via `defaultMemorySettings()` / `resolveMemorySettings()` in `cometmind.ts`.
- Partial PUT bodies can reset unset fields to zero; send a full settings object when toggling `enabled` (see `memory_handlers_test.go`).

---

## Pitfalls we hit

### OpenAPI SSE examples vs schema

Multi-line `data: …` SSE **stream** examples cannot validate as a single `StreamEvent` object. kin-openapi `doc.Validate()` failed until those examples were removed from `text/event-stream` responses. Use per-event JSON in contract tests instead.

### oapi-codegen and OpenAPI 3.1

`oapi-codegen` warns that 3.1 is not fully supported; **types-only** generation works for our spec today. Watch [oapi-codegen#373](https://github.com/oapi-codegen/oapi-codegen/issues/373) if we hit edge cases.

### Generated `MemorySettings` is all-optional

OpenAPI marks most memory fields optional. The UI layer keeps a stricter `MemorySettings` type and `resolveMemorySettings()` so settings panels do not sprawl `?.` everywhere.

### `step_finish.usage` is required on the wire

Generated `StreamEvent` requires `usage` on `step_finish`. Reducer tests that used `{ type: 'step_finish' }` alone needed a `TokenUsage` payload.

### PUT memory settings vs `EffectiveMemoryConfig`

Returning raw `a.config.Memory` on PUT disagreed with GET. Aligning both to `EffectiveMemoryConfig()` surfaced that partial PUT + `memoryBehaviorConfigured()` can merge defaults back in — tests must send enough fields to express intent.

---

## Regression checklist

- [ ] `make check` passes (includes codegen freshness + contract tests)
- [ ] `cd cometline && pnpm run test`
- [ ] After `openapi.yaml` edits: `make generate` and commit generated files
- [ ] New SSE kind: OpenAPI + `event.go` + reducer + contract test row

## Related docs

- [`AGENTS.md`](../../../AGENTS.md) — generate workflow for agents
- [`ARCHITECTURE_GUIDE.md`](../../../ARCHITECTURE_GUIDE.md) — SSE event contract overview
- [`ISSUE.md`](../../../ISSUE.md) — original backlog write-up (ISSUE-5)
