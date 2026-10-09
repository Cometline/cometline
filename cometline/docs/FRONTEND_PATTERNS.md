# Frontend Patterns (Cometline)

Conventions for the SvelteKit renderer. See also [`STYLING.md`](../STYLING.md) and [`COMETLINE_ARCHITECTURE.md`](COMETLINE_ARCHITECTURE.md).

## File roles

| Extension    | Purpose                                                |
| ------------ | ------------------------------------------------------ |
| `.svelte`    | Markup, bindings, and `$effect` wiring only            |
| `.svelte.ts` | Reactive controllers (`$state`, `$derived`, callbacks) |
| `.ts`        | Pure functions — unit-testable, no Svelte imports      |

**Stores** (`*.svelte.ts` singletons) are for cross-route session state. Prefer feature-local controllers when state does not need to be global.

## Controller pattern

Heavy UI features use a `create…Controller(deps)` factory that accepts getter callbacks (mirrors [`conversation-controller.ts`](../src/lib/features/chat/conversation-controller.ts) and chat thread controllers):

```typescript
export function createThreadScroll(deps: { getScroller: () => HTMLElement | null }) {
	// $state + methods
	return { scrollToBottom, setScroller };
}
```

The `.svelte` shell wires props, mounts controllers, and renders child components.

## Chat thread layout

```
ChatThread.svelte          — thin orchestrator + {#each} dispatch
├── createFoldController   — expand/collapse state
├── createThreadScroll     — scroll anchoring + near-top load-older
├── createThreadVirtual    — measure cache, virtual window, find jump
├── createSessionFindController — find UI + shell/session wiring
├── createThreadClocks     — copy feedback, memory cycle tick
├── thread-visibility.ts   — pure show/hide predicates
└── Row components         — UserMessageRow, AssistantMessageRow, …
```

Use [`ChatTurnContext`](../src/lib/conversation/chat-turn-context.ts) for stable bindings shared across the assistant subtree (fold controller, copy handler). Pass per-row data (`message`, `index`) as props.

## Route Surfaces

The renderer is no longer only the chat route. Current first-class routes are:

| Route                            | Surface                                   |
| -------------------------------- | ----------------------------------------- |
| `/`                              | New chat hero composer                    |
| `/session/[id]`                  | Full chat thread                          |
| `/gallery`                       | Generated and presented media library     |
| `/usage`                         | Token usage and estimated spend dashboard |
| `/jobs`                          | Jobs board and job detail drawer          |
| `/skills`                        | Skill browse/edit and draft review        |
| `/settings`                      | Direct settings route outside `AppShell`  |
| `/mini` and `/mini/session/[id]` | Compact mini-window chat                  |

Shared shell state is exposed through the `shellStore` facade in [`shell.svelte.ts`](../src/lib/stores/shell.svelte.ts). The facade re-exports the same API while the implementation lives in `src/lib/stores/shell/*.svelte.ts` (chrome, file tree, file preview, focus, panel history, panel navigation, terminal, web context, web tabs, and workspace panel). Route-local state should stay in route components or feature controllers.

## Workspace Panel Pattern

The workspace panel is a session-scoped shell feature, not a route. Keep transition logic in [`workspace-panel-state.ts`](../src/lib/features/workspace/workspace-panel-state.ts); it is pure TypeScript and has direct unit tests. `shell.svelte.ts` adapts those transitions to active-session state, focus requests, history, and Electron IPC.

[`WorkspacePanel.svelte`](../src/lib/features/workspace/components/WorkspacePanel.svelte) owns shell-level surface/mode selection, toolbar state, and leave-guard coordination. Keep individual surface DOM and lifecycle code in its owner:

- `WorkspaceWebSurface.svelte` owns Electron `<webview>` events, navigation, and page capture.
- `WorkspaceFileSurface.svelte` composes file preview/editing and reports active editor state; `FilePreview.svelte` owns external-change resolution and full-page diff mode.
- `GitChangesBrowser.svelte` owns change selection; `GitDiffView.svelte` renders the selected diff.
- `TerminalPanel.svelte` owns terminal start/focus behavior and `TerminalInstance.svelte` owns terminal rendering.

When an action can replace an open file, call `shellStore.openFilePreviewForActive()` and await its boolean result. A false result means the registered dirty-editor leave guard rejected navigation; callers must keep their own UI open rather than assuming the path changed.

## Error taxonomy

| Level       | When                             | UI                                                              |
| ----------- | -------------------------------- | --------------------------------------------------------------- |
| Connection  | CometMind unreachable            | Activity toast (`CometMind disconnected`); background reconnect |
| Route       | SvelteKit load failure           | `+error.svelte`                                                 |
| Recoverable | Send failed, session load failed | `ErrorBanner` inline in view                                    |
| Inline      | Field validation                 | Adjacent to the control                                         |

Connection loss is a toast, not a blocking overlay. Recoverable errors use `role="alert"` on a dismissible banner.

## Feature layout

Feature UI lives under `src/lib/features/{feature}/`. Each feature keeps `components/` for Svelte views and sibling `*.svelte.ts` controllers. `src/lib/components/` is only shared UI primitives (markdown, tooltips, confirm dialogs, thinking indicator, error banner, and similar cross-feature widgets).

| Feature      | Owns                                                    |
| ------------ | ------------------------------------------------------- |
| `chat`       | Thread, rows, assistant stack, flight                   |
| `composer`   | Composer, slash/mention menus, attachments              |
| `gallery`    | Gallery page and cards                                  |
| `inbox`      | Inbox drawer                                            |
| `jobs`       | Jobs board and job detail                               |
| `onboarding` | Setup wizard                                            |
| `settings`   | Settings panels, providers, MCP, model roles            |
| `shell`      | App shell chrome, intro, toasts, runtime overlay        |
| `sidebar`    | Session sidebar                                         |
| `skills`     | Skills page and slash-command catalog                   |
| `usage`      | Usage dashboard                                         |
| `workspace`  | Workspace panel, file tree, preview, git diff, terminal |

## Styling

- Colors and spacing: `var(--*)` tokens in [`app.css`](../src/app.css)
- Semantic status colors: `--status-success`, `--status-warning`, `--status-error`
- No raw hex in `.svelte` or `.css` files outside `app.css`. `scripts/lint-colors.mjs` fails the build on `#rgb` / `#rrggbb`. Add a token in `app.css` and reference `var(--*)`.
- Shared chat row chrome: [`ThreadRow.svelte`](../src/lib/features/chat/components/ThreadRow.svelte)
- Jobs, file, and settings panels should reuse global panel/card tokens rather than introducing route-local color systems

## Testing

- Pure logic: `*.test.ts` in `node` environment
- Components: `*.svelte.test.ts` in `jsdom` with `@testing-library/svelte`
- Storybook is available for isolated component work via `pnpm run storybook`
- Do not test generated OpenAPI client files

## Import boundaries

- `components/` must not import from `electron/`
- `conversation/*.ts` (non-`.svelte.ts`) must not import `.svelte` files
- Generated code under `generated/` is read-only
- API data flows through `src/lib/client/cometmind.ts`; components should not import generated endpoint functions directly
- Electron IPC remains optional in renderer code so Vite/browser dev contexts can still render
