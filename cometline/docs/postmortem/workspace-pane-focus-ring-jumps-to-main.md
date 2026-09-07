# Workspace pane focus ring jumps to the main window

**Date:** 2026-09-07  
**Components:** `AppShell.svelte`, `WorkspacePanel.svelte`, `MessageContextChips.svelte`, `FilePreview.svelte`, `shell.svelte.ts`, `workspace-pane-focus.ts`, `workspace-panel-state.ts`

## Symptom

With the workspace panel open, the glowing **pane focus ring** (`.pane-focus-active` on `<main>`) jumped from the workspace panel to the **main chat window**.

It happened when:

1. Opening a wiki or workspace file from the file tree.
2. Closing one file or URL tab while other tabs were still open.

It did **not** require clicking **Add to chat**. A **Viewing** chip still appeared in the composer on file open, which made it look like “add to chat” had run.

The intended rule: only an explicit **Add to chat** (selection popup, Git add path/diff, assistant selection), or a real click in the composer / thread, should move the main ring.

## What the ring actually is

`<main>` lights up when `shellStore.focusedPane === 'chat'` and the workspace panel is open:

```svelte
class:pane-focus-active={shellStore.focusedPane === 'chat' &&
  shellStore.workspacePanelOpen}
```

The workspace panel uses the same class when `focusedPane` is `web` or `terminal`.

So any unexpected `focusedPane = 'chat'` is the jump.

## Two different “add to chat” paths

| Action | What it writes | Moves the ring? |
| --- | --- | --- |
| Open wiki / workspace file | `setViewingFileContextForActive` — path-only **Viewing** chip | Must not |
| Close one of several tabs | Tab list + maybe a new active file / URL | Must not |
| **Add to chat** button | `addWebContextForActive` then `requestComposerFocus()` | Yes (design) |
| Click composer / thread | `handleMainMouseDown` → `setFocusedPane('chat')` | Yes (design) |
| Hide the last panel / last content | `requestComposerFocus()` | Yes (design) |

`setViewingFileContextForActive` never called `requestComposerFocus`. Opening a file already set `focusedPane = 'web'`. Closing one of several tabs also kept `focusedPane = 'web'` in the store when `stillFile` / `stillUrl` was true.

The ring jump was therefore a **side door**, not the Add to chat handler.

## Proven cause: effect dependencies

The session-change effect in `AppShell.svelte` called the store without isolating its reads:

```svelte
$effect(() => {
  void activeSessionId;
  shellStore.onActiveSessionChange();
});
```

Svelte tracks every synchronous reactive read, including reads inside called functions. `onActiveSessionChange()` reads the per-session panel surface and visibility maps, both directly and through `syncWorkspacePanelOpenForActiveSession()`.

Opening or closing a tab calls `applyPanelState()`, which replaces those maps. That schedules the effect again even when the session ID is unchanged. The effect then calls `onActiveSessionChange()`, whose non-terminal branch sets `focusedPane = 'chat'`.

The causal chain is:

```text
Open/close tab -> applyPanelState -> panel map replacement
  -> AppShell session-change effect reruns
  -> onActiveSessionChange -> focusedPane = 'chat'
```

This reproduces in mounted-component tests without Electron, CodeMirror, a composer chip, or a pointer event. A chip remount or DOM focus event does not synthesize `mousedown`; the earlier chip-remount explanation was not the cause.

The store-only tests passed because they asserted the immediate store result without mounting the effect that subsequently overwrote it.

## Policy

- Open / switch wiki or workspace files: keep `focusedPane = 'web'`, park on the panel host. The Viewing chip may update. **Ring stays on workspace.**
- Close a file or URL tab while others remain (active or inactive): keep `focusedPane = 'web'`, then `applyOwnedFocus`. **Ring stays on workspace.**
- Close the last tab, or hide the whole panel: existing browse / `requestComposerFocus` behavior.
- Explicit **Add to chat**: keep `requestComposerFocus()` so the main ring turns on and the composer is ready.
- A real click in the composer input or thread still takes the chat ring.

## Fix

Limit the session-change effect to its intended dependency:

```svelte
$effect(() => {
  void activeSessionId;
  // Panel mutations must not rerun the session-change focus reset.
  untrack(() => shellStore.onActiveSessionChange());
});
```

This preserves focus resets on actual session changes, while file opens, tab closes, guest navigation, and session-title updates do not reset pane ownership. No timers, repeated ownership writes, or global focus interception are needed to fix the ring.

## Local Focus Behavior

1. **`workspace-pane-focus.ts`**
   - `isComposerContextChipTarget` / `shouldClaimChatPaneFromMainPointer` — a pointer on `.message-context-chips` is not a chat-pane claim.

2. **`AppShell.svelte`**
   - `handleMainMouseDown` returns early unless `shouldClaimChatPaneFromMainPointer(event.target)`.

3. **`MessageContextChips.svelte`**
   - Mouse actions preserve DOM focus with `mousedown preventDefault`; buttons retain their normal keyboard tab stops. The earlier `tabindex="-1"` workaround was removed because it made open/remove/clear actions unreachable by Tab.

4. **`WorkspacePanel.svelte`**
   - After closing a file or URL tab, if the pane is still `web` and the panel is open, `tick()` then `applyOwnedFocus()`.
   - The content-identity `$effect` also tracks `panelFileTabs.length` and `panelUrlTabs.length`, so an inactive-tab close still reclaims.
   - File open still does `setFocusedPane('web')` then `applyOwnedFocus()`. Do **not** programmatically focus CodeMirror or the webview; park on `panelFocusEl`.

5. **No document-wide focus trap**
   - Removed the `focusin` listener and `shouldRejectFocusTransfer` helpers. A non-modal workspace panel must not reject focus in sidebar search, keyboard navigation, or other controls outside itself.

## Regression Tests

`AppShell.svelte.test.ts` mounts the real shell and workspace panel with real stores, stubbing unrelated child surfaces. It verifies the state and rendered ring after Svelte effects flush:

- Wiki/workspace file opens and Viewing-context updates keep workspace ownership.
- Closing inactive and active file/URL tabs while others remain keeps workspace ownership.
- Explicit composer-focus requests and mouse clicks still activate chat.
- Replacing session metadata without changing its ID preserves ownership; switching sessions still resets it.
- DOM focus outside the workspace is not trapped.

The five original regression cases failed before the `untrack` fix and passed afterward. A sixth case reproduced the document-level focus trap and passed after its removal. `MessageContextChips.svelte.test.ts` covers keyboard tab stops and remove/clear actions.

## How to avoid regressions

- **Do not call `requestComposerFocus()` from file-open or tab-close.** That function exists to take the chat ring. Viewing context is not a chat-ring claim.
- **Isolate reactive reads in session-change side effects.** Reading a store method inside `$effect` can subscribe to far more than the apparent session ID dependency.
- **Do not trap document focus in a non-modal pane or remove buttons from keyboard navigation.** Pane ownership and DOM focus are separate concerns.
- **Do not treat composer context chips as main-pane clicks.** They live in `<main>` only because the composer does. Keep `shouldClaimChatPaneFromMainPointer` in the mousedown path.
- **After any tab close that leaves tabs, reclaim workspace focus.** Do not rely on `panelFilePath` changing; inactive closes do not change it.
- **Park on `panelFocusEl`, never CodeMirror / `<webview>`.** Electron has bounced programmatic content focus back into the composer before.
- **If you add a new “add to chat” control**, call `requestComposerFocus()` there. If you add a new “I’m still in the workspace” mutation (open file, switch tab, guest URL sync), do not.

## Verification

- `make check` passed: generated-code freshness, SDK/runtime tests, frontend type checks, lint, and 1,065 Vitest tests.
- `pnpm run build` passed for the Electron main process and renderer. The build still reports large-chunk warnings.
- The native Electron interaction checklist below was not manually executed during this review.

1. Open the workspace panel → ring on the panel, not `<main>`.
2. Open a wiki file, then a workspace file → Viewing chip updates; ring stays on the panel.
3. Open several file tabs, close an inactive one, then the active one (while others remain) → ring stays on the panel.
4. Same for several URL tabs.
5. Select text in a file (or Git diff / assistant message) and click **Add to chat** → main ring turns on, composer is focused.
6. Click the composer or thread yourself → main ring turns on.
