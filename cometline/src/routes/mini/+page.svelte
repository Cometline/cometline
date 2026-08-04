<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import ThinkingIndicator from '$lib/components/ThinkingIndicator.svelte';

	const MIN_LOADING_MS = 320;
	const LOADING_FADE_MS = 160;

	let error = $state('');

	onMount(() => {
		const startedAt = performance.now();
		void (async () => {
			try {
				const { ensureMiniWindowSession } = await import('$lib/mini-window-session');
				const sessionId = await ensureMiniWindowSession();
				const remaining = Math.max(0, MIN_LOADING_MS - (performance.now() - startedAt));
				if (remaining > 0) await new Promise((resolve) => setTimeout(resolve, remaining));
				await goto(`/mini/session/${sessionId}`, { replaceState: true });
			} catch (err) {
				error = err instanceof Error ? err.message : 'Failed to open mini chat';
			}
		})();
	});
</script>

{#if error}
	<div class="mini-loading-error" role="alert">{error}</div>
{:else}
	<div class="mini-loading-overlay" transition:fade={{ duration: LOADING_FADE_MS }}>
		<ThinkingIndicator size={28} label="Opening mini chat" />
	</div>
{/if}

<style>
	.mini-loading-overlay {
		position: absolute;
		inset: 0;
		display: grid;
		place-items: center;
		padding: 24px;
		z-index: 90;
		background: color-mix(in srgb, var(--app-bg) 38%, transparent);
		backdrop-filter: blur(10px);
		-webkit-backdrop-filter: blur(10px);
		-webkit-app-region: no-drag;
	}

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
