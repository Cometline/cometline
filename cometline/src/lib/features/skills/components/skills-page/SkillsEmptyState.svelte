<script lang="ts">
	import type { Snippet } from 'svelte';

	let { title, children }: { title: string; children: Snippet } = $props();
</script>

<section class="empty-state settings-panel-frame">
	<h2>{title}</h2>
	<p>{@render children()}</p>
</section>

<style>
	.empty-state {
		width: 100%;
		min-width: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.empty-state h2 {
		margin: 0;
		color: var(--text-main);
	}

	.empty-state p {
		margin: 6px 0 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}
</style>
