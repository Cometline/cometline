<script lang="ts">
	import type { OllamaCatalogEntry } from '$lib/ollama/catalog';
	import { formatBytes, type OllamaPullProgress } from '$lib/ollama/client';
	import SettingsButton from '../SettingsButton.svelte';

	let {
		entry,
		installedAlready,
		isPulling,
		pullProgress,
		pullDisabled,
		onPull,
		onCancel
	}: {
		entry: OllamaCatalogEntry;
		installedAlready: boolean;
		isPulling: boolean;
		pullProgress: OllamaPullProgress | null;
		pullDisabled: boolean;
		onPull: () => void;
		onCancel: () => void;
	} = $props();
</script>

<article class="catalog-item" class:pulling={isPulling}>
	<div class="catalog-item-body">
		<strong>{entry.displayName}</strong>
		<small>
			{entry.pullName}
			<span class="meta-sep">·</span>
			{entry.sizeLabel}
			{#if entry.architectureNote}
				<span class="meta-sep">·</span>
				{entry.architectureNote}
			{/if}
		</small>
		{#if isPulling && pullProgress}
			<div class="pull-inline">
				<div
					class="progress-track"
					role="progressbar"
					aria-valuemin={0}
					aria-valuemax={100}
					aria-valuenow={pullProgress.percent ?? 0}
				>
					<span
						style:width={`${pullProgress.percent != null ? pullProgress.percent : 8}%`}
						class:indeterminate={pullProgress.percent == null}
					></span>
				</div>
				<span class="pull-status">
					{#if pullProgress.percent != null}
						{pullProgress.percent}%{#if pullProgress.completed != null && pullProgress.total}
							· {formatBytes(pullProgress.completed)} / {formatBytes(
								pullProgress.total
							)}{/if}
					{:else}
						{pullProgress.status || 'Starting…'}
					{/if}
				</span>
			</div>
		{/if}
	</div>
	<div class="catalog-item-action">
		{#if installedAlready}
			<span class="installed-label">Installed</span>
		{:else if isPulling}
			<SettingsButton variant="secondary" class="catalog-btn" onclick={onCancel}>
				Cancel
			</SettingsButton>
		{:else}
			<SettingsButton
				variant="secondary"
				class="catalog-btn"
				disabled={pullDisabled}
				onclick={onPull}
			>
				Pull
			</SettingsButton>
		{/if}
	</div>
</article>

<style>
	.progress-track {
		height: 3px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
		overflow: hidden;
	}

	.progress-track span {
		display: block;
		height: 100%;
		background: var(--accent);
		border-radius: inherit;
		transition: width 160ms ease;
	}

	.progress-track span.indeterminate {
		opacity: 0.55;
	}

	.catalog-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 9px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: rgba(255, 255, 255, 0.55);
	}

	.catalog-item.pulling {
		border-color: rgba(0, 102, 204, 0.22);
		background: rgba(0, 102, 204, 0.04);
	}

	.catalog-item-body {
		min-width: 0;
		flex: 1;
	}

	.catalog-item-body strong {
		display: block;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
		line-height: 1.3;
	}

	.catalog-item-body small {
		display: block;
		margin-top: 2px;
		font-size: 10px;
		line-height: 1.35;
		color: var(--text-soft);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta-sep {
		margin: 0 0.25em;
		opacity: 0.7;
	}

	.catalog-item-action {
		flex-shrink: 0;
		display: flex;
		align-items: center;
	}

	.catalog-item-action :global(.catalog-btn) {
		padding: 5px 10px;
		min-width: 4.5rem;
		justify-content: center;
	}

	.pull-inline {
		display: grid;
		gap: 3px;
		margin-top: 6px;
		max-width: 220px;
	}

	.pull-status {
		font-size: 10px;
		color: var(--text-soft);
	}

	.installed-label {
		font-size: 11px;
		font-weight: 600;
		color: var(--status-success);
		padding: 0 4px;
	}
</style>
