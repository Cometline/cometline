<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { startNewChat } from '$lib/actions/new-chat';
	import { narrowViewportQuery } from '$lib/layout/narrow-viewport';

	const FALLBACK_SIDEBAR_DURATION = 240;

	let {
		children,
		workspacePath = '/'
	}: { children: import('svelte').Snippet; workspacePath?: string } = $props();

	onMount(() => {
		// Narrow viewports start with the sidebar closed so chat gets full width.
		if (narrowViewportQuery().matches) {
			shellStore.closeSidebar();
		}

		function onKeydown(event: KeyboardEvent) {
			if (event.key === 'Escape' && shellStore.settingsOpen) {
				event.preventDefault();
				shellStore.closeSettings();
				return;
			}

			const command = event.metaKey || event.ctrlKey;
			if (!command) return;
			const key = event.key.toLowerCase();
			if (key === 'b') {
				event.preventDefault();
				shellStore.toggleSidebar();
			}
			if (key === ',') {
				event.preventDefault();
				shellStore.openSettings();
			}
			if (key === 't') {
				event.preventDefault();
				startNewChat();
			}
		}

		window.addEventListener('keydown', onKeydown);

		// macOS hides the traffic lights in fullscreen, so the renderer reclaims
		// the gutter that normally keeps the search bar clear of them. Pull the
		// current state on mount in case the initial push fired before this
		// listener was registered, then subscribe to future changes.
		function updateFullScreen(isFullScreen: boolean) {
			if (import.meta.env.DEV) {
				console.log('[AppShell] fullscreen state:', isFullScreen);
			}
			shellStore.setFullscreen(isFullScreen);
		}
		void window.electronAPI?.getFullScreen?.().then(updateFullScreen);
		const unsubscribeFullScreen = window.electronAPI?.onFullScreenChange?.(updateFullScreen);

		// Fallback in case the main-process push is missed or delayed.
		function onDomFullScreenChange() {
			updateFullScreen(Boolean(document.fullscreenElement));
		}
		document.addEventListener('fullscreenchange', onDomFullScreenChange);

		return () => {
			window.removeEventListener('keydown', onKeydown);
			unsubscribeFullScreen?.();
			document.removeEventListener('fullscreenchange', onDomFullScreenChange);
		};
	});

	function parseDuration(value: string) {
		const trimmed = value.trim();
		if (!trimmed) return FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('ms')) return Number(trimmed.slice(0, -2)) || FALLBACK_SIDEBAR_DURATION;
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
</script>

<div
	class="app-shell"
	class:sidebar-collapsed={!shellStore.sidebarOpen}
	class:is-fullscreen={shellStore.fullscreen}
>
	<Sidebar {workspacePath} collapsed={!shellStore.sidebarOpen} />
	<main class="main shadow max-[900px]:shadow-none">
		{@render children()}
		<RuntimeOverlay />
	</main>
	<SettingsModal />
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

	/* In fullscreen the native traffic lights are hidden, so the search bar can
	   reclaim the gutter that normally keeps it clear of them. */
	.app-shell.is-fullscreen {
		--traffic-light-gutter: 0px;
	}

	.main {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		position: relative;
		z-index: 1;
		margin: var(--content-panel-inset);
		margin-left: calc(-1 * var(--content-panel-overlap));
		overflow: hidden;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-window);
		transition: margin-left var(--duration-fast) var(--ease-smooth);
	}

	.app-shell.sidebar-collapsed .main {
		margin-left: var(--content-panel-inset);
	}

	/* Keep chat full-width; open sidebar becomes a full-window overlay. */
	@media (max-width: 900px) {
		.app-shell {
			--active-sidebar-width: 0px;
			background: var(--app-bg);
		}

		.main {
			margin: 0;
			border: none;
			border-radius: 0;
			background: transparent;
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
