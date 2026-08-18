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

Heavy UI features use a `create…Controller(deps)` factory that accepts getter callbacks (mirrors [`conversation-controller.ts`](../src/lib/conversation/conversation-controller.ts) and chat thread controllers):

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
├── createThreadScroll     — scroll anchoring
├── createThreadClocks     — copy feedback, memory cycle tick
├── thread-visibility.ts   — pure show/hide predicates
└── Row components         — UserMessageRow, AssistantMessageRow, …
```

Use [`ChatTurnContext`](../src/lib/conversation/chat-turn-context.ts) for stable bindings shared across the assistant subtree (fold controller, copy handler). Pass per-row data (`message`, `index`) as props.

## Route Surfaces

The renderer is no longer only the chat route. Current first-class routes are:

| Route | Surface |
| --- | --- |
| `/` | New chat hero composer |
| `/session/[id]` | Full chat thread |
| `/gallery` | Generated and presented media library |
| `/jobs` | Jobs board and job detail drawer |
| `/skill-drafts` | Skill draft review and promotion |
| `/settings` | Direct settings route outside `AppShell` |
| `/mini` and `/mini/session/[id]` | Compact mini-window chat |

Shared shell state lives in `shell.svelte.ts`; route-local state should stay in route components or feature controllers.

## Workspace Panel Pattern

The workspace panel is a session-scoped shell feature, not a route. Keep transition logic in `src/lib/workspace/workspace-panel-state.ts`; it is pure TypeScript and has direct unit tests. `shell.svelte.ts` adapts those transitions to active-session state, focus requests, history, and Electron IPC.

`WorkspacePanel.svelte` owns shell-level surface/mode selection, toolbar state, and leave-guard coordination. Keep individual surface DOM and lifecycle code in its owner:

- `WorkspaceWebSurface.svelte` owns Electron `<webview>` events, navigation, and page capture.
- `WorkspaceFileSurface.svelte` composes file preview/editing and reports active editor state; `FilePreview.svelte` owns external-change resolution and full-page diff mode.
- `GitChangesBrowser.svelte` owns change selection; `GitDiffView.svelte` renders the selected diff.
- `TerminalPanel.svelte` owns terminal start/focus behavior and `TerminalInstance.svelte` owns terminal rendering.

When an action can replace an open file, call `shellStore.openFilePreviewForActive()` and await its boolean result. A false result means the registered dirty-editor leave guard rejected navigation; callers must keep their own UI open rather than assuming the path changed.

## Error taxonomy

| Level       | When                             | UI                                                  |
| ----------- | -------------------------------- | --------------------------------------------------- |
| Fatal       | CometMind unreachable            | `RuntimeOverlay` — blocks interaction, retry action |
| Route       | SvelteKit load failure           | `+error.svelte`                                     |
| Recoverable | Send failed, session load failed | `ErrorBanner` inline in view                        |
| Inline      | Field validation                 | Adjacent to the control                             |

Fatal errors use `role="alert"`. Recoverable errors use `role="alert"` on a dismissible banner.

## Styling

- Colors and spacing: `var(--*)` tokens in [`app.css`](../src/app.css)
- Semantic status colors: `--status-success`, `--status-warning`, `--status-error`
- No hardcoded hex in components — add a token if a new semantic color is needed
- Shared chat row chrome: [`ThreadRow.svelte`](../src/lib/components/chat/ThreadRow.svelte)
- Jobs/file/settings panels should reuse global panel/card tokens rather than introducing route-local color systems

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
