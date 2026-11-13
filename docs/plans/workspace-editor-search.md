# Workspace editor search

## Goal

When the workspace file editor has focus, `Cmd+F` searches the open file. `Cmd+P` cycles Wiki, Workspace, and Content. Content searches workspace text. `Cmd+Shift+F` remains chat search.

This is not go-to-definition. Do not add `Cmd+Click` navigation in this change.

## Current code

- `FileEditor.svelte` builds a CodeMirror 6 view with `basicSetup`. It already handles `Mod-s` and scrolls to `revealRange`.
- `@codemirror/search` is not installed. `basicSetup` does not include the search panel.
- `findInSession` is `Cmd+F` and calls `shellStore.requestSessionFind()`.
- `focusSearch` is `Cmd+Shift+F` and focuses the chat-list search.
- Both shortcuts are handled in `app-shell-shortcuts.svelte.ts` and `MiniShell.svelte`.
- `openWorkspaceFilePreview(path, reveal)` can open a workspace file and scroll to a line. `FileRevealRange` is 1-based.
- CometMind `grep` already searches with `rg` and falls back to a native scanner. It is an agent tool, not a UI endpoint.

## Behavior

1. If focus is inside the workspace CodeMirror editor, `Cmd+F` opens CodeMirror's find panel for the current document. `Cmd+G` and `Shift+Cmd+G` move between matches. `Escape` closes the panel.
2. The same focused editor uses `Cmd+Shift+F` for workspace content search. This does not focus the chat sidebar.
3. If focus is outside that editor, `Cmd+F` still finds in the current chat and `Cmd+Shift+F` still searches chats.
4. A workspace search runs against the session workspace root, respects `.gitignore`, skips binary files, and caps results. Clicking a result opens that file in the workspace panel and reveals the matched line.
5. Wiki files and web tabs do not start a workspace search. The editor must be a workspace file.
6. The find panel must work on a read-only file. It must not mark the file dirty.

## Implementation

1. Add `@codemirror/search` to `cometline`.
2. In `FileEditor.svelte`, add `search({ top: true })` and `searchKeymap` beside the existing save keymap. Give the panel colors that match the editor theme.
3. Add a workspace content-search endpoint to CometMind, backed by the existing `rg` path and native fallback in `fsops`. Return path, 1-based line, column, and a short line preview. Cap the result count. Do not call the agent `grep` tool from the UI.
4. Generate the TypeScript client from `openapi.yaml`. Do not hand-edit generated files.
5. Add a workspace-search dialog in the workspace feature. It takes a query, calls the endpoint, and uses `openWorkspaceFilePreview` with the result line as `reveal`.
6. Route shortcuts by focus. The document listener may handle `Cmd+F` and `Cmd+Shift+F` before CodeMirror. If `event.target` is inside the active workspace `.cm-editor`, prevent the chat action. Let CodeMirror handle `Cmd+F`. Open the workspace-search dialog for `Cmd+Shift+F`.
7. Apply the same focus split in the mini window. A mini window with no workspace editor keeps the chat shortcuts.
8. Update the shortcut labels only if the settings screen would otherwise claim that these keys always search chats. The keys still search chats outside the editor.

## Out of scope

- Replace-in-file and replace-all.
- Search across wiki files.
- Regex options beyond what the first endpoint needs.
- `Cmd+Click` go-to-definition.
- Changing `Cmd+P` file-name search.

## Verification

### Automated

Run from `cometline`:

```bash
pnpm exec prettier --check src/lib/features/workspace/components/FileEditor.svelte
pnpm exec vitest run src/lib/keyboard-shortcuts.test.ts src/lib/features/shell
```

Add or extend tests for:

- `Cmd+F` inside `.cm-editor` does not call `requestSessionFind`.
- `Cmd+F` outside the editor still calls `requestSessionFind`.
- `Cmd+Shift+F` inside `.cm-editor` opens workspace search and does not focus sidebar search.
- `Cmd+Shift+F` outside the editor still focuses sidebar search.
- A search result calls `openWorkspaceFilePreview` with the file path and 1-based line.
- An empty query does not call the endpoint.
- A failed search shows an error and does not open a file.

Run the CometMind search tests from `cometmind`:

```bash
go test ./internal/tools/fsops ./internal/server
```

The server test should cover a match, a `.gitignore` skip, a binary skip, and the result cap.

### Manual

Use a workspace with one large text file and one ignored dependency directory.

1. Open a workspace file in the editor and click inside it. Press `Cmd+F`. The find panel opens over the editor. Type a word, then press `Cmd+G`. The selection moves to the next match.
2. Press `Escape`. The panel closes and the file is not dirty.
3. Click inside the same editor. Press `Cmd+Shift+F`. A workspace search opens. The chat sidebar search does not take focus.
4. Search for a word that appears in two files. Click the second result. That file opens and the matched line is visible.
5. Click the chat composer. Press `Cmd+F`. The chat find bar opens, not the editor panel.
6. Press `Cmd+Shift+F` from the chat. The sidebar chat search focuses.
7. Open a wiki file and press `Cmd+Shift+F`. It does not search the code workspace.
8. Resize the window narrow enough that the workspace panel is beside the chat. Repeat steps 1 and 3. The shortcuts still follow editor focus.
