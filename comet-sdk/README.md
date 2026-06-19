# Comet SDK

A provider-agnostic Go LLM client library. One interface, any backend.

This directory is one module inside the `cometline-release` monorepo. The historical standalone `comet-sdk` repo is archived; current development, issues, and pull requests land in the monorepo root.

```
module: github.com/cometline/comet-sdk
go:     1.25
```

---

## Status

Comet SDK remains a reusable Go module boundary, but it is no longer developed as a separate repository or separately released package today. In practice, it is maintained monorepo-first for CometMind and Cometline.

The code is still intentionally shaped like a library rather than an internal dump: the public types, provider packages, and `llm` helpers remain useful if Comet SDK is spun back out or published independently again later.

---

## Role in the Cometline stack

Comet SDK is the **LLM I/O layer** of the Cometline stack. CometMind consumes it for uniform model access regardless of provider.

```
┌─────────────────────────────────────────────┐
│  cometline   Electron desktop shell         │  ← UI
├─────────────────────────────────────────────┤
│  cometmind   General AI agent runtime       │  ← brain
│              (agent loop, tools, memory,    │
│               ACP delegation, persistence)  │
├─────────────────────────────────────────────┤
│  comet-sdk   Provider-agnostic LLM I/O      │  ← this repo
└─────────────────────────────────────────────┘
```

Comet SDK deliberately stays a **pure LLM I/O library**: no agent loop, no tool execution, no persistence, no memory. CometMind owns orchestration; the SDK focuses on talking to models cleanly across providers.

---

## What it does

Comet SDK gives you a single `Provider` interface that works identically for Anthropic, OpenAI, ChatGPT Codex, and OpenAI-compatible endpoints (DeepSeek, company gateways, OpenCode Zen, etc.). It handles:

- Streaming responses over SSE
- Reasoning / chain-of-thought events (`ReasoningStartEvent`, `ReasoningContentEvent`)
- Multimodal user input (`ImageBlock` with base64 data URLs)
- Tool calling with full delta assembly
- Provider-specific message normalisation
- Automatic retry with exponential backoff (429, 5xx, Anthropic 529)
- Token usage tracking, including Anthropic prompt-cache fields
- Structured debug logging via `log/slog`

For most callers, the recommended public entry point is the `llm` package: `llm.GenerateMessage`, `llm.StreamMessage`, and `llm.GenerateJSON`.

Use `Provider.Stream()` directly when you need lower-level control over raw events.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         caller                              │
│                                                             │
│   llm.StreamMessage(ctx, provider, &Request{                 │
│       Model, Messages, Tools, System,                       │
│       MaxTokens, Options["openai"|"anthropic"]              │
│   })                                                        │
└──────────────────────────┬──────────────────────────────────┘
                           │  <-chan Event
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                   cometsdk  (sdk.go)                        │
│                                                             │
│  Provider · Request · Message · Block                       │
│  Event (TextDelta · Reasoning* · ToolCall* · StepFinish ·   │
│         Error · Done)                                       │
│  Tool · TokenUsage · ProviderConfig · Option                │
│                                                             │
│  errors.go                                                  │
│  AuthError · RateLimitError · ServerError · StreamError     │
└────────────────┬──────────────────────┬─────────────────────┘
                 │                      │
    ┌────────────▼────────┐  ┌──────────▼───────────┐  ┌──────────▼─────────┐
    │  provider/anthropic  │  │   provider/openai     │  │  provider/codex    │
    │                      │  │                       │  │                    │
    │  client.go           │  │  client.go            │  │  client.go         │
    │  convert.go          │  │  convert.go           │  │  convert.go        │
    │  stream.go           │  │  stream.go            │  │  stream.go         │
    │  fixtures/           │  │  reasoning.go         │  │  auth.go           │
    └────────┬─────────────┘  │  fixtures/            │  └─────────┬──────────┘
             │                └──────────┬─────────────┘            │
             └─────────────┬─────────────┴──────────────────────────┘
                           │
           ┌───────────────▼───────────────┐
           │          internal/            │
           │                               │
           │  sse/scanner.go               │
           │  retry/retry.go               │
           │  providerbase/providerbase.go │
           └───────────────────────────────┘
                        │
          ┌─────────────┼──────────────────┐
          ▼             ▼                  ▼
   Anthropic API   OpenAI API      OpenAI-compatible APIs     ChatGPT Codex
   /v1/messages    /v1/chat/       (DeepSeek, gateways, etc.) /responses
                   completions
```

---

## Using it

Inside this monorepo, CometMind uses a local replace:

```go
replace github.com/cometline/comet-sdk => ../comet-sdk
```

For now, treat this module as source that lives in the monorepo rather than as a separately published package with its own release flow.

---

## Quick start

### Recommended: `llm.GenerateMessage`

```go
import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    cometsdk "github.com/cometline/comet-sdk"
    "github.com/cometline/comet-sdk/llm"
    "github.com/cometline/comet-sdk/provider/anthropic"
)

p := anthropic.NewAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY"))

req := &cometsdk.Request{
    Model:  "claude-sonnet-4-5",
    System: "You are a helpful coding assistant.",
    Messages: []cometsdk.Message{
        {
            Role:    cometsdk.RoleUser,
            Content: []cometsdk.Block{cometsdk.TextBlock{Text: "Read main.go and tell me what it does."}},
        },
    },
    Tools: []cometsdk.Tool{
        {
            Name:        "read_file",
            Description: "Read the contents of a file",
            Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`),
        },
    },
    MaxTokens: 1024,
}

result, err := llm.GenerateMessage(context.Background(), p, req)
if err != nil {
    panic(err)
}

fmt.Println("finish reason:", result.FinishReason)
for _, tc := range result.ToolCalls {
    fmt.Printf("tool: %s input=%s\n", tc.Name, tc.Input)
}
```

### Recommended streaming: `llm.StreamMessage`

```go
import (
    "context"
    "fmt"
    "os"

    cometsdk "github.com/cometline/comet-sdk"
    "github.com/cometline/comet-sdk/llm"
    "github.com/cometline/comet-sdk/provider/openai"
)

p := openai.NewOpenAIProvider(
    os.Getenv("OPENAI_API_KEY"),
    cometsdk.WithBaseURL("https://api.openai.com"),
)

req := &cometsdk.Request{
    Model:     "gpt-4o",
    MaxTokens: 1000,
    Messages: []cometsdk.Message{
        {
            Role: cometsdk.RoleUser,
            Content: []cometsdk.Block{
                cometsdk.TextBlock{Text: "What is in this image?"},
                cometsdk.ImageBlock{MediaType: "image/png", Data: base64PNG},
            },
        },
    },
}

stream := llm.StreamMessage(context.Background(), p, req)

for ev := range stream.Events() {
    switch e := ev.(type) {
    case cometsdk.TextDeltaEvent:
        fmt.Print(e.Text)
    case cometsdk.ReasoningContentEvent:
        fmt.Print(e.Text) // chain-of-thought tokens
    case cometsdk.ToolCallDoneEvent:
        fmt.Printf("\n[done tool %s input=%s]\n", e.Name, e.Input)
    }
}

result, err := stream.Result()
if err != nil {
    panic(err)
}

fmt.Printf("\nfinish=%s tool_calls=%d\n", result.FinishReason, len(result.ToolCalls))
```

**Important:** drain `Events()` before calling `Result()` — otherwise the stream deadlocks.

### JSON extraction: `llm.GenerateJSON`

For structured extraction (used by CometMind memory), `GenerateJSON` sets OpenAI `response_format: json_object` or appends a JSON-only hint for Anthropic, then parses the model output into your struct.

### Lower-level primitive: `Provider.Stream()`

Use `Provider.Stream()` when you want raw provider-normalized events and will assemble the response yourself.

```go
ch, err := p.Stream(context.Background(), req)
if err != nil {
    panic(err)
}

for event := range ch {
    switch e := event.(type) {
    case cometsdk.TextDeltaEvent:
        fmt.Print(e.Text)
    case cometsdk.ReasoningContentEvent:
        fmt.Print(e.Text)
    case cometsdk.ToolCallDoneEvent:
        fmt.Printf("\n[tool: %s(%s)]\n", e.Name, e.Input)
    case cometsdk.StepFinishEvent:
        fmt.Printf("\n[tokens: %d in / %d out]\n", e.Usage.InputTokens, e.Usage.OutputTokens)
    case cometsdk.ErrorEvent:
        panic(e.Err)
    }
}
```

---

## Event vocabulary

| Event | Meaning |
|---|---|
| `TextDeltaEvent` | Visible answer text chunk |
| `ReasoningStartEvent` | Reasoning block begins |
| `ReasoningContentEvent` | Reasoning token chunk |
| `ToolCallStartEvent` | Tool call started (name known, input may be partial) |
| `ToolCallDeltaEvent` | Partial tool argument JSON |
| `ToolCallDoneEvent` | Complete tool call with valid JSON input |
| `StepFinishEvent` | One model step ended; carries `FinishReason` and `TokenUsage` |
| `ErrorEvent` | Mid-stream failure; channel closes after |
| `DoneEvent` | Terminal success marker |

After `Collect` / `StreamMessage.Result`, reasoning is stored separately in `Message.ReasoningContent` as `ReasoningBlock` values — not mixed into the main answer text.

---

## Provider-specific options

Any parameter not in the `Request` struct can be passed through `Options` without changing SDK code:

```go
// Anthropic — thinking, top_k, top_p, cache_control, etc.
Options: map[string]any{
    "anthropic": map[string]any{
        "top_k": 40,
        "thinking": map[string]any{
            "type":          "enabled",
            "budget_tokens": 5000,
        },
    },
}

// OpenAI — top_p, presence_penalty, seed, etc.
Options: map[string]any{
    "openai": map[string]any{
        "top_p":             1.0,
        "presence_penalty":  1.0,
        "frequency_penalty": 0.5,
    },
}
```

SDK-managed fields (`model`, `messages`, `stream`, `max_tokens`) cannot be overridden via Options. Use the top-level `Request.Temperature` field for providers that support it.

### ChatGPT Codex

`provider/codex` talks to ChatGPT Codex's `/responses` endpoint and reuses the local Codex CLI login. Run `codex login` first so `~/.codex/auth.json` exists. The provider refreshes the borrowed access token when possible and does not use an API key.

---

## Configuration options

```go
p := anthropic.NewAnthropicProvider(apiKey,
    cometsdk.WithBaseURL("https://custom-endpoint.example.com"),
    cometsdk.WithTimeout(30 * time.Second),
    cometsdk.WithMaxRetries(3),
    cometsdk.WithHTTPClient(myHTTPClient),
    cometsdk.WithLogger(slog.Default()),
    cometsdk.WithBearerAuth(), // unified gateway auth for Anthropic
)
```

### Debug logging

```go
log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))
p := anthropic.NewAnthropicProvider(apiKey, cometsdk.WithLogger(log))
```

Pass `cometsdk.WithLogger(nil)` to silence all SDK output.

---

## OpenAI-compatible quirks handled in code

The OpenAI provider includes compatibility shims for real-world gateways:

- **`reasoning_content`** treated as a reasoning alias (DeepSeek and similar)
- Embedded thinking tags in content (for example `<think>...</think>`)
- **`reasoning_split`** retry for MiniMax / `mimo-*` models
- Image input downgrade when a model rejects vision (retries with a text placeholder)
- `stream_options.include_usage: true` so usage arrives before `[DONE]`

---

## Error handling

All errors are concrete types, inspectable with `errors.As`:

```go
_, err := llm.GenerateMessage(ctx, p, req)
if err != nil {
    var rle *cometsdk.RateLimitError
    if errors.As(err, &rle) {
        time.Sleep(rle.RetryAfter)
    }
}
```

| Error type | When |
|---|---|
| `AuthError` | Invalid or missing API key (401/403) |
| `RateLimitError` | Rate limited (429); carries `RetryAfter` |
| `ServerError` | Provider non-success HTTP response (including context-length 400s) |
| `StreamError` | Error occurring mid-stream (after HTTP 200) |

Finish reasons are normalized by `NormalizeFinishReason` to `stop`, `tool_use`, `max_tokens`, or `error`.

---

## Testing

```bash
make test               # go test ./..., no API calls, CI-safe
make test-verbose       # same with -v
make test-anthropic     # Anthropic package only
make test-openai        # OpenAI package only
make test-live          # real API calls (requires env vars)
make test-live-anthropic
make test-live-openai
make lint               # golangci-lint (must be installed)
```

Provider packages replay checked-in SSE fixtures under `provider/*/fixtures/` via `httptest` — update fixtures when parser behavior changes.

**Live test environment variables:**

```bash
# Company unified API (supports both Anthropic /v1/messages and OpenAI /v1/chat/completions)
export CUSTOM_API_KEY="..."
export CUSTOM_BASE_URL="https://your-company-api.example.com"
# also valid:
export CUSTOM_BASE_URL="https://your-company-api.example.com/v1"

# Direct provider access (used as fallback when CUSTOM_API_KEY is not set)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."

# Optional model override for llm live tests
export LIVE_TEST_MODEL="gpt-4o"
```

---

## Dependencies

| Package | Purpose |
|---|---|
| `github.com/cenkalti/backoff/v4` | Exponential backoff retry (runtime) |
| `github.com/stretchr/testify` | Test assertions (test only) |
| `log/slog` | Structured logging (stdlib) |

---
