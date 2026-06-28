<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import ChatView from '$lib/components/ChatView.svelte';

	let sessionId = $derived(page.params.id ?? '');
	let resolvedSessionId = $state('');
	let resolvingRun = 0;
	let error = $state('');

	async function resolveSession(id: string, run: number) {
		try {
			const { ensureMiniWindowSession } = await import('$lib/mini-window-session');
			const ensuredSessionId = await ensureMiniWindowSession(id);
			if (run !== resolvingRun) return;
			if (ensuredSessionId !== id) {
				await goto(`/mini/session/${ensuredSessionId}`, { replaceState: true });
				return;
			}
			resolvedSessionId = ensuredSessionId;
			error = '';
		} catch (err) {
			if (run !== resolvingRun) return;
			error = err instanceof Error ? err.message : 'Failed to open mini chat';
			resolvedSessionId = '';
		}
	}

	$effect(() => {
		if (!sessionId) return;
		resolvedSessionId = '';
		error = '';
		const run = ++resolvingRun;
		void resolveSession(sessionId, run);
	});
</script>

{#if resolvedSessionId}
	<ChatView sessionId={resolvedSessionId} compact />
{:else}
	<div class="mini-session-loading">
		<p>{error || 'Opening mini chat...'}</p>
	</div>
{/if}

<style>
	.mini-session-loading {
		display: grid;
		place-items: center;
		min-height: 100vh;
		padding: 24px;
		background: var(--app-bg);
		color: var(--text-main);
		-webkit-app-region: drag;
	}

	p {
		margin: 0;
		font-size: 14px;
		line-height: 1.5;
	}
</style>
