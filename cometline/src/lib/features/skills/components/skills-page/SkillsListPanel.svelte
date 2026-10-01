<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		countLabel,
		toolbar,
		children,
		footer
	}: {
		title: string;
		countLabel: string;
		toolbar?: Snippet;
		children: Snippet;
		footer?: Snippet;
	} = $props();
</script>

<section class="item-list settings-panel-frame">
	<div class="section-header">
		<h2>{title}</h2>
		<span>{countLabel}</span>
	</div>
	{@render toolbar?.()}
	<div class="item-list-scroll scrollbar-none">
		{@render children()}
	</div>
	{@render footer?.()}
</section>

<style>
	.section-header h2 {
		margin: 0;
		color: var(--text-main);
	}

	.section-header {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		align-items: flex-start;
		margin-bottom: 12px;
	}

	.section-header span {
		font-size: 11px;
		font-weight: 700;
		padding: 3px 7px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.06);
		color: var(--text-muted);
	}

	.item-list {
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.item-list-scroll {
		overflow: auto;
		border: 1px solid var(--border-soft);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.58);
	}
</style>
