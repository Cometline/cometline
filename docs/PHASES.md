# CometMind Phased Development Plan

CometMind is the Go backend and local agent runtime for Cometline. It owns the trusted boundary: provider selection, agent loop, persistence, permissions, tools, memory, secrets, local API, and future extension systems.

## North Star

Build a local-first Hermes-like agent runtime in Go: persistent sessions, provider switching, permissioned tools, durable memory, skills, scheduler, subagents, MCP, browser/computer-use tools, and a stable HTTP/SSE API consumed by the SvelteKit frontend.

## Load-Bearing Boundaries

- CometMind is the brain. Cometline is the UI/shell.
- CometMind talks to models only through `comet-sdk`.
- Every powerful action must flow through a permissioned tool runtime.
- Every UI-visible runtime transition must be represented in REST/SSE contracts.
- SQLite is the local source of truth for sessions, messages, tool calls, memory metadata, and audit trails.
- Secrets must not be stored as plain config once provider management is user-facing.

## Phase 0 — Stabilize Current Runtime Slice

Goal: make the existing session/message/tool loop predictable before adding larger Hermes-like systems.

Day-by-day:

1. Reconcile `openapi.yaml` with implemented server routes and documented Cometline needs.
2. Add API-level tests for session creation, workspace lookup, message streaming, abort, and error responses.
3. Add a migration/version marker to SQLite schema management so future table changes are explicit.
4. Add run-loop tests with a fake provider that emits text, reasoning, tool calls, errors, and max-step behavior.
5. Document current SSE event ordering guarantees and cancellation behavior.

Exit criteria:

- `go test ./...` passes.
- The frontend can rely on one canonical OpenAPI file.
- Runtime errors map to stable API error codes.

## Phase 1 — Permissioned Tool Loop

Goal: make tool execution safe enough for real local work.

Day-by-day:

1. Add tool metadata to `internal/tools.Tool`: safety class, default approval policy, human-readable summary, and whether output may contain secrets.
2. Extend `tool_calls` with `status`, `risk_class`, `approval_scope`, `requested_at`, `approved_at`, `denied_at`, and `decision_reason`.
3. Add runtime event types: `tool_permission_requested`, `tool_permission_resolved`, and `tool_execution_started`.
4. Change the agent runner so impactful tools pause the run instead of executing immediately.
5. Add REST endpoints to approve or deny pending tool calls for once/session/always scopes.
6. Add an allowlist/blocklist config model for command patterns and tool names.
7. Add tests for read-only auto-approval, write/run pending state, denial feedback, cancellation while pending, and resume after approval.

Exit criteria:

- `read_file` and `list_dir` can run automatically.
- `write_file` and `run_command` require approval by default.
- The UI can render and resolve permission prompts entirely from API/SSE data.

## Phase 2 — Provider, Model, Config, And Secrets API

Goal: support real settings UI without leaking credentials.

Day-by-day:

1. Add `GET /api/v1/config` and `PATCH /api/v1/config` for non-secret runtime settings.
2. Add `GET /api/v1/providers`, `GET /api/v1/models`, and `POST /api/v1/providers/test`.
3. Introduce a secret store interface with local dev fallback and OS keychain implementation where available.
4. Store provider records with secret references, not raw keys.
5. Add redaction utilities for logs, API responses, exports, and tool outputs.
6. Add tests for provider validation, missing secrets, invalid credentials, and key rotation.

Exit criteria:

- Cometline can build provider settings without touching config files directly.
- Secrets are never returned by local API responses.
- Provider test failures produce user-actionable errors.

## Phase 3 — Memory MVP

Goal: give the agent durable, inspectable memory before adding auto-learning.

Day-by-day:

1. Add `memories` table with scope, kind, content, source, confidence, timestamps, and archived state.
2. Add `memory_events` audit table for create/update/delete/inject/extract decisions.
3. Implement manual memory CRUD and search endpoints.
4. Add a simple retrieval layer that injects selected memories before the model call.
5. Emit `memory_injected` SSE events so the UI can show what influenced the run.
6. Add memory settings for enabled/disabled, max injected memories, and scope filtering.
7. Add tests for workspace-scoped memories, global user memories, injection limits, and deletion.

Exit criteria:

- User can create, inspect, edit, delete, and search memories.
- Agent runs can include memories with visible traceability.
- No automatic extraction exists yet; manual memory must be trustworthy first.

## Phase 4 — Memory Extraction And User Model

Goal: make CometMind learn carefully from conversations without silently poisoning context.

Day-by-day:

1. Use structured model output to propose memory writes after important conversations.
2. Stage proposed writes for review before saving by default.
3. Add write approval settings: off, prompt, auto-save trusted categories.
4. Add duplicate detection and memory consolidation rules.
5. Add user profile memory type for preferences, communication style, stable identity, and project facts.
6. Add tests for prompt injection rejection, conflicting memories, and stale memory archival.

Exit criteria:

- The agent can propose useful memories.
- User can approve/reject staged memory writes.
- Memory updates are auditable and reversible.

## Phase 5 — Skills And Prompt Packs

Goal: add procedural memory similar to Hermes skills.

Day-by-day:

1. Define `SKILL.md` format and local skill directory layout under the CometMind data directory.
2. Add skill discovery, metadata indexing, and read-only skill loading.
3. Add skill selection by explicit slash command and by model-visible skill index.
4. Add permission gates for skill-managed files and scripts.
5. Add staged skill creation/update flow; do not allow silent skill rewrites at first.
6. Add APIs for listing, viewing, enabling, disabling, creating, and deleting skills.
7. Add tests for skill shadowing, malformed frontmatter, missing files, and staged updates.

Exit criteria:

- Skills can be installed locally and loaded into a run.
- Agent-created skills require review before becoming active.
- Skill docs and scripts never bypass tool permissions.

## Phase 6 — MCP And External Tools

Goal: connect external tools without compromising the local trust boundary.

Day-by-day:

1. Add MCP server registry and stdio process manager.
2. Import MCP tool schemas into the same tool registry used by built-ins.
3. Apply the same permission metadata model to MCP tools.
4. Add per-server environment variable filtering and secret allowlists.
5. Add API and UI-facing status for connected/disconnected/error MCP servers.
6. Add tests using a tiny fake MCP stdio server.

Exit criteria:

- MCP tools appear as first-class tools.
- MCP cannot access secrets or filesystem paths unless explicitly configured.
- Tool audit records do not distinguish built-in vs MCP for safety review.

## Phase 7 — Scheduler And Background Jobs

Goal: support Hermes-like scheduled automations safely.

Day-by-day:

1. Add `jobs` table with prompt, schedule, target session/space, enabled state, and last run status.
2. Add a scheduler service inside CometMind, disabled by default in development.
3. Define headless approval behavior: deny impactful tools unless job policy explicitly allows them.
4. Add job run history and SSE/loggable event records.
5. Add APIs to create, pause, resume, run-now, and delete jobs.
6. Add tests for missed runs, disabled jobs, permission denial, and crash recovery.

Exit criteria:

- Scheduled jobs can run local assistant tasks without a foreground chat.
- Every job action is auditable.
- Dangerous commands do not auto-run from background jobs by default.

## Phase 8 — Subagents And Coding Delegation

Goal: support isolated workstreams and delegate coding to specialized ACP-compatible agents.

Day-by-day:

1. Define subagent session model: parent session, child session, purpose, status, and output summary.
2. Add task delegation tool that creates a child run with bounded context.
3. Add ACP client wrapper for opencode/claude-code-style coding agents.
4. Stream child progress into parent session using structured events.
5. Add cancellation and timeout policies for delegated tasks.
6. Add tests with fake child agents and fake ACP responses.

Exit criteria:

- CometMind can delegate coding without embedding coding-agent behavior in its core loop.
- Parent sessions can display child progress and final artifacts.

## Phase 9 — Browser And Computer Use

Goal: add browser/computer-use capabilities only after permissions and audit are mature.

Day-by-day:

1. Define browser tool risk classes: read page, navigate, click, type, upload, download.
2. Add browser session manager and local-only automation backend.
3. Emit screenshot/page-read artifacts as stored runtime artifacts.
4. Gate mutating actions such as click/type/upload by default.
5. Add Chrome Relay or equivalent only behind explicit user pairing.
6. Add tests for domain allowlists, blocked URLs, and permission prompts.

Exit criteria:

- Browser tools are observable and revocable.
- The user can see what page/action the agent is acting on before mutation.

## Phase 10 — Messaging Gateway

Goal: let the same CometMind runtime operate across chat surfaces.

Day-by-day:

1. Define gateway-independent message/session identity model.
2. Add platform adapter interface for inbound messages and outbound delivery.
3. Start with one platform only after local desktop is stable.
4. Add user allowlists, pairing, and per-platform authorization.
5. Reuse memory, skills, permissions, and scheduler across gateway sessions.
6. Add tests for unauthorized users, pairing expiry, and cross-platform session routing.

Exit criteria:

- Gateway sessions use the same runtime contracts as desktop sessions.
- Unauthorized users cannot trigger tools or read session data.

## Recommended Immediate Build Order

1. Phase 0: OpenAPI/runtime stabilization.
2. Phase 1: permissioned tool loop.
3. Phase 2: provider/config/secrets API.
4. Phase 3: manual memory MVP.
5. Phase 4: reviewed memory extraction.
6. Phase 5: skills.

Do not start MCP, scheduler, browser, or gateway work until permissions and secrets are solid.
