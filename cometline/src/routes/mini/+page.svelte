<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let error = $state('');

	onMount(() => {
		void (async () => {
			try {
				const { ensureMiniWindowSession } = await import('$lib/mini-window-session');
				const sessionId = await ensureMiniWindowSession();
				await goto(`/mini/session/${sessionId}`, { replaceState: true });
			} catch (err) {
				error = err instanceof Error ? err.message : 'Failed to open mini chat';
			}
		})();
	});
</script>

<div class="mini-loading-shell">
	<div class="mini-loading-card">
		<p>{error || 'Opening mini chat...'}</p>
	</div>
</div>

<style>
	.mini-loading-shell {
		display: grid;
		place-items: center;
		min-height: 100vh;
		padding: 24px;
		background: var(--app-bg);
		color: var(--text-main);
		-webkit-app-region: drag;
	}

	.mini-loading-card {
		width: min(100%, 320px);
		padding: 18px 20px;
		border-radius: 16px;
		background: color-mix(in srgb, var(--panel-bg) 90%, transparent);
		border: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
		box-shadow: 0 18px 48px rgba(0, 0, 0, 0.22);
	}

	p {
		margin: 0;
		font-size: 14px;
		line-height: 1.5;
	}
</style>
