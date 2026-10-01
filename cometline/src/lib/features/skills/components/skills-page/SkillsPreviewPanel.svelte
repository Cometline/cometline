<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		loading,
		loadingLabel,
		emptyLabel,
		item,
		path,
		notice = '',
		readonly = false,
		value = $bindable(),
		actions
	}: {
		loading: boolean;
		loadingLabel: string;
		emptyLabel: string;
		item: { name: string; description: string } | null;
		path?: string;
		notice?: string;
		readonly?: boolean;
		value: string;
		actions: Snippet;
	} = $props();
</script>

<section class="item-preview settings-panel-frame">
	{#if loading}
		<p class="page-muted">{loadingLabel}</p>
	{:else if item}
		<header class="preview-header">
			<div>
				<h2>{item.name}</h2>
				<p>{item.description}</p>
				{#if path !== undefined}
					<p class="skill-path">{path}</p>
				{/if}
				{#if notice}
					<p>{notice}</p>
				{/if}
			</div>
			<div class="preview-actions">
				{@render actions()}
			</div>
		</header>
		<textarea class="item-markdown" bind:value spellcheck="false" {readonly}></textarea>
	{:else}
		<p class="page-muted">{emptyLabel}</p>
	{/if}
</section>

<style>
	.preview-header > div:first-child {
		min-width: 0;
	}

	.preview-header h2 {
		margin: 0;
		color: var(--text-main);
	}

	.preview-header p,
	.page-muted {
		margin: 6px 0 0;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.preview-header {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		align-items: flex-start;
		margin-bottom: 12px;
	}

	.item-preview {
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.skill-path {
		word-break: break-all;
		font-family: var(--font-mono, 'SFMono-Regular', ui-monospace, monospace);
		font-size: 11px;
	}

	.preview-actions {
		display: flex;
		gap: 8px;
		flex-shrink: 0;
	}

	.item-markdown {
		margin: 0;
		padding: 12px;
		border-radius: 12px;
		border: 1px solid var(--border-soft);
		background: var(--app-bg);
		white-space: pre-wrap;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-main);
		overflow: auto;
		flex: 1;
		width: 100%;
		resize: none;
		font-family: var(--font-mono, 'SFMono-Regular', ui-monospace, monospace);
	}

	.item-markdown[readonly] {
		opacity: 0.82;
	}

	@media (max-width: 980px) {
		.preview-header {
			flex-direction: column;
		}
	}

	@container main-pane (max-width: 760px) {
		.preview-header {
			flex-direction: column;
		}

		.preview-actions {
			width: 100%;
			flex-wrap: wrap;
		}
	}
</style>
