<script lang="ts">
	import { onMount } from 'svelte';

	let error = $state('');

	async function openMiniWindow() {
		try {
			const { activateMiniWindow } = await import('$lib/mini-window-session');
			await activateMiniWindow();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to open mini chat';
		}
	}

	onMount(() => {
		void openMiniWindow();
		return window.electronAPI?.onMiniWindowActivated?.(() => {
			void openMiniWindow();
		});
	});
</script>

{#if error}
	<div class="mini-loading-error" role="alert">{error}</div>
{/if}

<style>
	.mini-loading-error {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 24px;
		color: var(--status-error);
		text-align: center;
		-webkit-app-region: no-drag;
	}
</style>
