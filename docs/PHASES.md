# Comet SDK Phased Development Plan

Comet SDK is the provider-normalized LLM I/O layer for Cometline. It must stay small, deterministic, and free of agent policy. If a feature needs memory, tools, permissions, sessions, UI, or workflow decisions, it belongs in CometMind or Cometline instead.

> Domain glossary: [CONTEXT.md](../CONTEXT.md). Architecture decisions: [docs/adr/](./adr/).

## Status Snapshot

- **Phase 0 — Contract Freeze**: largely done. `FinishReason` is normalized at the Stream seam (ADR-0002), sealed `Event`/`Block` sum types documented (ADR-0001), golden SSE fixtures exist for both providers. Remaining: explicit stable/experimental annotations on every public type.
- **Phase 1 — Provider Robustness**: largely done. Cross-provider behavior consolidated in `internal/providerbase` (ADR-0003), `Retry-After` honored (ADR-0004), typed errors for auth/rate-limit/server/stream. Remaining: malformed-SSE fixtures, log redaction rules.
- **Phase 2+**: not started.

## North Star

Expose one stable Go interface that lets CometMind stream model output, tool calls, reasoning deltas, usage, and provider errors across Anthropic, OpenAI, and OpenAI-compatible APIs without leaking provider-specific behavior into the runtime.

## Load-Bearing Boundaries

- `Provider.Stream(ctx, *Request) (<-chan Event, error)` is the core contract.
- `llm.StreamMessage` is the runtime-friendly assembly layer for agent loops.
- Provider-specific knobs live in `Request.Options`, not in CometMind conditionals.
- SDK errors must be typed enough for CometMind to show actionable UI states.
- SDK must not persist data, execute tools, manage secrets, or choose policies.

## Phase 0 — Contract Freeze

Goal: make the current provider contract safe to build the runtime on top of.

Day-by-day:

1. Document every public type in `sdk.go` as either stable, experimental, or internal-by-convention.
2. [done] Add golden stream fixtures for Anthropic and OpenAI text, reasoning, single tool, multiple tools, and provider error streams.
3. [done] Define exact `FinishReason` values accepted by CometMind: `stop`, `tool_use`, `max_tokens`, `error`. Normalized at the Stream seam — see ADR-0002.
4. [done] Add tests proving `llm.StreamMessage` never drops tool calls or usage when deltas arrive interleaved with text/reasoning.
5. Add a short compatibility note for OpenAI-compatible gateways and `WithBaseURL` behavior.

Exit criteria:

- `make test` passes with no network calls.
- CometMind can treat SDK events as provider-neutral.
- Public event semantics are documented well enough to generate API docs from comments later.

## Phase 1 — Provider Robustness

Goal: make provider calls reliable enough for a desktop app.

Day-by-day:

1. [done] Normalize context cancellation behavior across Anthropic and OpenAI providers.
2. [done] Confirm retry behavior for 429, 5xx, transient network errors, and `Retry-After` — server `Retry-After` is now honored (ADR-0004). Shared retry/classification lives in `internal/providerbase` (ADR-0003).
3. [done] Add typed errors for authentication, rate limits, server errors, and malformed provider streams. (`ContextLengthError` was removed as a phantom type — never constructed; re-add when a provider actually maps it.)
4. Add provider test fixtures for partial JSON tool-call arguments and malformed SSE frames.
5. Add debug logging redaction rules so request logs never expose API keys or sensitive message content by default.

Exit criteria:

- CometMind can map SDK failures to UI-safe error codes.
- Live tests are optional and gated by environment variables.
- Retried requests respect caller cancellation.

## Phase 2 — Model Capability Metadata

Goal: let CometMind decide which model can support tools, reasoning, images, JSON mode, and context size without hardcoding provider quirks.

Day-by-day:

1. Add a `ModelInfo` type with provider ID, model ID, context window, max output, tool support, reasoning support, and modality flags.
2. Add static provider model registries for the first supported Anthropic and OpenAI models.
3. Add provider method or helper to list known models without requiring network access.
4. Document how external gateways can provide custom model metadata.
5. Add tests that CometMind can validate a selected model before starting a run.

Exit criteria:

- CometMind can populate provider/model settings UI from SDK-owned metadata or runtime config.
- Unsupported tool/reasoning use can fail before a model call.

## Phase 3 — Structured Output And JSON Mode

Goal: support non-chat runtime features that need structured model output, such as title generation, memory extraction, and skill generation.

Day-by-day:

1. Add provider-neutral request fields for JSON object output where providers support it.
2. Define fallback behavior for providers without native JSON mode.
3. Add `llm.GenerateJSON` helper with schema validation owned by the caller.
4. Create fixtures for valid JSON, invalid JSON, and partial stream recovery.
5. Document which runtime features may use structured output.

Exit criteria:

- CometMind can build memory extraction and title generation without ad hoc provider prompts.
- JSON mode behavior is consistent across providers or explicitly marked as degraded.

## Phase 4 — Multimodal Readiness

Goal: prepare the SDK for future image/audio/document inputs without forcing CometMind to redesign message storage later.

Day-by-day:

1. Extend `Block` types with image and file references using provider-neutral metadata.
2. Decide whether SDK accepts bytes, file paths, URLs, or caller-managed object references.
3. Add conversion tests for provider-specific multimodal payloads.
4. Document storage responsibilities: SDK converts payloads, CometMind stores artifacts.
5. Keep multimodal disabled in CometMind until artifact storage exists.

Exit criteria:

- SDK can represent multimodal requests without changing the existing text/tool contract.
- CometMind can defer implementation safely.

## Phase 5 — Provider Expansion

Goal: add new providers only after the core contract is stable.

Candidate order:

1. OpenRouter or other OpenAI-compatible gateway.
2. Google Gemini if product need appears.
3. Local model runtime if Cometline needs offline mode.

Exit criteria for each provider:

- Text streaming fixture.
- Tool-call fixture if supported.
- Usage mapping.
- Error mapping.
- Live test gated by provider-specific environment variables.

## Not In Scope

- Tool execution.
- Tool permissions.
- Memory retrieval or extraction.
- Session persistence.
- Secret storage UX.
- Scheduler, gateway, browser automation, or plugins.

Those belong in CometMind and Cometline.
