# Session switch: slow first load and stuck "no messages"

**Date:** 2026-06-17  
**Components:** `chat.svelte.ts` (`loadTranscript`), `ChatView.svelte` (snapshot / `hasVisibleConversation`), `+layout.svelte` (`loadSessions`), `Composer.svelte` (file-index prefetch), `cometmind/internal/session/transcript.go` + `service.go` (`BuildSDKMessages`), `tool_calls.sql`

## Symptom

Switching to a different chat session from the sidebar — especially one whose
workspace differs (a session created via `/change` fork) — had two problems:

1. The **first API call after the switch was very slow**.
2. If the user clicked **another** session while the first was still loading, the
   second session's transcript **never appeared** (empty state, stuck until you
   navigated back to it).

## Root cause

### Slow first load — three concurrent backend hits contend on one SQLite connection

On a workspace-crossing switch, `Sidebar.selectSession` sets
`shellStore.setWorkspacePath(...)` and `goto('/session/<id>')`. That fanned out
into several simultaneous requests competing for the localhost server and the
single `modernc.org/sqlite` connection:

1. **`+layout.svelte` `loadSessions`** ran `await ensureWorkspace()` **then**
   `await listAllSessions()` — sequential, and fired on *every* workspace change
   even though the all-sessions list does not depend on the current workspace.
2. **`Composer.svelte`** immediately fired `GET /api/v1/workspaces/files?limit=500`,
   which does a full `filepath.WalkDir` over the workspace (hundreds of ms on a
   large repo) — purely to warm the `@`-mention picker the user hadn't opened yet.
3. **`ChatView`** fired the transcript load `GET /api/v1/sessions/<id>/messages`,
   whose backend `LoadTranscript` had an **N+1**: one `ListToolCallsByMessage`
   query per assistant message.

The file walk (#2) and the N+1 transcript (#3) both landed at the same moment as
the session-list refetch (#1), so the user-visible transcript waited behind work
it didn't need.

### Stuck "no messages" — superseded load never finishes its state, view falls back to an empty snapshot

`chat.svelte.ts` `loadTranscript` bailed out of a superseded run with
`if (run !== loadRun) return`, and its `finally` only cleaned up when
`run === loadRun`:

```ts
} finally {
    if (run === loadRun) {        // a newer switch advanced loadRun
        isLoading = false;        // → never runs for the abandoned load
        ...
    }
}
```

So a quickly-superseded load could leave `isLoading`/`loadPromise` in a stale
state. Meanwhile `ChatView.hasVisibleConversation` fell back to a per-component
snapshot when the store was still bound to the previous session:

```ts
if (chatStore.sessionID === sessionId) {
    return chatStore.items.length > 0 || chatStore.isLoading;
}
return snapshotItems.length > 0 || snapshotLoading;   // freshly mounted → both empty
```

A freshly mounted `ChatView` for the second session had `snapshotItems = []` and
`snapshotLoading = false`, so `hasVisibleConversation` was `false` → it showed the
**empty state** instead of a loading spinner, and never recovered.

## Fix

1. **`chat.svelte.ts`** — key the `finally` cleanup on `loadPromiseSession`
   (the latest request for this session id) instead of `loadRun`, so a
   superseded run always finishes its loading state and clears its promise. Only
   apply results when this run still owns the current session.

2. **`ChatView.svelte`** — default `snapshotLoading = true` and add a
   `snapshotSynced` flag. Before the first sync (mid-switch), assume loading so
   a freshly mounted view shows the spinner instead of flashing/locking the
   empty state.

3. **`+layout.svelte`** — decouple the session list from `workspacePath`. The
   all-sessions list loads once at startup (subsequent new/fork/delete mutate
   `sessionStore` locally). `ensureWorkspace` now runs in the background, only
   when the path actually changes, and never blocks the list or the transcript.

4. **`Composer.svelte`** — defer the workspace file walk to
   `requestIdleCallback` (fallback `setTimeout`), cancelled on cleanup, so the
   `@`-mention prefetch never competes with the session switch.

5. **Backend N+1** — add `ListToolCallsBySession` (one JOIN query) and group by
   `message_id` in memory. `LoadTranscript` and `BuildSDKMessages` no longer run
   one tool-call query per assistant message; `assistantBlocks` became a pure
   function taking the pre-grouped calls.

## How to avoid regressions

- **Async store state must self-finalize per owner, not per global run counter.**
  Keying cleanup on a monotonically-increasing `loadRun` drops cleanup for
  superseded runs. Key it on the per-session request identity.
- **A freshly mounted view should assume "loading", not "empty".** Empty state
  is only correct after a confirmed empty result.
- **The all-sessions list is workspace-independent.** Don't refetch it on every
  `workspacePath` change; mutate the store locally on create/fork/delete.
- **Keep non-critical warmups (file-index walk) off the critical path.** Defer to
  idle and cancel on teardown so navigation stays responsive.
- **Watch for N+1 over the SQLite connection.** It serializes work; a per-message
  query amplifies latency for long transcripts. Prefer one grouped query.

## Verification

1. `cd cometmind && go test ./...` — full suite green (incl. session/server).
2. `cd cometline && pnpm run test` — 159 tests green.
3. `make check` — codegen freshness (sqlc/openapi), Go tests, svelte-check.
4. In-app: fork via `/change`, then rapidly switch between sessions in different
   workspaces — the first load is snappy and every switched-to session loads its
   messages (spinner, then transcript), with no stuck empty state.
