<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import ChatView from '$lib/components/ChatView.svelte';
	import { miniShellStore } from '$lib/stores/mini-shell.svelte';

	let sessionId = $derived(page.params.id ?? '');
	let resolvedSessionId = $state('');
	let resolvingRun = 0;
	let error = $state('');

	async function resolveSession(id: string, run: number, openingRun: number) {
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
			void miniShellStore.finishOpening(openingRun);
		} catch (err) {
			if (run !== resolvingRun) return;
			miniShellStore.resetOpening();
			error = err instanceof Error ? err.message : 'Failed to open mini chat';
			resolvedSessionId = '';
		}
	}

	$effect(() => {
		if (!sessionId) return;
		const existingSession = untrack(() => Boolean(resolvedSessionId));
		const showLoading = !existingSession || miniShellStore.consumeNewSessionRequest(sessionId);
		const openingRun =
			showLoading || miniShellStore.opening ? miniShellStore.ensureOpening() : 0;
		resolvedSessionId = sessionId;
		error = '';
		const run = ++resolvingRun;
		void resolveSession(sessionId, run, openingRun);
	});
</script>

{#if resolvedSessionId}
	<ChatView sessionId={resolvedSessionId} compact />
{:else if error}
	<div class="mini-session-error" role="alert">{error}</div>
{/if}

<style>
	.mini-session-error {
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
