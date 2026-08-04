<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import ChatView from '$lib/components/ChatView.svelte';
	import ThinkingIndicator from '$lib/components/ThinkingIndicator.svelte';
	import { miniShellStore } from '$lib/stores/mini-shell.svelte';

	const MIN_LOADING_MS = 320;

	let sessionId = $derived(page.params.id ?? '');
	let resolvedSessionId = $state('');
	let resolvingRun = 0;
	let resolving = $state(true);
	let showLoading = $state(true);
	let error = $state('');

	async function resolveSession(id: string, run: number) {
		const startedAt = performance.now();
		try {
			const { ensureMiniWindowSession } = await import('$lib/mini-window-session');
			const ensuredSessionId = await ensureMiniWindowSession(id);
			if (run !== resolvingRun) return;
			if (ensuredSessionId !== id) {
				await goto(`/mini/session/${ensuredSessionId}`, { replaceState: true });
				return;
			}
			const remaining = Math.max(0, MIN_LOADING_MS - (performance.now() - startedAt));
			if (remaining > 0) await new Promise((resolve) => setTimeout(resolve, remaining));
			if (run !== resolvingRun) return;
			resolvedSessionId = ensuredSessionId;
			error = '';
			resolving = false;
		} catch (err) {
			if (run !== resolvingRun) return;
			error = err instanceof Error ? err.message : 'Failed to open mini chat';
			resolvedSessionId = '';
			resolving = false;
		}
	}

	$effect(() => {
		if (!sessionId) return;
		const existingSession = untrack(() => Boolean(resolvedSessionId));
		showLoading = !existingSession || miniShellStore.consumeNewSessionRequest(sessionId);
		resolvedSessionId = sessionId;
		error = '';
		resolving = true;
		const run = ++resolvingRun;
		void resolveSession(sessionId, run);
	});
</script>

{#if resolvedSessionId}
	<ChatView sessionId={resolvedSessionId} compact />
{:else if error}
	<div class="mini-session-error" role="alert">{error}</div>
{/if}

{#if showLoading && resolving && !error}
	<div class="mini-loading-overlay">
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
