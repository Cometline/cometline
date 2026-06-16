# Memory auto-summarize uses wrong provider after successful reply

**Date:** 2026-06-16  
**Components:** `cometmind/internal/agent/runner.go`, `cometmind/internal/memory/{service,extractor,updater}.go`, `cometmind/internal/runtime/runtime.go`, `cometmind/internal/event/event.go`, `cometline/src/lib/reducers/chat.ts`, `cometline/src/lib/components/ChatThread.svelte`

## Symptom

After a **successful** assistant reply (e.g. OpenCode-go / `qwen3.7-plus`), a red
**Error** card appeared in the thread:

```text
cometsdk: openai: server error (HTTP 400): /chat/completions:
Invalid model name passed in model=qwen3.7-plus.
Call '/v1/models' to view available models for your key.
```

The main chat was fine; the user could keep talking. The failure looked like the
turn broke, but only a **background memory step** had failed.

With **Auto summarize** enabled, this could happen on **every completed turn**
when the session provider differed from the runtime default provider.

## Root cause

Two separate agent loops share one user-visible stream, but memory was wired to
the wrong backend:

### 1. Main chat vs memory extract use different providers

| Step | Provider | Model |
| ---- | -------- | ----- |
| Main turn (`Runner.Run`) | `ProviderForSession(sess)` — e.g. OpenCode-go | Session model, e.g. `qwen3.7-plus` |
| Memory extract (`memory.NewService`) | `provider.New(cfg)` — **config default / active provider** | Same session model name passed in |

`runtime.New` builds the memory service once at startup:

```go
p, err := provider.New(cfg) // default provider id, not session provider
mem, err := memory.NewService(sqlDB, cfg.MemorySettings(), p, sessions)
```

After each turn, `emitMemoryExtract` called `ExtractAfterTurn` with
`turn.ModelID` but the service's embedded `cometsdk.Provider` was still the
default. That provider's base URL / key does not accept `qwen3.7-plus` → HTTP
400.

### 2. Updater merge path had a second model mismatch

When extract found a similar existing memory (similarity ≥ 0.80), `updater.decide`
used `u.model()`, which fell back to **`claude-sonnet-4-5`** when
`extraction_model` was unset — not the session model. Even after fixing the
provider, merge/supersede could still call the wrong model.

### 3. Background failure surfaced as a chat error

`emitMemoryExtract` pushed `event.Errorf(..., "memory")` on extract failure.
The turn had already completed and the assistant bubble was shown, so the error
card was misleading.

### 4. Extract runs once per completed turn (not only when something is saved)

When **Auto summarize** (`auto_extract`) is on, `emitMemoryExtract` runs after
every agent turn finish (stop / no more tool calls). An LLM **review** call
always runs; **persisting** memory still depends on `should_save`, confidence,
and similarity gates. See Settings → Memory → Auto summarize.

## Fix process

### 1. Pass the session provider into extract

`Runner` already holds the session-scoped provider (`ProviderForSession`). Thread
it through extract:

```go
// runner.go
changes, err := r.Memory.ExtractAfterTurn(ctx, turn.ID, turn.ModelID, r.Provider)
```

```go
// service.go
func (s *Service) ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]Change, error)
```

`extractor.extractAfterTurn` and `updater.decide` call `llm.GenerateJSON` with
that provider instead of the service default.

### 2. Use one extraction model for extract and update

Resolve model once per extract pass:

```go
useModel := strings.TrimSpace(e.settings.ExtractionModel)
if useModel == "" {
    useModel = model // session model from the turn
}
```

Pass `useModel` into `updater.handleSimilar` / `decide` so merge and supersede
use the same model as the initial extract JSON call. Removed the hardcoded
`claude-sonnet-4-5` fallback on the update path.

### 3. Treat extract as best-effort in the UI stream

On extract error, return silently from `emitMemoryExtract` — do **not** emit
`event.Error`. The user already got a successful reply; memory is auxiliary.

### 4. Surface successful writes subtly (same workstream)

When extract **does** persist memory, emit `memory_updated` SSE before `done`
and show a faint **Memory saved** / **Memory updated** hint next to Copy (hover
for tooltip). This is separate from the provider bug but part of the same memory
MVP UX pass.

## Related memory settings (same release)

- **Embedding model** in Settings → Memory is derived from enabled provider
  models (name contains `embed`); not a separate provider block.
- **Auto retrieve** injects memories at turn start (`memory_injected` → Thinking
  fold); **Auto summarize** reviews at turn end.

## How to avoid regressions

- Any LLM call that runs **in the runner** for a session turn must use
  **`ProviderForSession`**, not `provider.New(cfg)` or the memory service's
  startup provider.
- When adding async/post-turn hooks, decide explicitly: **fatal to the turn** vs
  **best-effort**. Memory extract/compaction should stay best-effort unless the
  product requires hard failure.
- If you see **HTTP 400 invalid model** only after the assistant bubble, check
  memory extract first — not the main chat provider.
- Test matrix: session on **non-default** provider (OpenCode-go, custom
  openai-compatible) + Auto summarize on + empty `extraction_model`.

## Verification

1. Session on OpenCode-go with model `qwen3.7-plus`, Auto summarize on → send
   `hi` → assistant replies, **no red error card**.
2. Send a preference (`Use Traditional Chinese`) → hover reply → faint
   **Memory saved** (if LLM chose to persist) with tooltip content.
3. Session on default OpenAI provider → extract still works (regression).
4. Turn off Auto summarize → no post-turn extract LLM call (no memory hint).

## Relation to other postmortems

- [fetch-models-data-clone-error.md](./fetch-models-data-clone-error.md) —
  provider configuration in Settings; wrong base URL / model list is a separate
  class of provider bugs.
- CometMind architecture: `AGENTS.md` — memory service is global on `Runtime`;
  runner must pass per-session provider into extract until memory service gets a
  factory hook.
