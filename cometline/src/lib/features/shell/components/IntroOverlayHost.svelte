<script lang="ts">
	import { shellStore } from '#lib/stores/shell.svelte.js';
	import { createIntroLazyLoader } from '#lib/features/shell/intro-lazy.svelte.js';

	const intro = createIntroLazyLoader();

	$effect(() => {
		if (!shellStore.introOpen || intro.Intro) return;
		void intro.loadIntroAnimation();
	});
</script>

<div class="intro-root-overlay" aria-hidden={!shellStore.introOpen}>
	{#if shellStore.introOpen}
		{#if intro.Intro}
			<intro.Intro />
		{:else}
			<div class="intro-root-placeholder" aria-label="Loading introduction"></div>
		{/if}
	{/if}
</div>

<style>
	.intro-root-overlay {
		position: fixed;
		inset: 0;
		z-index: 100000;
		pointer-events: none;
	}

	.intro-root-overlay :global(.intro),
	.intro-root-overlay :global(.intro-v2) {
		pointer-events: auto;
		z-index: 100000;
	}

	.intro-root-placeholder {
		position: absolute;
		inset: 0;
		background: var(--intro-bg, var(--color-fafafa));
	}
</style>
