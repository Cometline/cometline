<script lang="ts">
	import { fade, fly } from 'svelte/transition';
	import { Check, Sparkles } from '@lucide/svelte';
	import { modelStore, type ModelOption } from '$lib/stores/model.svelte';

	let {
		onModelChange
	}: {
		onModelChange?: (option: ModelOption) => void | Promise<void>;
	} = $props();

	let modelOpen = $state(false);
	let modelSearch = $state('');
	let modelSearchInput = $state<HTMLInputElement | null>(null);

	let groupedModelOptions = $derived.by(() => {
		const query = modelSearch.trim().toLowerCase();
		const groups = new Map<
			string,
			{
				providerId: string;
				providerName: string;
				providerMethod: string;
				options: ModelOption[];
			}
		>();
		for (const option of modelStore.options) {
			if (
				query &&
				!option.label.toLowerCase().includes(query) &&
				!option.modelId.toLowerCase().includes(query)
			) {
				continue;
			}
			const existing = groups.get(option.providerId);
			if (existing) {
				existing.options.push(option);
			} else {
				groups.set(option.providerId, {
					providerId: option.providerId,
					providerName: option.providerName,
					providerMethod: option.providerMethod,
					options: [option]
				});
			}
		}
		return [...groups.values()];
	});

	function selectModel(option: ModelOption) {
		modelStore.select(option);
		modelOpen = false;
		modelSearch = '';
		void onModelChange?.(option);
	}

	function toggleModelMenu() {
		modelOpen = !modelOpen;
		if (modelOpen) {
			void Promise.resolve().then(() => modelSearchInput?.focus());
		}
	}

	function closeModelMenu(e: FocusEvent) {
		const container = e.currentTarget as HTMLElement;
		if (!container.contains(e.relatedTarget as Node)) {
			modelOpen = false;
			modelSearch = '';
		}
	}
</script>

<div class="model-picker" onfocusout={closeModelMenu}>
	<button
		class="model-button"
		aria-label="Select model"
		aria-expanded={modelOpen}
		title={modelStore.options.length > 0
			? 'Select model for new chats'
			: 'Enable a model in Settings first'}
		disabled={modelStore.options.length === 0}
		onclick={toggleModelMenu}
	>
		<Sparkles size={14} stroke-width={1.8} />
		<span>{modelStore.selected?.label ?? 'No enabled models'}</span>
		<svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor" aria-hidden="true">
			<path d="M2 4l3 3 3-3H2z" />
		</svg>
	</button>

	{#if modelOpen}
		<div class="model-menu" transition:fly={{ y: 6, duration: 120 }}>
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
						<small>{group.providerMethod}</small>
					</div>
					{#each group.options as option (option.id)}
						<button class="model-option" onclick={() => selectModel(option)}>
							<span class="model-check">
								{#if option.id === modelStore.selected?.id}
									<Check size={14} stroke-width={2} />
								{/if}
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
	.model-picker {
		position: relative;
	}

	.model-button {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		max-width: 100%;
		padding: 5px 8px;
		border: none;
		border-radius: 7px;
		background: transparent;
		font-size: 13px;
		font-weight: 500;
		line-height: 1;
		color: var(--text-muted);
		white-space: nowrap;
		cursor: pointer;
	}

	.model-button span {
		min-width: 0;
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.model-button:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.04);
		color: var(--text-main);
	}

	.model-button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.model-button :global(svg) {
		flex-shrink: 0;
	}

	.model-menu {
		position: absolute;
		left: 0;
		bottom: calc(100% + 8px);
		z-index: 30;
		width: min(320px, calc(100vw - 48px));
		max-height: 280px;
		overflow: auto;
		padding: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(246, 249, 252, 0.98);
		box-shadow: var(--shadow-card);
		scrollbar-gutter: stable;
	}

	.model-search {
		width: 100%;
		margin-bottom: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 9px;
		background: rgba(255, 255, 255, 0.76);
		padding: 8px 10px;
		font: inherit;
		font-size: 12px;
		color: var(--text-main);
		outline: none;
	}

	.model-search:focus {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	.model-group + .model-group {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.model-group-heading {
		display: flex;
		align-items: baseline;
		gap: 8px;
		padding: 4px 8px 6px;
	}

	.model-group-heading strong {
		font-size: 11px;
		font-weight: 700;
		color: var(--text-main);
	}

	.model-group-heading small {
		font-size: 10px;
		color: var(--text-soft);
		text-transform: uppercase;
	}

	.model-option {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 8px;
		border: none;
		border-radius: 9px;
		background: transparent;
		text-align: left;
		cursor: pointer;
	}

	.model-option:hover {
		background: rgba(15, 23, 42, 0.06);
	}

	.model-check {
		display: grid;
		place-items: center;
		width: 16px;
		height: 16px;
		flex-shrink: 0;
		color: var(--accent);
	}

	.model-option-copy {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.model-option-copy strong {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-main);
	}

	.model-option-copy small {
		font-size: 10px;
		color: var(--text-soft);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.model-empty {
		margin: 0;
		padding: 10px 8px;
		font-size: 12px;
		color: var(--text-muted);
	}
</style>
