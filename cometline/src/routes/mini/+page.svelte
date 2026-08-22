<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { miniShellStore } from '$lib/stores/mini-shell.svelte';

	let error = $state('');

	async function openMiniWindow() {
		miniShellStore.startOpening();
		try {
			const { ensureMiniWindowSession } = await import('$lib/mini-window-session');
			const sessionId = await ensureMiniWindowSession();
			await goto(`/mini/session/${sessionId}`, { replaceState: true });
		} catch (err) {
			miniShellStore.resetOpening();
			error = err instanceof Error ? err.message : 'Failed to open mini chat';
		}
	}

	onMount(() => {
		void openMiniWindow();
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
