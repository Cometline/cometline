# Postmortem: Codex Streaming and Tool-Call Failures

Date: 2026-06-25

## Summary

Codex sessions could complete without rendering an assistant response in Cometline. After the streaming issue was fixed, Codex tool calls surfaced a second failure: tool arguments were sometimes passed to Go tools as a JSON string instead of a JSON object, and replayed tool results were missing the function name required by the Codex Responses API.

## Impact

- Codex chat turns appeared to succeed but produced no assistant message in the UI.
- Empty assistant turns could be persisted and later poison provider switches, because OpenAI-compatible providers reject assistant history entries with neither content nor tool calls.
- Codex tool calls could fail during execution with JSON unmarshal errors.
- Follow-up Codex requests after tool execution could fail with HTTP 400: `Missing required parameter: 'input[7].name'`.

## Timeline

- A user reported that Codex produced only user bubbles and no assistant replies.
- Logs showed Codex streams opening and closing with `events=0`, `saw_first_output=false`, and an empty finish reason.
- A direct curl request to the Codex backend worked, proving the account and upstream API were healthy.
- Live diagnostics reproduced that real agent requests with many tools caused very large early SSE events.
- The SSE scanner limit was increased and stream errors are now preserved when a provider closes without a `DoneEvent`.
- After normal responses worked, a Codex tool-call session exposed argument normalization and replay conversion issues.
- Tool-call parsing and replay conversion were patched and verified with targeted tests.

## Root Causes

### 1. SSE scanner token limit was too small

The shared SSE scanner used Go's default `bufio.Scanner` maximum token size of 64 KiB. Codex echoes the full request shape, including the complete tool schema registry, in early Responses API events such as `response.created` and `response.in_progress`.

With the real CometMind tool registry, those event lines can exceed 64 KiB. The scanner failed before any SDK events were forwarded, which made the stream look like a clean no-op turn.

### 2. Stream errors could be converted into empty successful turns

`llm.MessageStream.run` stored provider `ErrorEvent`s but, if the channel closed without a `DoneEvent`, finalized the message instead of returning the stored error. That turned scanner failures into empty assistant results.

### 3. Codex tool arguments have multiple streaming shapes

Codex can emit tool-call arguments as `response.function_call_arguments.delta` chunks and can also report final `arguments` as a JSON string containing the actual JSON object. The parser only consumed final raw arguments and passed string-wrapped JSON through unchanged.

That meant a tool like `list_dir` received `"{\"path\":\".\"}"` instead of `{ "path": "." }`, causing Go JSON unmarshalling to fail.

### 4. Codex requires tool-output names during replay

The SDK's provider-agnostic `ToolResultBlock` stores the tool call ID and content, but not the tool name. Codex replay conversion emitted `function_call_output` items without `name`, which the Codex backend rejected.

The tool name is available earlier in the same history from the assistant `ToolCallBlock`, so the Codex converter now tracks `call_id -> name` while converting messages and attaches the name to matching tool outputs.

## Fixes

- Increased the shared SSE scanner maximum event size to 16 MiB.
- Added a regression test for large SSE event lines.
- Preserved stream errors when a provider channel closes without a `DoneEvent`.
- Avoided persisting empty assistant turns from no-op results.
- Dropped historical empty assistant messages during provider history normalization.
- Made Codex stream parsing use the SSE `event:` field when JSON payloads omit `type`.
- Added Codex handling for tool argument deltas.
- Normalized Codex string-wrapped tool arguments into JSON objects before emitting `ToolCallDoneEvent`.
- Deduplicated completed Codex tool calls when both argument-done and output-item-done events are present.
- Added tool names to replayed Codex `function_call_output` items when the matching assistant tool call is present in history.

## Verification

- `rtk go test ./internal/sse ./llm ./provider/codex` passed in `comet-sdk`.
- `rtk go test ./internal/agent` passed in `cometmind`.
- The sidecar was rebuilt and restarted.
- Manual user verification confirmed normal Codex answers and tool execution now work.

## Prevention

- Keep provider parsers tolerant of real streamed event shapes, not only the smallest documented examples.
- Treat oversized SSE events as expected for providers that echo tool schemas.
- Never convert provider stream errors into successful empty assistant turns.
- Provider replay conversion should preserve provider-required metadata even when the provider-agnostic SDK model omits it.
- Add tests whenever a provider supports tool calls, including streamed argument deltas, final-only arguments, and replayed tool outputs.

## Anthropic Comparison

Anthropic is less likely to hit this exact failure mode because its parser already accumulates `input_json_delta` chunks by content block index and emits `ToolCallDoneEvent` only on `content_block_stop`. It also receives tool metadata from `content_block_start`, so the ID and name are tracked before argument deltas arrive.

The shared large-SSE scanner fix still benefits Anthropic defensively, but Anthropic's current event format does not typically echo the full tool registry in a single large SSE line the same way Codex did.

The remaining Anthropic-specific risk is malformed or unexpected tool argument JSON. The parser currently passes the accumulated JSON through directly. If Anthropic ever emitted string-wrapped tool arguments or changed its delta shape, it would need a similar normalization layer, but there is no evidence of that behavior in the current implementation.
