<script lang="ts">
	import { Check, LoaderCircle, Search } from '@lucide/svelte';
	import type { ProviderConfig } from '$lib/types';
	import { providerLabel } from '$lib/features/onboarding/setup-wizard';
	import ProviderTabs from './ProviderTabs.svelte';
	import StepIntro from './StepIntro.svelte';

	let {
		providers,
		activeProviderId,
		provider,
		filteredModels,
		modelFilter = $bindable(),
		canFetch,
		fetching,
		onShowProvider,
		onFetchModels,
		onSelectModel
	}: {
		providers: ProviderConfig[];
		activeProviderId: string;
		provider: ProviderConfig | undefined;
		filteredModels: string[];
		modelFilter: string;
		canFetch: (provider: ProviderConfig) => boolean;
		fetching: boolean;
		onShowProvider: (id: string) => void;
		onFetchModels: () => void;
		onSelectModel: (model: string) => void;
	} = $props();
</script>

{#snippet selectionBadge(tab: ProviderConfig)}
	{#if tab.enabledModels.length > 0}<span class="selection-count">{tab.enabledModels.length}</span
		>{/if}
{/snippet}

<ProviderTabs {providers} {activeProviderId} onSelect={onShowProvider} badge={selectionBadge} />
{#if provider}
	<StepIntro>
		Choose one or more models for {providerLabel(provider)}. The first selected model is used by
		default for this provider.
	</StepIntro>
	<div class="fetch-row">
		<button class="fetch-btn" onclick={onFetchModels} disabled={!canFetch(provider)}>
			{#if fetching}<LoaderCircle size={14} class="spin" />{/if}
			Fetch models
		</button>
		{#if provider.models.length > 0}
			<span class="fetch-hint">{provider.models.length} models available</span>
		{/if}
	</div>
	{#if provider.models.length > 0}
		<label class="model-search">
			<Search size={14} />
			<span class="sr-only">Filter models</span>
			<input type="search" placeholder="Filter models" bind:value={modelFilter} />
		</label>
		<div class="model-list scrollbar-none">
			{#each filteredModels as model (model)}
				<button
					class="model-option"
					class:selected={provider.enabledModels.includes(model)}
					onclick={() => onSelectModel(model)}
				>
					<span>{model}</span>
					{#if provider.enabledModels.includes(model)}
						<Check size={14} />
					{/if}
				</button>
			{:else}
				<p class="empty-models">No models match “{modelFilter}”.</p>
			{/each}
		</div>
	{/if}
{/if}

<style>
	.selection-count {
		display: grid;
		place-items: center;
		min-width: 16px;
		height: 16px;
		padding: 0 4px;
		border-radius: 999px;
		background: rgba(0, 102, 204, 0.1);
		font-size: 9px;
	}

	.fetch-row {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 14px;
	}

	.fetch-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 8px 14px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, var(--panel-bg));
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}

	.fetch-btn:hover:not(:disabled) {
		border-color: var(--accent);
	}

	.fetch-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.fetch-hint {
		font-size: 12px;
		color: var(--text-muted);
	}

	.model-search {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 10px;
		padding: 8px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		color: var(--text-muted);
	}

	.model-search:focus-within {
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.model-search input {
		min-width: 0;
		flex: 1;
		border: 0;
		outline: 0;
		background: transparent;
		color: var(--text-main);
		font: inherit;
		font-size: 12px;
	}

	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}

	.model-list {
		display: grid;
		gap: 6px;
		margin-bottom: 16px;
		max-height: 220px;
		overflow-y: auto;
	}

	.model-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 9px 12px;
		border: 1px solid var(--border-soft);
		border-radius: 8px;
		background: var(--panel-bg, var(--panel-bg));
		cursor: pointer;
		font: inherit;
		font-size: 12px;
		color: var(--text-main);
		text-align: left;
	}

	.model-option:hover {
		border-color: rgba(0, 102, 204, 0.3);
	}

	.model-option.selected {
		border-color: var(--accent);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.12);
	}

	.empty-models {
		margin: 10px 0;
		color: var(--text-muted);
		font-size: 12px;
		text-align: center;
	}
</style>
