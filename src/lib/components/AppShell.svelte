<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from './Sidebar.svelte';
	import RuntimeOverlay from './RuntimeOverlay.svelte';
	import SettingsModal from './SettingsModal.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';

	let { children, workspacePath = '/' }: { children: import('svelte').Snippet; workspacePath?: string } =
		$props();

	onMount(() => {
		function onKeydown(event: KeyboardEvent) {
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
		}

		window.addEventListener('keydown', onKeydown);
		return () => window.removeEventListener('keydown', onKeydown);
	});
</script>

<div class="app-shell" class:sidebar-collapsed={!shellStore.sidebarOpen}>
	<Sidebar {workspacePath} collapsed={!shellStore.sidebarOpen} />
	<main class="main">
		{@render children()}
		<RuntimeOverlay />
		<SettingsModal />
	</main>
</div>

<style>
	.app-shell {
		--active-sidebar-width: var(--sidebar-width);
		display: flex;
		width: 100vw;
		height: 100vh;
		background: var(--app-bg);
	}

	.app-shell.sidebar-collapsed {
		--active-sidebar-width: 0px;
	}

	.main {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		position: relative;
		overflow: hidden;
	}
</style>
