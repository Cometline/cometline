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
- Semantic status colors: `--status-success`, `--status-error`
- No hardcoded hex in components — add a token if a new semantic color is needed
- Shared chat row chrome: [`ThreadRow.svelte`](../src/lib/components/chat/ThreadRow.svelte)

## Testing

- Pure logic: `*.test.ts` in `node` environment
- Components: `*.svelte.test.ts` in `jsdom` with `@testing-library/svelte`
- Do not test generated OpenAPI client files

## Import boundaries

- `components/` must not import from `electron/`
- `conversation/*.ts` (non-`.svelte.ts`) must not import `.svelte` files
- Generated code under `generated/` is read-only
