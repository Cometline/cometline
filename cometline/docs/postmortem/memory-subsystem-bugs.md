# Memory subsystem bugs (embedding settings not persisting + extract wrong provider)

**Date:** 2026-06-16  
**Components:** `cometline/src/lib/components/SettingsMemoryPanel.svelte`, `cometline/src/lib/components/SettingsPanel.svelte`, `cometline/src/lib/embedding-models.ts`, `cometline/electron/main.cjs`, `cometmind/server/server.go`, `cometmind/server/memory_handlers.go`, `cometmind/internal/agent/runner.go`, `cometmind/internal/memory/{service,extractor,updater}.go`, `cometmind/internal/runtime/runtime.go`, `cometmind/internal/event/event.go`, `cometline/src/lib/reducers/chat.ts`, `cometline/src/lib/components/ChatThread.svelte`

## Symptom A: Embedding model resets after save

In **Settings → Memory**, the user selected an embedding model (e.g. `OpenAI Compatible · text-embedding-3-small`), clicked save, and reopened the panel — but the dropdown showed **Select embedding model…** again.

Memory **write/search could still work** (manual add, auto-retrieve, compaction), which made the bug confusing: runtime memory was alive, but the UI looked unconfigured.

In dev (`make dev`, renderer on `http://127.0.0.1:5173`), saving sometimes surfaced **Failed to fetch** in the settings footer due to CORS blocking PUT.

## Symptom B: Auto-summarize uses wrong provider after successful reply

After a **successful** assistant reply (e.g. OpenCode-go / `qwen3.7-plus`), a red **Error** card appeared in the thread:

```text
cometsdk: openai: server error (HTTP 400): /chat/completions:
Invalid model name passed in model=qwen3.7-plus.
```

The main chat was fine; only a **background memory step** had failed. With **Auto summarize** enabled, this could happen on **every completed turn** when the session provider differed from the runtime default provider.

## Root causes

### A1. UI reconciliation was too strict

The dropdown bound to `selectedEmbeddingKey`, which had to exactly match `${providerId}:${model}` in **currently enabled** provider models. Saved values missing `provider_id`, not in `enabledModels`, or only present in one store were dropped.

Memory could still run because CometMind `resolveMemoryEmbedding()` falls back to provider config when `memory.embedding` is empty.

### A2. Split persistence and split Save buttons

Saving embedding touched three places (CometMind API, local JSON, config.toml). The footer **Save** button did not call `PUT /memory/settings`; only a separate Memory-panel **Save settings** button did.

### A3. CORS blocked PUT in dev

CometMind `localCORS()` allowed `GET, POST, PATCH, DELETE, OPTIONS` but not **PUT**. Memory settings use `PUT /api/v1/memory/settings`.

### B1. Main chat vs memory extract use different providers

| Step                                 | Provider                                 | Model                             |
| ------------------------------------ | ---------------------------------------- | --------------------------------- |
| Main turn (`Runner.Run`)             | `ProviderForSession(sess)`               | Session model                     |
| Memory extract (`memory.NewService`) | `provider.New(cfg)` — **config default** | Same session model name passed in |

`runtime.New` builds the memory service once at startup with the default provider. After each turn, `emitMemoryExtract` called `ExtractAfterTurn` with `turn.ModelID` but the service's embedded provider was still the default.

### B2. Updater merge path had a second model mismatch

When extract found a similar existing memory, `updater.decide` used `u.model()`, which fell back to **`claude-sonnet-4-5`** when `extraction_model` was unset.

### B3. Background failure surfaced as a chat error

`emitMemoryExtract` pushed `event.Errorf(...)` on extract failure. The turn had already completed, so the error card was misleading.

## Fix

### A: Embedding settings persistence

1. **Harden embedding resolution (`embedding-models.ts`)** — `mergeEmbeddingFields()`, `resolveEmbeddingSelection()` with orphan fallback, `buildEmbeddingDropdownOptions()`.
2. **Unify Save UX** — Removed Memory-panel **Save settings** button; footer **Save** calls `memoryPanel.saveMemorySettings()`.
3. **Keep providers in sync on save** — `persistMemoryEmbedding()` adds the chosen embedding model to `enabledModels` before writing local settings.
4. **Allow PUT in CometMind CORS** — Added `PUT` to `Access-Control-Allow-Methods`.

### B: Extract wrong provider

1. **Pass the session provider into extract** — `Runner` threads `ProviderForSession` through `ExtractAfterTurn`.
2. **Use one extraction model for extract and update** — Resolve model once per extract pass; removed hardcoded `claude-sonnet-4-5` fallback.
3. **Treat extract as best-effort in the UI stream** — On extract error, return silently; do not emit `event.Error`.
4. **Surface successful writes subtly** — Emit `memory_updated` SSE and show faint **Memory saved** hint next to Copy.

## How to avoid regressions

### Embedding settings

- Any new CometMind endpoint used from the **Vite dev renderer** must be allowed in `localCORS()` — especially non-GET methods.
- Memory UI must not assume `GET /memory/settings` is the only source of truth; reconcile with local settings and show orphan options when enabled providers drift.
- Do not reintroduce a second save path for Memory-only fields without wiring the footer **Save** to the same flow.
- **Memory working ≠ embedding saved in UI** — provider fallback can embed without explicit `[memory.embedding]` in config.

### Extract provider

- Any LLM call that runs **in the runner** for a session turn must use **`ProviderForSession`**, not `provider.New(cfg)` or the memory service's startup provider.
- When adding async/post-turn hooks, decide explicitly: **fatal to the turn** vs **best-effort**. Memory extract/compaction should stay best-effort.
- If you see **HTTP 400 invalid model** only after the assistant bubble, check memory extract first — not the main chat provider.
- Test matrix: session on **non-default** provider + Auto summarize on + empty `extraction_model`.

## Structural fix: JSON SSOT (ISSUE #1)

Follow-up work consolidated settings into a single schema-owned module:

- [`src/lib/settings/schema.ts`](../../src/lib/settings/schema.ts) — defaults, `normalizeSettings`, `validateSettings`, `runtimeSlice`
- Electron bundles schema to [`electron/settings-schema.cjs`](../../electron/settings-schema.cjs); `writeCometMindConfig()` removed
- CometMind [`internal/config/cometline_settings.go`](../../../../cometmind/internal/config/cometline_settings.go) reads `cometline-settings.json` directly
- [`src/lib/settings/persist.ts`](../../src/lib/settings/persist.ts) — footer Save runs JSON write + `PUT /memory/settings` + sidecar restart in one flow

## Verification

### Embedding settings

1. Dev: `make dev` — open Settings → Memory, select embedding, footer **Save** — no CORS error in console.
2. Close and reopen Settings → Memory — dropdown shows saved model.
3. Quit and relaunch Cometline — selection persists.
4. `cd cometmind && go test -run TestLocalCORS ./server`

### Extract provider

1. Session on OpenCode-go with model `qwen3.7-plus`, Auto summarize on → send `hi` → assistant replies, **no red error card**.
2. Send a preference → hover reply → faint **Memory saved** (if LLM chose to persist).
3. Turn off Auto summarize → no post-turn extract LLM call.

## Relation to other postmortems

- [fetch-models-data-clone-error.md](./fetch-models-data-clone-error.md) — provider configuration in Settings; embedding models must be enabled there before they appear in the Memory dropdown.
