<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import IntroAnimation from './IntroAnimation.svelte';
	import SetupWizard from './onboarding/SetupWizard.svelte';
	import UpdateButton from './UpdateButton.svelte';
	import WebPanel from './WebPanel.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { startNewChat } from '$lib/actions/new-chat';
	import { navigateAdjacentSession } from '$lib/actions/navigate-adjacent-session';
	import { narrowViewportQuery, subscribeNarrowViewport } from '$lib/layout/narrow-viewport';
	import { matchesShortcut, type ShortcutAction } from '$lib/keyboard-shortcuts';

	const FALLBACK_SIDEBAR_DURATION = 240;

	let { children }: { children: import('svelte').Snippet } = $props();

	let sidebarRef = $state<{ focusSearch: () => void } | null>(null);
	let contentRowRef = $state<HTMLDivElement | null>(null);

	let activeSessionId = $derived(sessionStore.current?.id ?? null);

	$effect(() => {
		window.electronAPI?.setSessionNavigationSuspended?.(shellStore.settingsOpen);
	});

	$effect(() => {
		void activeSessionId;
		shellStore.onActiveSessionChange();
	});

	function isCloseWebPanelShortcut(event: KeyboardEvent) {
		return (
			event.metaKey &&
			!event.ctrlKey &&
			!event.altKey &&
			!event.shiftKey &&
			event.key.toLowerCase() === 'w'
		);
	}

	// Single source of truth for what each global shortcut does, so it behaves
	// identically whether the key arrives via DOM keydown (renderer focused) or
	// via IPC forwarded from the webview guest (web panel focused).
	function runShortcutAction(action: ShortcutAction) {
		switch (action) {
			case 'toggleSidebar':
				shellStore.toggleSidebar();
				return;
			case 'toggleWebPanel':
				shellStore.toggleWebPanel();
				return;
			case 'openWebPanel':
				shellStore.openWebPanelFromShortcut();
				return;
			case 'openSettings':
				shellStore.openSettings();
				return;
			case 'newChat':
				startNewChat();
				return;
			case 'focusSearch':
				shellStore.openSidebar();
				sidebarRef?.focusSearch();
				return;
			case 'previousSession':
				if (shellStore.settingsOpen) return;
				navigateAdjacentSession('prev');
				return;
			case 'nextSession':
				if (shellStore.settingsOpen) return;
				navigateAdjacentSession('next');
				return;
		}
	}

	onMount(() => {
		let resizeObserver: ResizeObserver | null = null;

		if (narrowViewportQuery().matches) {
			shellStore.closeSidebar();
		}

		const unsubscribeNarrowViewport = subscribeNarrowViewport((narrow) => {
			if (narrow) {
				shellStore.closeSidebar();
			}
		});

		function onKeydown(event: KeyboardEvent) {
			const shortcuts = settingsStore.settings.shortcuts;

			if (isCloseWebPanelShortcut(event)) {
				if (shellStore.webPanelOpen) {
					event.preventDefault();
					shellStore.closeWebPanel();
				}
				return;
			}
			if (shellStore.webPanelOpen && event.key === 'Escape' && !shellStore.settingsOpen) {
				event.preventDefault();
				shellStore.closeWebPanel();
				return;
			}
			if (matchesShortcut(event, shortcuts.closeSettings) && shellStore.settingsOpen) {
				event.preventDefault();
				shellStore.closeSettings();
				return;
			}
			if (matchesShortcut(event, shortcuts.toggleSidebar)) {
				event.preventDefault();
				runShortcutAction('toggleSidebar');
				return;
			}
			if (matchesShortcut(event, shortcuts.toggleWebPanel)) {
				event.preventDefault();
				runShortcutAction('toggleWebPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openWebPanel)) {
				event.preventDefault();
				runShortcutAction('openWebPanel');
				return;
			}
			if (matchesShortcut(event, shortcuts.openSettings)) {
				event.preventDefault();
				runShortcutAction('openSettings');
				return;
			}
			if (matchesShortcut(event, shortcuts.newChat)) {
				event.preventDefault();
				runShortcutAction('newChat');
				return;
			}
			if (matchesShortcut(event, shortcuts.focusSearch)) {
				event.preventDefault();
				runShortcutAction('focusSearch');
				return;
			}
			if (shellStore.settingsOpen) return;
			if (matchesShortcut(event, shortcuts.previousSession)) {
				event.preventDefault();
				runShortcutAction('previousSession');
				return;
			}
			if (matchesShortcut(event, shortcuts.nextSession)) {
				event.preventDefault();
				runShortcutAction('nextSession');
				return;
			}
		}

		window.addEventListener('keydown', onKeydown, true);

		const unsubscribeNavigate = window.electronAPI?.onNavigateSession?.((direction) => {
			if (shellStore.settingsOpen) return;
			navigateAdjacentSession(direction);
		});

		const unsubscribeCloseWebPanel = window.electronAPI?.onCloseWebPanel?.(() => {
			shellStore.closeWebPanel();
		});

		const unsubscribeToggleWebPanel = window.electronAPI?.onToggleWebPanel?.(() => {
			if (shellStore.settingsOpen) return;
			shellStore.toggleWebPanel();
		});

		const unsubscribeOpenWebPanel = window.electronAPI?.onOpenWebPanel?.(() => {
			if (shellStore.settingsOpen) return;
			shellStore.openWebPanelFromShortcut();
		});

		// Shortcuts forwarded from the webview guest (web panel focused). Run the
		// same effects as the DOM keydown dispatcher above.
		const unsubscribeShortcutAction = window.electronAPI?.onShortcutAction?.((action) => {
			runShortcutAction(action);
		});

		function updateFullScreen(isFullScreen: boolean) {
			if (import.meta.env.DEV) {
				console.log('[AppShell] fullscreen state:', isFullScreen);
			}
			shellStore.setFullscreen(isFullScreen);
		}
		void window.electronAPI?.getFullScreen?.().then(updateFullScreen);
		const unsubscribeFullScreen = window.electronAPI?.onFullScreenChange?.(updateFullScreen);

		function onDomFullScreenChange() {
			updateFullScreen(Boolean(document.fullscreenElement));
		}
		document.addEventListener('fullscreenchange', onDomFullScreenChange);

		// Keep a custom panel width within bounds when the window is resized so a
		// previously-saved large width can't exceed the viewport.
		function clampPanelWidthToLayout() {
			if (!shellStore.webPanelOpen) return;
			const raw = document.documentElement.style.getPropertyValue('--web-panel-width').trim();
			const current = raw.endsWith('px') ? Number.parseFloat(raw) : currentPanelWidth();
			const px = Number.isFinite(current) ? current : currentPanelWidth();
			if (!Number.isFinite(px)) return;
			const clamped = clampPanelWidth(px);
			if (clamped !== px) {
				document.documentElement.style.setProperty('--web-panel-width', `${clamped}px`);
			}
		}

		function onWindowResize() {
			clampPanelWidthToLayout();
		}
		window.addEventListener('resize', onWindowResize);

		if (contentRowRef) {
			resizeObserver = new ResizeObserver(() => {
				clampPanelWidthToLayout();
			});
			resizeObserver.observe(contentRowRef);
		}

		return () => {
			unsubscribeNarrowViewport();
			window.removeEventListener('keydown', onKeydown, true);
			unsubscribeNavigate?.();
			unsubscribeCloseWebPanel?.();
			unsubscribeToggleWebPanel?.();
			unsubscribeOpenWebPanel?.();
			unsubscribeShortcutAction?.();
			unsubscribeFullScreen?.();
			document.removeEventListener('fullscreenchange', onDomFullScreenChange);
			window.removeEventListener('resize', onWindowResize);
			resizeObserver?.disconnect();
		};
	});

	function parseDuration(value: string) {
		const trimmed = value.trim();
		if (!trimmed) return FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('ms'))
			return Number(trimmed.slice(0, -2)) || FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('s')) return (Number(trimmed.slice(0, -1)) || 0) * 1000;
		return Number(trimmed) || FALLBACK_SIDEBAR_DURATION;
	}

	function sidebarTransitionDuration() {
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return 0;
		return parseDuration(
			getComputedStyle(document.documentElement).getPropertyValue('--duration-fast')
		);
	}

	$effect(() => {
		window.electronAPI?.setSidebarOpen?.({
			open: shellStore.sidebarOpen,
			duration: sidebarTransitionDuration()
		});
	});

	function handleMainMouseDown(event: MouseEvent) {
		shellStore.setFocusedPane('chat');
		if (event.button !== 0) return;
		const target = event.target;
		if (!(target instanceof HTMLElement)) {
			shellStore.requestComposerFocus();
			return;
		}
		if (
			target.closest(
				'button, a, input, textarea, select, [role="button"], [contenteditable="true"]'
			)
		) {
			return;
		}
		const selection = window.getSelection();
		if (selection && !selection.isCollapsed) return;
		shellStore.requestComposerFocus();
	}

	// --- Web/file panel resize ---------------------------------------------
	const PANEL_MIN_WIDTH = 320;
	// Keep the chat pane wide enough for avatar + bubble layout even when the
	// sidebar and web panel are both visible.
	const MAIN_PANEL_MIN_WIDTH = 720;
	function panelMaxWidth() {
		const rowWidth = contentRowRef?.clientWidth ?? window.innerWidth;
		return Math.max(PANEL_MIN_WIDTH, rowWidth - MAIN_PANEL_MIN_WIDTH);
	}
	function currentPanelWidth() {
		const raw = getComputedStyle(document.documentElement)
			.getPropertyValue('--web-panel-width')
			.trim();
		const px = Number.parseFloat(raw);
		if (raw.endsWith('px') && Number.isFinite(px)) return px;
		// Fallback for vw/default: measure the rendered panel inner element.
		const inner = document.querySelector<HTMLElement>('.web-panel-inner');
		if (inner) return inner.getBoundingClientRect().width;
		return Math.round(window.innerWidth * 0.5);
	}

	let resizing = $state(false);
	let resizeStartX = 0;
	let resizeStartWidth = 0;

	function clampPanelWidth(width: number) {
		return Math.min(Math.max(width, PANEL_MIN_WIDTH), panelMaxWidth());
	}

	function onResizePointerDown(event: PointerEvent) {
		if (event.button !== 0) return;
		event.preventDefault();
		resizing = true;
		resizeStartX = event.clientX;
		resizeStartWidth = currentPanelWidth();
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		document.body.classList.add('panel-resizing');
	}

	function onResizePointerMove(event: PointerEvent) {
		if (!resizing) return;
		// Panel is on the right: dragging left (negative delta) grows it.
		const next = clampPanelWidth(resizeStartWidth - (event.clientX - resizeStartX));
		document.documentElement.style.setProperty('--web-panel-width', `${next}px`);
	}

	function endResize(event: PointerEvent) {
		if (!resizing) return;
		resizing = false;
		const target = event.currentTarget as HTMLElement;
		if (target.hasPointerCapture(event.pointerId)) {
			target.releasePointerCapture(event.pointerId);
		}
		document.body.classList.remove('panel-resizing');
		void settingsStore.saveWebPanelWidth(currentPanelWidth());
	}

	function onResizeKeydown(event: KeyboardEvent) {
		const step = event.shiftKey ? 64 : 16;
		let next: number | null = null;
		if (event.key === 'ArrowLeft') next = currentPanelWidth() + step;
		else if (event.key === 'ArrowRight') next = currentPanelWidth() - step;
		if (next === null) return;
		event.preventDefault();
		const clamped = clampPanelWidth(next);
		document.documentElement.style.setProperty('--web-panel-width', `${clamped}px`);
		void settingsStore.saveWebPanelWidth(clamped);
	}
</script>

<div
	class="app-shell"
	class:sidebar-collapsed={!shellStore.sidebarOpen}
	class:is-fullscreen={shellStore.fullscreen}
>
	<Sidebar bind:this={sidebarRef} collapsed={!shellStore.sidebarOpen} />
	<div class="content-row" bind:this={contentRowRef}>
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<main
			class="main content-panel-surface max-[900px]:shadow-none"
			class:pane-focus-active={shellStore.focusedPane === 'chat' && shellStore.webPanelOpen}
			onmousedown={handleMainMouseDown}
		>
			{@render children()}
			<RuntimeOverlay />
		</main>
		{#if shellStore.webPanelOpen}
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<div
				class="panel-resizer"
				class:resizing
				role="separator"
				aria-orientation="vertical"
				aria-label="Resize web panel"
				tabindex="0"
				onpointerdown={onResizePointerDown}
				onpointermove={onResizePointerMove}
				onpointerup={endResize}
				onpointercancel={endResize}
				onkeydown={onResizeKeydown}
			></div>
		{/if}
		<WebPanel />
	</div>
	<SettingsModal />
	<UpdateButton />
	{#if shellStore.introOpen}
		<IntroAnimation />
	{/if}
	{#if shellStore.setupOpen}
		<SetupWizard />
	{/if}
</div>

<style>
	.app-shell {
		--active-sidebar-width: var(--sidebar-width);
		display: flex;
		width: 100vw;
		height: 100vh;
		background: var(--shell-canvas-bg);
		box-sizing: border-box;
	}

	.app-shell.sidebar-collapsed {
		--active-sidebar-width: 0px;
	}

	.app-shell.is-fullscreen {
		--traffic-light-gutter: 0px;
	}

	.content-row {
		flex: 1;
		min-width: 0;
		display: flex;
		position: relative;
	}

	.main {
		flex: 1 1 0;
		min-width: 0;
		display: flex;
		flex-direction: column;
		position: relative;
		z-index: 1;
		margin: var(--content-panel-inset);
		margin-left: calc(-1 * var(--content-panel-overlap));
		overflow: hidden;
		transition:
			margin-left var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.app-shell.sidebar-collapsed .main {
		margin-left: var(--content-panel-inset);
	}

	.panel-resizer {
		flex: 0 0 auto;
		width: 8px;
		margin: 0 -3px;
		z-index: 2;
		cursor: col-resize;
		align-self: stretch;
		background: transparent;
		position: relative;
	}

	.panel-resizer::before {
		content: '';
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		width: 2px;
		height: 36px;
		border-radius: 999px;
		background: var(--border-subtle, rgba(148, 163, 184, 0.4));
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	.panel-resizer:hover::before,
	.panel-resizer:focus-visible::before,
	.panel-resizer.resizing::before {
		opacity: 1;
	}

	.panel-resizer:focus-visible {
		outline: none;
	}

	@media (prefers-reduced-motion: reduce) {
		.main {
			transition: none;
		}
	}

	@media (max-width: 900px) {
		.app-shell {
			--active-sidebar-width: 0px;
			background: var(--app-bg);
		}

		.content-row {
			display: flex;
		}

		.main {
			margin: 0;
			border: none;
			border-radius: 0;
			background: transparent;
			box-shadow: none;
		}

		.panel-resizer {
			display: none;
		}

		.app-shell:not(.sidebar-collapsed) :global(.sidebar:not(.collapsed)) {
			position: fixed;
			inset: 0;
			width: 100vw;
			height: 100vh;
			z-index: 50;
			flex-shrink: 0;
			border-right: none;
		}
	}
</style>
