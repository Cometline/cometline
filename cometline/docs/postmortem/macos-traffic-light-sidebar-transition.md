# macOS traffic-light sidebar transition

**Date:** 2026-06-14  
**Components:** `electron/main.cjs`, `electron/preload.cjs`, `AppShell.svelte`, `Sidebar.svelte`, `app.css`, `app.d.ts`

## Symptom

The macOS traffic-light buttons did not move when the sidebar opened or closed. The sidebar width animated in the renderer, but the native window controls stayed at the default `hiddenInset` position, so the app did not feel like one coordinated native surface.

## Root cause

Traffic lights are not DOM elements. On macOS, Electron exposes them as native `NSWindow` controls above the WebView. CSS transitions cannot animate them.

The repo already had a partial bridge:

```javascript
// electron/preload.cjs
setSidebarOpen: (open) => ipcRenderer.send('cometline:set-sidebar-open', open);
```

But three pieces were missing:

- `electron/main.cjs` had no `ipcMain.on('cometline:set-sidebar-open', ...)` handler.
- `AppShell.svelte` never called `window.electronAPI.setSidebarOpen` when `shellStore.sidebarOpen` changed.
- `app.d.ts` did not type the API, so future renderer calls would be invisible to TypeScript.

The old sidebar also used `pl-[72px]` as a hardcoded traffic-light gutter, with no shared token and no `-webkit-app-region` drag/no-drag model.

## Design applied

### 1. Renderer is the conductor

`AppShell.svelte` owns the sidebar-open CSS class, so it also reports the sidebar state to Electron:

```typescript
$effect(() => {
	window.electronAPI?.setSidebarOpen?.({
		open: shellStore.sidebarOpen,
		duration: sidebarTransitionDuration()
	});
});
```

`sidebarTransitionDuration()` reads `--duration-fast` from `app.css`, so native button motion stays aligned with the CSS sidebar width transition. If `prefers-reduced-motion: reduce` is active, duration is sent as `0`.

### 2. Main process tweens native controls

`electron/main.cjs` handles `cometline:set-sidebar-open` and calls `BrowserWindow.setWindowButtonPosition(...)` in a small timer loop. Electron sets the position instantly; it does not provide native tweening, so the app owns interpolation.

Current constants:

```javascript
const WINDOW_BUTTON_OPEN_POSITION = { x: 16, y: 17 };
const WINDOW_BUTTON_CLOSED_POSITION = { x: 17, y: 17 };
const WINDOW_BUTTON_DEFAULT_DURATION = 240;
const sidebarChromeEase = cubicBezier(0.22, 1, 0.36, 1);
```

The easing matches CSS `--ease-smooth`. The default duration matches CSS `--duration-fast`.

The `y` is derived, not guessed. The sidebar titlebar row sits flush against the window top, the search input is 28px tall and centered in the 48px row, so its center sits at `y = 10 + 14 = 24` from the window top. A traffic light is ~14px tall, so `y = 24 - 7 = 17` lines the button center up with the search-bar center. If the titlebar height, input height, or row padding changes, recompute this.

Because `setWindowButtonPosition` only works on frameless windows, `titleBarStyle` must be `'hidden'` (not `'hiddenInset'`); otherwise Electron silently ignores custom traffic-light positions.

### 3. macOS window chrome opts into native material

`BrowserWindow` uses `titleBarStyle: 'hidden'` (required for `setWindowButtonPosition` to work), then adds macOS-only window material:

```javascript
...(process.platform === 'darwin'
	? {
			backgroundColor: '#00000000',
			transparent: true,
			vibrancy: 'sidebar',
			visualEffectState: 'active'
		}
	: {}),
```

The CSS `body` background is transparent and `--shell-canvas-bg` is translucent so the native material can show through without changing non-macOS behavior.

### 4. Sidebar chrome uses tokens, not Tailwind literals

Traffic-light spacing now lives in `app.css`:

```css
--titlebar-height: 48px;
--traffic-light-gutter: 78px;
```

`Sidebar.svelte` uses a Heptabase-style `.sidebar-titlebar-row` instead of treating the top controls as a normal sidebar row with `pl-[72px]`. The row is a real 48px titlebar/drag band under the native buttons:

```css
.sidebar-titlebar-row {
	height: var(--titlebar-height);
	display: grid;
	grid-template-columns: minmax(0, 1fr) auto;
	align-items: center;
	gap: 8px;
	padding: 10px 8px;
	-webkit-app-region: drag;
}

.search-field {
	margin-left: var(--traffic-light-gutter);
	transition: margin-left var(--duration-fast) var(--ease-smooth);
}
```

This mirrors the inspected Heptabase pattern: the traffic lights remain native controls, while the renderer provides a fixed-height DOM strip underneath them. The search box and new-chat button are vertically centered in that titlebar strip so they sit on the same visual row as the open-state traffic lights.

The gutter is applied as the search field's left margin rather than row padding, so the empty area under the traffic lights stays inside the draggable titlebar band. Interactive controls inside that row use `.no-drag`, and global interactive elements are also marked `-webkit-app-region: no-drag`.

`AppShell.svelte` does **not** pad the top/left/bottom of the window; the inset is applied to the floating `.main` panel instead. This lets the sidebar extend flush to the window edges, which keeps the titlebar row at a predictable `y = 0` and makes the traffic-light math reliable in both wide and narrow layouts:

```css
.app-shell {
	display: flex;
	/* no padding here */
}

.main {
	margin: var(--content-panel-inset);
	margin-left: calc(-1 * var(--content-panel-overlap));
}
```

### 5. Fullscreen reclaims the traffic-light gutter

In macOS fullscreen, the native traffic lights are hidden, so the 78px gutter that keeps the search bar clear of them is wasted space. The main process reports fullscreen transitions and the renderer collapses the gutter so the search bar slides left to fill it.

Main process broadcasts the state:

```javascript
function sendFullScreenState() {
	if (!mainWindow || mainWindow.isDestroyed()) return;
	mainWindow.webContents.send('cometline:fullscreen-changed', mainWindow.isFullScreen());
}

mainWindow.on('enter-full-screen', sendFullScreenState);
mainWindow.on('leave-full-screen', sendFullScreenState);
// also fired once on ready-to-show for the initial state
```

`preload.cjs` exposes `onFullScreenChange(callback)` (returns an unsubscribe) and `getFullScreen()` for the initial state. `AppShell.svelte` subscribes in `onMount`, stores the flag in `shellStore.fullscreen`, and toggles an `is-fullscreen` class. The class overrides the gutter token, and the search field animates its `margin-left` to follow. A `fullscreenchange` DOM listener is also kept as a fallback.

```css
.app-shell.is-fullscreen {
	--traffic-light-gutter: 0px;
}

.sidebar-titlebar-row {
	/* ... */
	transition: padding-left var(--duration-fast) var(--ease-smooth);
}
```

Because `--traffic-light-gutter` is a custom property, overriding it on `.app-shell` cascades into the scoped `.sidebar-titlebar-row` without coupling the two components.

## If changing the traffic-light layout later

Start here:

| What to change                                 | File                                                                       | Notes                                                                                                  |
| ---------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Native button x/y when sidebar is open         | `electron/main.cjs`                                                        | Edit `WINDOW_BUTTON_OPEN_POSITION`.                                                                    |
| Native button x/y when sidebar is closed       | `electron/main.cjs`                                                        | Edit `WINDOW_BUTTON_CLOSED_POSITION`.                                                                  |
| Native tween duration fallback                 | `electron/main.cjs`                                                        | Edit `WINDOW_BUTTON_DEFAULT_DURATION`, but keep it aligned with CSS unless intentionally different.    |
| Sidebar CSS transition duration                | `app.css`                                                                  | Edit `--duration-fast`; renderer sends this duration to Electron.                                      |
| Sidebar row left padding around traffic lights | `app.css`                                                                  | Edit `--traffic-light-gutter`. This affects DOM content only, not native button position.              |
| Sidebar titlebar row structure                 | `Sidebar.svelte`                                                           | Keep `.sidebar-titlebar-row` as the fixed-height drag band under the native traffic lights.            |
| Drag/no-drag behavior                          | `Sidebar.svelte`, `app.css`                                                | Keep the draggable strip on `.sidebar-titlebar-row`; keep actual controls as `no-drag`.                |
| IPC contract                                   | `electron/preload.cjs`, `app.d.ts`, `AppShell.svelte`, `electron/main.cjs` | Keep payload shape `{ open, duration }` unless all four places are updated together.                   |
| Fullscreen gutter behavior                     | `AppShell.svelte`, `electron/main.cjs`                                     | Edit `.app-shell.is-fullscreen { --traffic-light-gutter }`; main emits `cometline:fullscreen-changed`. |

## How to avoid regressions

- Do not try to style or animate traffic lights with CSS. They are native window controls.

- Do not remove the `AppShell.svelte` `$effect` unless another renderer-side conductor replaces it. The main process cannot infer sidebar layout from the DOM.

- Keep CSS timing and native timing together. Sidebar width uses `--duration-fast` and `--ease-smooth`; native traffic lights use the same duration and cubic-bezier.

- Keep the position values macOS-only. `setWindowButtonPosition` is a macOS API; non-macOS platforms should no-op.

- When moving the top sidebar controls, tune both `--traffic-light-gutter` and the `WINDOW_BUTTON_*_POSITION` constants. One controls DOM spacing; the other controls native button placement.

## Verification

1. macOS desktop app: press `Cmd+B`; sidebar and traffic lights should move in the same 240ms beat.
2. Toggle repeatedly while the transition is in flight; the native tween should cancel and retarget without jumping far behind.
3. Search input and new-chat button in the top sidebar row remain clickable because they are `no-drag`.
4. Dragging empty space in the top sidebar row should move the window.
5. With reduced motion enabled, traffic lights should jump to final position instead of tweening.
6. Enter fullscreen (green button / `Ctrl+Cmd+F`); the search bar should slide left into the freed gutter, and slide back on exit.
7. `pnpm check`, `pnpm build`, and lint should remain clean.
