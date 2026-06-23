<script lang="ts">
	import { tick } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { Check, ChevronDown, Sparkles } from '@lucide/svelte';
	import type { CometMindSettings } from '$lib/cometmind-settings';
	import type { ProviderConfig } from '$lib/types';
	import { isEmbeddingModelName } from '$lib/embedding-models';

	interface ModelEntry {
		id: string;
		label: string;
		providerId: string;
		providerName: string;
		modelId: string;
	}

	let {
		cometmind = $bindable(),
		defaultModelId = $bindable(''),
		defaultProviderId = $bindable(''),
		providers = []
	}: {
		cometmind: CometMindSettings;
		defaultModelId: string;
		defaultProviderId: string;
		providers?: ProviderConfig[];
	} = $props();

	const runtimeProviders = $derived(
		providers.filter((provider) => provider.enabled && provider.enabledModels.length > 0)
	);

	function modelsForProvider(provider: ProviderConfig | undefined): string[] {
		if (!provider) return [];
		return provider.enabledModels.length ? provider.enabledModels : provider.models;
	}

	// ── Default model picker ────────────────────────────────────────────
	let modelMenuOpen = $state(false);
	let modelSearch = $state('');
	let modelSearchInput = $state<HTMLInputElement | null>(null);

	function labelForModel(modelID: string) {
		return modelID
			.split(/[_/]+/)
			.filter(Boolean)
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1).toUpperCase())
			.join(' ');
	}

	let modelOptions = $derived.by(() => {
		const options: ModelEntry[] = [];
		for (const provider of providers) {
			if (!provider.enabled) continue;
			for (const modelId of provider.enabledModels) {
				if (isEmbeddingModelName(modelId)) continue;
				options.push({
					id: `${provider.id}:${modelId}`,
					label: labelForModel(modelId),
					providerId: provider.id,
					providerName: provider.name || provider.id,
					modelId
				});
			}
		}
		return options;
	});

	let filteredModelOptions = $derived.by(() => {
		const query = modelSearch.trim().toLowerCase();
		if (!query) return modelOptions;
		return modelOptions.filter(
			(option) =>
				option.label.toLowerCase().includes(query) ||
				option.modelId.toLowerCase().includes(query) ||
				option.providerName.toLowerCase().includes(query)
		);
	});

	let groupedModelOptions = $derived.by(() => {
		const groups: {
			providerId: string;
			providerName: string;
			options: ModelEntry[];
		}[] = [];
		for (const option of filteredModelOptions) {
			let group = groups.find((item) => item.providerId === option.providerId);
			if (!group) {
				group = {
					providerId: option.providerId,
					providerName: option.providerName,
					options: []
				};
				groups.push(group);
			}
			group.options.push(option);
		}
		return groups;
	});

	let selectedLabel = $derived.by(() => {
		if (!defaultModelId || !defaultProviderId) return 'First enabled model';
		const match = modelOptions.find(
			(o) => o.providerId === defaultProviderId && o.modelId === defaultModelId
		);
		return match?.label ?? 'First enabled model';
	});

	function selectDefaultModel(option: ModelEntry) {
		defaultModelId = option.modelId;
		defaultProviderId = option.providerId;
		modelMenuOpen = false;
		modelSearch = '';
	}

	function clearDefaultModel() {
		defaultModelId = '';
		defaultProviderId = '';
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

	// ── Title model (provider + model, falls back to session) ───────────
	const titleProvider = $derived(
		runtimeProviders.find((provider) => provider.id === cometmind.titleProviderId) ??
			providers.find((provider) => provider.id === cometmind.titleProviderId)
	);

	const titleModels = $derived(modelsForProvider(titleProvider));

	function setTitleProvider(providerId: string) {
		if (!providerId) {
			cometmind = { ...cometmind, titleProviderId: '', titleModelId: '' };
			return;
		}
		const provider = providers.find((item) => item.id === providerId);
		const modelId = provider
			? (provider.enabledModels[0] ?? provider.selectedModel ?? provider.models[0] ?? '')
			: '';
		cometmind = { ...cometmind, titleProviderId: providerId, titleModelId: modelId };
	}

	function setTitleModel(modelId: string) {
		cometmind = { ...cometmind, titleModelId: modelId };
	}

	// ── Memory extraction (provider + model, falls back to session) ─────
	const extractionProvider = $derived(
		runtimeProviders.find(
			(provider) => provider.id === cometmind.memory.extractionProviderId
		) ?? providers.find((provider) => provider.id === cometmind.memory.extractionProviderId)
	);

	const extractionModels = $derived(modelsForProvider(extractionProvider));

	function setExtractionProvider(providerId: string) {
		if (!providerId) {
			cometmind = {
				...cometmind,
				memory: { ...cometmind.memory, extractionProviderId: '', extractionModel: '' }
			};
			return;
		}
		const provider = providers.find((item) => item.id === providerId);
		const modelId = provider
			? (provider.enabledModels[0] ?? provider.selectedModel ?? provider.models[0] ?? '')
			: '';
		cometmind = {
			...cometmind,
			memory: {
				...cometmind.memory,
				extractionProviderId: providerId,
				extractionModel: modelId
			}
		};
	}

	function setExtractionModel(modelId: string) {
		cometmind = {
			...cometmind,
			memory: { ...cometmind.memory, extractionModel: modelId }
		};
	}
</script>

<section class="model-roles-panel settings-panel-frame">
	<div class="settings-panel-body">
	<div class="settings-section">
		<div class="settings-section-heading">
			<div>
				<h3>Default model</h3>
				<p>
					Choose which model new chats use by default. You can still switch models per
					session.
				</p>
			</div>
		</div>
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
			{#if defaultModelId}
				<button
					class="clear-default-button"
					onclick={clearDefaultModel}
					title="Clear default (use first enabled model)"
				>
					&times;
				</button>
			{/if}

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
								<button
									class="model-option"
									onclick={() => selectDefaultModel(option)}
								>
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
	</div>

	<div class="settings-section">
		<div class="settings-section-heading">
			<h3>Session titles</h3>
			<p>
				CometMind names each session from your first message using an LLM. Pin a cheaper /
				faster model here, or leave as default to reuse the session's own model.
			</p>
		</div>
		<label>
			<span>Title provider</span>
			<select
				value={cometmind.titleProviderId}
				onchange={(e) => setTitleProvider(e.currentTarget.value)}
			>
				<option value="">Use session model (default)</option>
				{#each providers as provider (provider.id)}
					<option value={provider.id}>{provider.name}</option>
				{/each}
			</select>
		</label>
		{#if cometmind.titleProviderId}
			<label>
				<span>Title model</span>
				<select
					value={cometmind.titleModelId || titleModels[0] || ''}
					onchange={(e) => setTitleModel(e.currentTarget.value)}
				>
					{#each titleModels as model (model)}
						<option value={model}>{model}</option>
					{/each}
				</select>
				<p class="settings-field-hint">
					A small, fast model is ideal — titles are short and don't need a frontier model.
				</p>
			</label>
		{/if}
	</div>

	<div class="settings-section">
		<div class="settings-section-heading">
			<h3>Memory extraction</h3>
			<p>
				After each turn, CometMind extracts durable memories in the background. Pin a
				cheaper model from any provider for this step, or leave as default to reuse the
				session's own model.
			</p>
		</div>
		<label>
			<span>Extraction provider</span>
			<select
				value={cometmind.memory.extractionProviderId}
				onchange={(e) => setExtractionProvider(e.currentTarget.value)}
			>
				<option value="">Use session model (default)</option>
				{#each providers as provider (provider.id)}
					<option value={provider.id}>{provider.name}</option>
				{/each}
			</select>
		</label>
		{#if cometmind.memory.extractionProviderId}
			<label>
				<span>Extraction model</span>
				<select
					value={cometmind.memory.extractionModel || extractionModels[0] || ''}
					onchange={(e) => setExtractionModel(e.currentTarget.value)}
				>
					{#each extractionModels as model (model)}
						<option value={model}>{model}</option>
					{/each}
				</select>
				<p class="settings-field-hint">
					A small, fast model is ideal — extraction runs after every turn in the
					background.
				</p>
			</label>
		{/if}
	</div>
	</div>
</section>

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

	.clear-default-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		border-radius: 50%;
		border: none;
		background: rgba(0, 0, 0, 0.06);
		color: var(--text-muted);
		font-size: 14px;
		line-height: 1;
		cursor: pointer;
		transition: background 0.15s;
	}

	.clear-default-button:hover {
		background: rgba(0, 0, 0, 0.12);
		color: var(--text-main);
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
