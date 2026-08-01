# 04 — comet-sdk (LLM I/O Library)

> **Prerequisite:** [03-data-flows.md](./03-data-flows.md)  
> **Next:** [05-cometmind-runtime.md](./05-cometmind-runtime.md)

## Purpose

`comet-sdk` is the **provider-normalized LLM I/O boundary**. It exposes one `Provider` interface and one event vocabulary so CometMind never cares whether the backend is Anthropic Messages, OpenAI Chat Completions, ChatGPT Codex, or xAI Grok.

It deliberately does **not** own agent loops, tool execution, sessions, persistence, or UI.

## The Provider interface

```go
type Provider interface {
    ID() string
    Stream(ctx context.Context, req *Request) (<-chan Event, error)
}
```

Every provider returns either:
- A **pre-stream error** (auth, rate limit, bad request), or
- A **channel of Events** that closes on `DoneEvent`, `ErrorEvent`, or EOF

## Core types

| Type | Role |
|------|------|
| `Request` | Messages, tools, system prompt, max tokens, temperature, provider options |
| `Message` | One turn: role + content blocks + optional reasoning |
| `Block` variants | `TextBlock`, `ReasoningBlock`, `ToolCallBlock`, `ToolResultBlock` |
| `Tool` | JSON-schema tool definition for the model |
| `Event` variants | `TextDeltaEvent`, `ReasoningDeltaEvent`, `ToolCallDeltaEvent`, `StepFinishEvent`, … |
| `TokenUsage` | Input/output/cache token counts per step |
| `ProviderConfig` | Base URL, HTTP client, timeout, retry count, auth |

Finish reasons are normalized by `NormalizeFinishReason` to: `stop`, `tool_use`, `max_tokens`, `error`.

## Package layout

```text
comet-sdk/
├── sdk.go              Public API and types
├── errors.go           AuthError, RateLimitError, ServerError, StreamError
├── llm/
│   ├── stream.go       StreamMessage (primary CometMind entry)
│   ├── collect.go      Collect, GenerateText, GenerateMessage
│   └── *_test.go       Unit + live tests
├── provider/
│   ├── anthropic/      client.go, convert.go, stream.go, fixtures/
│   ├── openai/         client.go, convert.go, stream.go, fixtures/
│   ├── codex/          ChatGPT Codex adapter (subscription session)
│   └── xai/            xAI Grok adapter (borrowed subscription session)
└── internal/
    ├── providerbase/   Endpoint helpers, HTTP error classification
    ├── retry/          Exponential backoff for pre-stream failures
    └── sse/            SSE frame scanner
```

## StreamMessage — the API CometMind uses

`llm.StreamMessage(ctx, provider, req)` returns a `*MessageStream`:

```text
1. Start provider.Stream in background goroutine
2. MessageStream.run():
   - Forward substantive events to Events() channel
   - Suppress ErrorEvent/DoneEvent from public channel
   - Accumulate text, reasoning, tool calls, usage
3. Caller drains Events()
4. Caller calls Result() → final message + error
```

Source: `comet-sdk/llm/stream.go`

GitNexus shows `Runner.Run` as the sole production caller of `StreamMessage`.

### Invariants

| Rule | Consequence if violated |
|------|------------------------|
| Drain `Events()` before `Result()` | Deadlock |
| Providers close channel on terminal event | Goroutine leak |
| `Request.Options` can't override SDK-managed fields | Broken payloads |
| OpenAI usage may arrive after finish reason | Lost token counts |
| Anthropic tool call IDs sanitized before wire | API rejection |

## Provider implementation pattern

Each provider follows the same shape:

```text
constructor(config) → Provider
  Stream(ctx, req):
    convert SDK Request → wire JSON
    POST streaming endpoint (with retry for pre-stream)
    scan SSE body
    convert wire events → SDK Events
    emit DoneEvent or ErrorEvent
    close channel
```

### Anthropic (`provider/anthropic/`)

- Native Messages API
- Content block assembly from streaming deltas
- Cache token usage in `TokenUsage`
- Tool call ID sanitization
- Retry on 529 (overloaded)

### OpenAI (`provider/openai/`)

- Chat Completions-compatible format
- `stream_options.include_usage` for token counts
- Tool-call index assembly from parallel deltas
- `reasoning_content` alias support for reasoning models

### Codex (`provider/codex/`)

- ChatGPT Codex-specific auth and endpoints
- Subscription OAuth session from `~/.codex/auth.json` (or `$CODEX_HOME/auth.json`)
- No API key in Cometline settings — Electron/Codex CLI owns sign-in

### OpenAI Responses (`provider/openairesponses/`)

- API-key Responses provider used by OpenCode Go models (`@ai-sdk/openai` metadata)
- `store: false`, `include: ["reasoning.encrypted_content"]` for stateless encrypted reasoning replay
- Same capability fallbacks as Codex (`max_output_tokens`, reasoning summary, encrypted replay)

The wire protocol shared by Codex and OpenCode Go lives in `internal/responsesproto`: request conversion, SSE parser (`response.completed` / `response.incomplete` / `response.failed`), encrypted reasoning state capture, and error classification.

### xAI (`provider/xai/`)

- Grok subscription / borrowed session auth
- Session token stored at `~/.cometmind/xai/auth.json`
- Wired through `cometmind/internal/provider/factory.go` like other methods

## API-key vs subscription providers

| Kind | Methods | Auth |
|------|---------|------|
| API key | `anthropic`, `openai`, `openai-compatible`, `opencode-go` | Key in settings / env (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, …) |
| Subscription session | `codex`, `xai` | Local auth JSON; Electron IPC for sign-in / discovery |

## Error handling

Typed errors in `errors.go`:

| Error | When |
|-------|------|
| `AuthError` | 401/403, invalid API key |
| `RateLimitError` | 429, includes `RetryAfter` |
| `ServerError` | 5xx from provider |
| `StreamError` | Mid-stream parse or connection failure |

`internal/providerbase` classifies HTTP responses consistently across providers.

## Retry behavior

`internal/retry` handles **pre-stream** HTTP failures only (429, 5xx, Anthropic 529). Once the SSE body starts, errors surface as `StreamError` — no mid-stream retry.

## Testing strategy

| Test type | Location | Requires API key? |
|-----------|----------|-------------------|
| Unit/parser | `provider/*/stream_test.go`, `fixtures/` | No |
| StreamMessage | `llm/stream_test.go` | No |
| Live | `*_live_test.go` with `//go:build live` | Yes |

```bash
cd comet-sdk
make test          # CI-safe
make test-live     # Requires ANTHROPIC_API_KEY / OPENAI_API_KEY
```

Fixtures under `provider/*/fixtures/` are checked-in SSE snapshots. Update them when parser behavior changes.

## Adding a new provider

1. Create `provider/<name>/client.go`, `convert.go`, `stream.go`
2. Implement `cometsdk.Provider`
3. Use `internal/sse` for SSE parsing
4. Use `internal/retry` and `internal/providerbase`
5. Emit only canonical SDK event types
6. Add fixtures + unit tests
7. Wire in `cometmind/internal/provider/factory.go`
8. Add provider defaults/validation in `cometline/src/lib/settings/schema.ts`
9. Add UI in `SettingsProvidersPanel.svelte` (and Electron auth helpers if subscription-based)

## Mental model

Think of comet-sdk as a **streaming translator**:

```
Provider wire format  →  cometsdk.Event channel  →  assembled Message + TokenUsage
```

CometMind only sees the middle and right side. The left side is entirely encapsulated here.

## What's next

[05-cometmind-runtime.md](./05-cometmind-runtime.md) — how CometMind consumes `StreamMessage` and orchestrates the agent loop.