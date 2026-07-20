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
		return [...provider.enabledModels];
	}

	function firstModelForProvider(provider: ProviderConfig | undefined): string {
		return provider?.enabledModels[0] ?? '';
	}

	function providerById(providerId: string) {
		return runtimeProviders.find((provider) => provider.id === providerId);
	}

	function firstRuntimeProvider() {
		return runtimeProviders[0];
	}

	function providerOptionLabel(provider: ProviderConfig): string {
		return provider.method === 'ollama' ? `${provider.name} (Local)` : provider.name;
	}

	function isRuntimeProviderId(providerId: string): boolean {
		return runtimeProviders.some((provider) => provider.id === providerId);
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
		const match = modelOptions.find(
			(o) => o.providerId === defaultProviderId && o.modelId === defaultModelId
		);
		return match ? `${match.providerName} · ${match.modelId}` : 'No model selected';
	});

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

	// ── Title model (provider + model, falls back to Default) ───────────
	const titleProvider = $derived(providerById(cometmind.titleProviderId));

	const titleModels = $derived(modelsForProvider(titleProvider));

	function setTitleProvider(providerId: string) {
		if (!providerId) {
			cometmind = { ...cometmind, titleProviderId: '', titleModelId: '' };
			return;
		}
		const modelId = firstModelForProvider(providerById(providerId));
		cometmind = { ...cometmind, titleProviderId: providerId, titleModelId: modelId };
	}

	function setTitleModel(modelId: string) {
		cometmind = { ...cometmind, titleModelId: modelId };
	}

	// ── Memory extraction (provider + model, falls back to Default) ─────
	const extractionProvider = $derived(providerById(cometmind.memory.extractionProviderId));

	const extractionModels = $derived(modelsForProvider(extractionProvider));

	function setExtractionProvider(providerId: string) {
		if (!providerId) {
			cometmind = {
				...cometmind,
				memory: { ...cometmind.memory, extractionProviderId: '', extractionModel: '' }
			};
			return;
		}
		const modelId = firstModelForProvider(providerById(providerId));
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

	// ── Autonomous jobs and skill synthesis model roles ─────────────────
	const autonomyProvider = $derived(providerById(cometmind.autonomy.providerId));
	const autonomyModels = $derived(modelsForProvider(autonomyProvider));
	const synthesisProvider = $derived(providerById(cometmind.skills.synthesisProviderId));
	const synthesisModels = $derived(modelsForProvider(synthesisProvider));

	$effect(() => {
		const first = modelOptions[0];
		const selected = modelOptions.find(
			(option) => option.providerId === defaultProviderId && option.modelId === defaultModelId
		);
		if (!selected && first) {
			defaultProviderId = first.providerId;
			defaultModelId = first.modelId;
		}
	});

	// Drop role pins that point at disabled / model-less providers.
	$effect(() => {
		const next = { ...cometmind };
		let changed = false;
		if (next.titleProviderId && !isRuntimeProviderId(next.titleProviderId)) {
			next.titleProviderId = '';
			next.titleModelId = '';
			changed = true;
		}
		if (
			next.memory.extractionProviderId &&
			!isRuntimeProviderId(next.memory.extractionProviderId)
		) {
			next.memory = { ...next.memory, extractionProviderId: '', extractionModel: '' };
			changed = true;
		}
		if (next.autonomy.providerId && !isRuntimeProviderId(next.autonomy.providerId)) {
			next.autonomy = { ...next.autonomy, providerId: '', modelId: '' };
			changed = true;
		}
		if (
			next.skills.synthesisProviderId &&
			!isRuntimeProviderId(next.skills.synthesisProviderId)
		) {
			next.skills = { ...next.skills, synthesisProviderId: '', synthesisModel: '' };
			changed = true;
		}
		if (changed) cometmind = next;
	});

	function setAutonomyProvider(providerId: string) {
		if (!providerId) {
			cometmind = {
				...cometmind,
				autonomy: { ...cometmind.autonomy, providerId: '', modelId: '' }
			};
			return;
		}
		const provider = providerById(providerId) ?? firstRuntimeProvider();
		cometmind = {
			...cometmind,
			autonomy: {
				...cometmind.autonomy,
				providerId: provider?.id ?? '',
				modelId: firstModelForProvider(provider)
			}
		};
	}

	function setAutonomyModel(modelId: string) {
		cometmind = { ...cometmind, autonomy: { ...cometmind.autonomy, modelId } };
	}

	function setSynthesisProvider(providerId: string) {
		if (!providerId) {
			cometmind = {
				...cometmind,
				skills: { ...cometmind.skills, synthesisProviderId: '', synthesisModel: '' }
			};
			return;
		}
		const provider = providerById(providerId) ?? firstRuntimeProvider();
		cometmind = {
			...cometmind,
			skills: {
				...cometmind.skills,
				synthesisProviderId: provider?.id ?? '',
				synthesisModel: firstModelForProvider(provider)
			}
		};
	}

	function setSynthesisModel(modelId: string) {
		cometmind = { ...cometmind, skills: { ...cometmind.skills, synthesisModel: modelId } };
	}
</script>

<section class="model-roles-panel settings-panel-frame">
	<div class="settings-panel-body">
		<div class="settings-section">
			<div class="settings-section-heading">
				<div>
					<h3>Default model</h3>
					<p>
						Choose which model new chats use by default, and what unpinned roles
						(titles, extraction, jobs, synthesis) fall back to. You can still switch
						models per session.
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
				<h3>Autonomous jobs</h3>
				<p>
					Optional override for background job claims. Leave empty to use the Default
					model.
				</p>
			</div>
			<label>
				<span>Autonomous jobs provider</span>
				<select
					value={cometmind.autonomy.providerId}
					onchange={(e) => setAutonomyProvider(e.currentTarget.value)}
				>
					<option value="">Use default model</option>
					{#each runtimeProviders as provider (provider.id)}
						<option value={provider.id}>{providerOptionLabel(provider)}</option>
					{/each}
				</select>
			</label>
			{#if cometmind.autonomy.providerId}
				<label>
					<span>Autonomous jobs model</span>
					<select
						value={cometmind.autonomy.modelId || autonomyModels[0] || ''}
						onchange={(e) => setAutonomyModel(e.currentTarget.value)}
					>
						{#each autonomyModels as model (model)}
							<option value={model}>{model}</option>
						{/each}
					</select>
					<p class="settings-field-hint">
						Pick a reliable coding-capable model. Job runs can execute tools and
						continue without a visible chat open.
					</p>
				</label>
			{/if}
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Skill synthesis</h3>
				<p>
					Optional override for skill draft proposals after completed jobs. Leave empty to
					use the Default model.
				</p>
			</div>
			<label>
				<span>Synthesis provider</span>
				<select
					value={cometmind.skills.synthesisProviderId}
					onchange={(e) => setSynthesisProvider(e.currentTarget.value)}
				>
					<option value="">Use default model</option>
					{#each runtimeProviders as provider (provider.id)}
						<option value={provider.id}>{providerOptionLabel(provider)}</option>
					{/each}
				</select>
			</label>
			{#if cometmind.skills.synthesisProviderId}
				<label>
					<span>Synthesis model</span>
					<select
						value={cometmind.skills.synthesisModel || synthesisModels[0] || ''}
						onchange={(e) => setSynthesisModel(e.currentTarget.value)}
					>
						{#each synthesisModels as model (model)}
							<option value={model}>{model}</option>
						{/each}
					</select>
					<p class="settings-field-hint">
						This model only drafts skills. Drafts still require explicit promotion
						before becoming active skills.
					</p>
				</label>
			{/if}
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Session titles</h3>
				<p>
					CometMind names each session from your first message using an LLM. Pin a cheaper
					/ faster model here, or leave empty to use the Default model.
				</p>
			</div>
			<label>
				<span>Title provider</span>
				<select
					value={cometmind.titleProviderId}
					onchange={(e) => setTitleProvider(e.currentTarget.value)}
				>
					<option value="">Use default model</option>
					{#each runtimeProviders as provider (provider.id)}
						<option value={provider.id}>{providerOptionLabel(provider)}</option>
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
						A small, fast model is ideal — titles are short and don't need a frontier
						model.
					</p>
				</label>
			{/if}
		</div>

		<div class="settings-section">
			<div class="settings-section-heading">
				<h3>Memory extraction</h3>
				<p>
					After each turn, CometMind extracts durable memories in the background. The same
					provider also runs memory compaction merges. Leave empty to use the Default
					model.
				</p>
			</div>
			<label>
				<span>Extraction provider</span>
				<select
					value={cometmind.memory.extractionProviderId}
					onchange={(e) => setExtractionProvider(e.currentTarget.value)}
				>
					<option value="">Use default model</option>
					{#each runtimeProviders as provider (provider.id)}
						<option value={provider.id}>{providerOptionLabel(provider)}</option>
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
						A small, fast model is ideal — extraction and compaction both use this pin.
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
