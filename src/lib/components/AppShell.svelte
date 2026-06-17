<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import IntroAnimation from './IntroAnimation.svelte';
	import UpdateButton from './UpdateButton.svelte';
	import WebPanel from './WebPanel.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { startNewChat } from '$lib/actions/new-chat';
	import { navigateAdjacentSession } from '$lib/actions/navigate-adjacent-session';
	import { narrowViewportQuery, subscribeNarrowViewport } from '$lib/layout/narrow-viewport';
	import { matchesShortcut } from '$lib/keyboard-shortcuts';

	const FALLBACK_SIDEBAR_DURATION = 240;

	let {
		children,
		workspacePath = '/'
	}: { children: import('svelte').Snippet; workspacePath?: string } = $props();

	let sidebarRef = $state<{ focusSearch: () => void } | null>(null);

	let activeSessionId = $derived(sessionStore.current?.id ?? null);

	$effect(() => {
		window.electronAPI?.setSessionNavigationSuspended?.(shellStore.settingsOpen);
	});

	$effect(() => {
		activeSessionId;
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

	onMount(() => {
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
				shellStore.toggleSidebar();
				return;
			}
			if (matchesShortcut(event, shortcuts.toggleWebPanel)) {
				event.preventDefault();
				shellStore.toggleWebPanel();
				return;
			}
			if (matchesShortcut(event, shortcuts.openWebPanel)) {
				event.preventDefault();
				shellStore.openWebPanelFromShortcut();
				return;
			}
			if (matchesShortcut(event, shortcuts.openSettings)) {
				event.preventDefault();
				shellStore.openSettings();
				return;
			}
			if (matchesShortcut(event, shortcuts.newChat)) {
				event.preventDefault();
				startNewChat();
				return;
			}
			if (matchesShortcut(event, shortcuts.focusSearch)) {
				event.preventDefault();
				shellStore.openSidebar();
				sidebarRef?.focusSearch();
				return;
			}
			if (shellStore.settingsOpen) return;
			if (matchesShortcut(event, shortcuts.previousSession)) {
				event.preventDefault();
				navigateAdjacentSession('prev');
				return;
			}
			if (matchesShortcut(event, shortcuts.nextSession)) {
				event.preventDefault();
				navigateAdjacentSession('next');
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

		return () => {
			unsubscribeNarrowViewport();
			window.removeEventListener('keydown', onKeydown, true);
			unsubscribeNavigate?.();
			unsubscribeCloseWebPanel?.();
			unsubscribeToggleWebPanel?.();
			unsubscribeOpenWebPanel?.();
			unsubscribeFullScreen?.();
			document.removeEventListener('fullscreenchange', onDomFullScreenChange);
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

	function handleMainMouseDown() {
		shellStore.setFocusedPane('chat');
	}
</script>

<div
	class="app-shell"
	class:sidebar-collapsed={!shellStore.sidebarOpen}
	class:is-fullscreen={shellStore.fullscreen}
>
	<Sidebar bind:this={sidebarRef} {workspacePath} collapsed={!shellStore.sidebarOpen} />
	<div class="content-row">
		<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
		<main
			class="main content-panel-surface max-[900px]:shadow-none"
			class:pane-focus-active={shellStore.focusedPane === 'chat'}
			onmousedown={handleMainMouseDown}
		>
			{@render children()}
			<RuntimeOverlay />
		</main>
		<WebPanel />
	</div>
	<SettingsModal />
	<UpdateButton />
	{#if shellStore.introOpen}
		<IntroAnimation />
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
