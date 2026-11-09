<script lang="ts">
	import type { Component } from 'svelte';
	import { settingsStore } from '#lib/stores/settings.svelte.js';
	import type { IntroVariant } from '#lib/types.js';

	let IntroImpl = $state<Component | null>(null);

	function loadVariant(variant: IntroVariant) {
		return variant === 'v2'
			? import('./IntroAnimationV2.svelte')
			: import('./IntroAnimation.svelte');
	}

	$effect(() => {
		const variant = settingsStore.settings.app.introVariant;
		let cancelled = false;
		void loadVariant(variant).then((module) => {
			if (!cancelled) IntroImpl = module.default;
		});
		return () => {
			cancelled = true;
		};
	});
</script>

{#if IntroImpl}
	<IntroImpl />
{/if}
