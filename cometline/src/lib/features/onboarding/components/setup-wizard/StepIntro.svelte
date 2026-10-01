<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<p class="step-intro">{@render children()}</p>

<style>
	.step-intro {
		margin: 0 0 16px;
		font-size: 13px;
		line-height: 1.5;
		color: var(--text-muted);
	}
</style>
