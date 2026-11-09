<script lang="ts">
	import IntroAnimationV2 from '#lib/features/shell/components/IntroAnimationV2.svelte';
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { onMount } from 'svelte';

	const frozenMs = $derived.by(() => {
		if (typeof window === 'undefined') return null;
		const raw = new URLSearchParams(window.location.search).get('t');
		if (!raw) return null;
		const n = Number(raw);
		return Number.isFinite(n) ? n : null;
	});

	onMount(() => {
		shellStore.openIntro();
	});
</script>

<svelte:head>
	<title>Intro lab — Cometline</title>
</svelte:head>

{#if shellStore.introOpen}
	<IntroAnimationV2 debugFrozenElapsed={frozenMs} />
{/if}

<p class="hint">
	Dev-only preview route. Visit <code>/intro-lab</code> while running <code>pnpm run dev</code>.
</p>

<style>
	.hint {
		position: fixed;
		top: 8px;
		left: 8px;
		z-index: 200;
		margin: 0;
		padding: 6px 10px;
		font-size: 11px;
		border-radius: 8px;
		background: rgba(255, 255, 255, 0.9);
		color: #333;
	}
</style>
