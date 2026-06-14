<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { startNewChat } from '$lib/actions/new-chat';
	import { narrowViewportQuery } from '$lib/layout/narrow-viewport';

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
		return () => {
			window.removeEventListener('keydown', onKeydown);
		};
	});
</script>

<div class="app-shell" class:sidebar-collapsed={!shellStore.sidebarOpen}>
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
		padding: var(--content-panel-inset) var(--content-panel-inset) var(--content-panel-inset) 0;
		box-sizing: border-box;
	}

	.app-shell.sidebar-collapsed {
		--active-sidebar-width: 0px;
		padding-left: var(--content-panel-inset);
	}

	.main {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		position: relative;
		z-index: 1;
		margin-left: calc(-1 * var(--content-panel-overlap));
		overflow: hidden;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-window);
		transition: margin-left var(--duration-fast) var(--ease-smooth);
	}

	.app-shell.sidebar-collapsed .main {
		margin-left: 0;
	}

	/* Keep chat full-width; open sidebar becomes a full-window overlay. */
	@media (max-width: 900px) {
		.app-shell {
			--active-sidebar-width: 0px;
			padding: 0;
			background: var(--app-bg);
		}

		.main {
			margin-left: 0;
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
