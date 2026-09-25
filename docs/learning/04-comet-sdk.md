# 04 - comet-sdk (LLM I/O Library)

> **Prerequisite:** [03-data-flows.md](./03-data-flows.md)  
> **Next:** [05-cometmind-runtime.md](./05-cometmind-runtime.md)

Prerequisite means read that page first.

## Purpose

`comet-sdk` is the **provider-normalized LLM I/O boundary**. **LLM** means large language model. **I/O** means input and output. **Provider-normalized** means every provider looks the same to the caller. A **boundary** means this library does only LLM input and output.

It provides one `Provider` interface and one event vocabulary. A **vocabulary** here is the shared set of event names. CometMind does not need to know the backend. The backend may be Anthropic Messages, OpenAI Chat Completions, ChatGPT Codex, or xAI Grok.

On purpose, it does **not** own agent loops, tool execution, sessions, persistence, or UI. **Persistence** means saved data.

## The Provider interface

```go
type Provider interface {
    ID() string
    Stream(ctx context.Context, req *Request) (<-chan Event, error)
}
```

Every provider returns one of two results:

- A **pre-stream error** (auth, rate limit, or bad request). **Pre-stream** means the error happens before events start. **Auth** means the key or sign-in was rejected. A **rate limit** means too many requests in a short time.
- A **channel of Events** that closes on `DoneEvent`, `ErrorEvent`, or EOF. A **channel** is a Go queue of events. **EOF** means end of file. The stream has ended.

## Core types

| Type | Role |
|------|------|
| `Request` | Messages, tools, system prompt, max tokens, temperature, provider options |
| `Message` | One turn: role, content blocks, and optional reasoning |
| `Block` variants | `TextBlock`, `ReasoningBlock`, `ToolCallBlock`, `ToolResultBlock` |
| `Tool` | JSON-schema tool definition for the model |
| `Event` variants | `TextDeltaEvent`, `ReasoningDeltaEvent`, `ToolCallDeltaEvent`, `StepFinishEvent`, … |
| `TokenUsage` | Input, output, and cache token counts per step |
| `ProviderConfig` | Base URL, HTTP client, timeout, retry count, auth |

A **variant** is one kind of that type.

`Request.MaxTokens` is the size limit for one model step. A **token** is a small piece of text that the model counts. CometMind sets this limit to `min(current model output limit, 32,000)` before it calls the provider. `min` means the smaller number. The SDK does not choose that number. `0` means "use the provider default" only when the caller leaves it empty.

Finish reasons are normalized by `NormalizeFinishReason` to `stop`, `tool_use`, `max_tokens`, and `error`. **Normalized** means every provider uses these same words.

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

In that tree, **exponential backoff** means each retry waits longer than the last. **SSE** means server-sent events. A **frame scanner** reads one SSE event at a time. A **subscription session** is a saved sign-in, not an API key. **Borrowed** means that sign-in comes from an existing login.

## StreamMessage - the API CometMind uses

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

A **goroutine** is a Go task that runs in the background. **Substantive** means the events that carry text, reasoning, or tool calls. **Suppress** means those two end events are not sent on the public channel. **Drain** means read every event until the channel closes.

Source: `comet-sdk/llm/stream.go`

GitNexus shows `Runner.Run` as the only production caller of `StreamMessage`. **Production** means the running app, not tests.

### Invariants

An **invariant** is a rule that must stay true.

| Rule | If the rule is broken |
|------|------------------------|
| Drain `Events()` before `Result()` | Deadlock |
| Providers close channel on terminal event | Goroutine leak |
| `Request.Options` can't override SDK-managed fields | Broken payloads |
| OpenAI usage may arrive after finish reason | Lost token counts |
| Anthropic tool call IDs sanitized before wire | API rejection |

A **deadlock** means both sides wait, and neither finishes. A **terminal event** is the last event, such as done or error. A **goroutine leak** means that background task never stops. A **payload** is the request body. **SDK-managed** fields are fields the SDK sets itself. **Sanitized** means unsafe characters are removed. The **wire** is the data sent to the provider.

## Provider implementation pattern

Each provider follows the same steps:

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

- Uses the native Messages API. **Native** means Anthropic's own API.
- Builds content blocks from streaming deltas. A **delta** is one small update in the stream.
- Stores cache token usage in `TokenUsage`
- Sanitizes tool call IDs before they are sent
- Retries on 529 (overloaded). **Overloaded** means the provider is too busy.

### OpenAI (`provider/openai/`)

- Uses a Chat Completions-compatible format. **Compatible** means the request has the same shape.
- Sends `stream_options.include_usage` so the response includes token counts
- Builds tool calls by index from parallel deltas. **Parallel** means several updates arrive together.
- Supports `reasoning_content` as another name for reasoning text on reasoning models. An **alias** is another name for the same field.

### Codex (`provider/codex/`)

- Uses ChatGPT Codex-specific auth and endpoints
- Loads the subscription OAuth session from `~/.codex/auth.json` (or `$CODEX_HOME/auth.json`). **OAuth** is a sign-in flow that saves a session.
- Does not store an API key in Cometline settings. Electron or the Codex CLI owns sign-in.

### OpenAI Responses (`provider/openairesponses/`)

- This is the API-key Responses provider used by OpenCode Go models (`@ai-sdk/openai` metadata)
- Sends `store: false` and `include: ["reasoning.encrypted_content"]` for stateless encrypted reasoning replay. **Stateless** means the provider does not store the turn. **Replay** means a later step sends the saved reasoning again.
- Uses the same capability fallbacks as Codex (`max_output_tokens`, reasoning summary, encrypted replay). A **capability** is one feature of that model call. A **fallback** is the second choice when the first one is not used.

Codex and OpenCode Go share one wire protocol. It lives in `internal/responsesproto`. That package converts requests. It parses SSE events: `response.completed`, `response.incomplete`, and `response.failed`. It captures encrypted reasoning state. It also classifies errors.

### xAI (`provider/xai/`)

- Uses Grok subscription auth, or a borrowed session
- Stores the session token at `~/.cometmind/xai/auth.json`
- Is connected through `cometmind/internal/provider/factory.go`, like the other methods

## API-key vs subscription providers

| Kind | Methods | Auth |
|------|---------|------|
| API key | `anthropic`, `openai`, `openai-compatible`, `opencode-go` | Key in settings / env (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, …) |
| Subscription session | `codex`, `xai` | Local auth JSON; Electron IPC for sign-in / discovery |

**IPC** means messages between Electron processes. **Discovery** means finding a sign-in that is already on the machine.

## Error handling

Typed errors live in `errors.go`. A **typed error** has its own Go type, not only a text string.

| Error | When |
|-------|------|
| `AuthError` | 401/403, invalid API key |
| `RateLimitError` | 429, includes `RetryAfter` |
| `ServerError` | 5xx from provider |
| `StreamError` | Mid-stream parse or connection failure |

401 and 403 mean the server rejected the sign-in. 429 means too many requests. `RetryAfter` is the wait time included with that error. 5xx means the provider server failed. **Mid-stream** means the failure happens after events have started.

`internal/providerbase` classifies HTTP responses in the same way for every provider.

## Retry behavior

`internal/retry` handles **pre-stream** HTTP failures only (429, 5xx, Anthropic 529). Once the SSE body starts, errors become `StreamError`. There is no retry in the middle of a stream.

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

**CI-safe** means `make test` does not need a live API key.

Fixtures under `provider/*/fixtures/` are checked-in SSE snapshots. A **fixture** is a saved sample used by tests. Update the fixtures when parser behavior changes.

## Adding a new provider

1. Create `provider/<name>/client.go`, `convert.go`, `stream.go`
2. Implement `cometsdk.Provider`
3. Use `internal/sse` for SSE parsing
4. Use `internal/retry` and `internal/providerbase`
5. Emit only canonical SDK event types. **Canonical** means the shared event names, not provider-specific names.
6. Add fixtures and unit tests
7. Register the provider in `cometmind/internal/provider/factory.go`
8. Add provider defaults and validation in `cometline/src/lib/settings/schema.ts`
9. Add UI in `SettingsProvidersPanel.svelte`. If the provider uses a subscription, also add Electron auth helpers.

## Mental model

`comet-sdk` converts provider wire format into SDK events while the reply is still arriving:

```
Provider wire format  →  cometsdk.Event channel  →  assembled Message + TokenUsage
```

CometMind only sees the middle and the right side of that diagram. The left side stays inside this library. **Encapsulated** means those wire details stay behind the `Provider` interface.

## What's next

[05-cometmind-runtime.md](./05-cometmind-runtime.md) shows how CometMind uses `StreamMessage` and runs the agent loop.
