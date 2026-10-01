<script lang="ts">
	import { tick } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { Check, ChevronDown, Sparkles } from '@lucide/svelte';
	import type { ProviderConfig } from '$lib/types';
	import {
		buildModelOptions,
		filterModelOptions,
		groupModelOptions,
		selectedModelLabel,
		type ModelEntry
	} from '$lib/features/settings/model-roles-panel-options';

	let {
		defaultModelId = $bindable(),
		defaultProviderId = $bindable(),
		providers
	}: {
		defaultModelId: string;
		defaultProviderId: string;
		providers: ProviderConfig[];
	} = $props();

	let modelMenuOpen = $state(false);
	let modelSearch = $state('');
	let modelSearchInput = $state<HTMLInputElement | null>(null);

	let modelOptions = $derived(buildModelOptions(providers));
	let filteredModelOptions = $derived(filterModelOptions(modelOptions, modelSearch));
	let groupedModelOptions = $derived(groupModelOptions(filteredModelOptions));
	let selectedLabel = $derived(
		selectedModelLabel(modelOptions, defaultProviderId, defaultModelId)
	);

	function selectDefaultModel(option: ModelEntry) {
		defaultModelId = option.modelId;
		defaultProviderId = option.providerId;
		modelMenuOpen = false;
		modelSearch = '';
	}

	async function openModelMenu() {
		if (modelOptions.length === 0) return;
		modelMenuOpen = true;
		modelSearch = '';
		await tick();
		modelSearchInput?.focus();
		modelSearchInput?.select();
	}

	function toggleModelMenu() {
		if (modelMenuOpen) {
			modelMenuOpen = false;
			modelSearch = '';
			return;
		}
		void openModelMenu();
	}

	function closeModelMenu(e: FocusEvent) {
		const next = e.relatedTarget as Node | null;
		const current = e.currentTarget as Node;
		if (next && current.contains(next)) return;
		modelMenuOpen = false;
		modelSearch = '';
	}
</script>

<div class="default-model-picker" onfocusout={closeModelMenu}>
	<button
		class="model-button"
		aria-label="Select default model"
		aria-expanded={modelMenuOpen}
		disabled={modelOptions.length === 0}
		onclick={toggleModelMenu}
	>
		<Sparkles size={14} stroke-width={1.8} />
		<span>{selectedLabel}</span>
		<ChevronDown size={12} stroke-width={2} />
	</button>
	{#if modelMenuOpen}
		<div class="model-menu scrollbar-none" transition:fly={{ y: 6, duration: 120 }}>
			<input
				class="model-search"
				bind:this={modelSearchInput}
				bind:value={modelSearch}
				placeholder="Search models..."
				spellcheck="false"
			/>
			{#each groupedModelOptions as group (group.providerId)}
				<div class="model-group" transition:fade={{ duration: 90 }}>
					<div class="model-group-heading">
						<strong>{group.providerName}</strong>
					</div>
					{#each group.options as option (option.id)}
						<button class="model-option" onclick={() => selectDefaultModel(option)}>
							<span class="model-check">
								{#if option.providerId === defaultProviderId && option.modelId === defaultModelId}<Check
										size={14}
										stroke-width={2}
									/>{/if}
							</span>
							<span class="model-option-copy">
								<strong>{option.label}</strong>
								<small>{option.modelId}</small>
							</span>
						</button>
					{/each}
				</div>
			{:else}
				<p class="model-empty">No enabled models match your search.</p>
			{/each}
		</div>
	{/if}
</div>

<style>
	.default-model-picker {
		position: relative;
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.model-button {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 8px 12px;
		border-radius: 11px;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.76);
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		transition:
			border-color 0.15s,
			box-shadow 0.15s;
	}

	.model-button:hover:not(:disabled) {
		background: rgba(15, 23, 42, 0.08);
		border-color: rgba(15, 23, 42, 0.18);
	}

	.model-button:disabled {
		opacity: 0.5;
		cursor: default;
	}

	.model-menu {
		position: absolute;
		top: calc(100% + 6px);
		left: 0;
		z-index: 100;
		min-width: 280px;
		max-height: 320px;
		overflow-y: auto;
		padding: 6px;
		border-radius: 12px;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.96);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
		backdrop-filter: blur(20px);
	}

	.model-search {
		width: 100%;
		padding: 8px 10px;
		border-radius: 8px;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.8);
		font: inherit;
		font-size: 12px;
		outline: none;
		margin-bottom: 4px;
	}

	.model-search:focus {
		border-color: rgba(0, 102, 204, 0.35);
	}

	.model-search::placeholder {
		color: var(--text-muted);
	}

	.model-group {
		margin-top: 4px;
	}

	.model-group-heading {
		padding: 4px 8px 2px;
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.model-option {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 7px 8px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--text-main);
		font: inherit;
		font-size: 13px;
		cursor: pointer;
		text-align: left;
	}

	.model-option:hover {
		background: rgba(0, 102, 204, 0.08);
	}

	.model-check {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		flex-shrink: 0;
		color: rgba(0, 102, 204, 0.8);
	}

	.model-option-copy {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}

	.model-option-copy strong {
		font-weight: 550;
		font-size: 13px;
	}

	.model-option-copy small {
		font-size: 11px;
		color: var(--text-muted);
	}

	.model-empty {
		padding: 12px 8px;
		margin: 0;
		text-align: center;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
