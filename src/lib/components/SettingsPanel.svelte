<script lang="ts">
	import { fade, fly, scale } from 'svelte/transition';
	import { Check, LoaderCircle, Plus, Settings, Trash2, X } from '@lucide/svelte';
	import type { ProviderConfig, ProviderMethod, ProviderSettings } from '$lib/types';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';

	const METHOD_LABELS: Record<ProviderMethod, string> = {
		'openai-compatible': 'OpenAI-compatible',
		openai: 'OpenAI',
		anthropic: 'Anthropic',
		'opencode-go': 'OpenCode Go'
	};

	const DEFAULT_PROVIDER_IDS = new Set(['openai-compatible', 'anthropic', 'openai', 'opencode-go']);
	const OPENCODE_GO_DEFAULT_MODELS = ['deepseek-v4-flash'];

	function cloneProvider(provider: ProviderConfig): ProviderConfig {
		return {
			...provider,
			models: [...provider.models],
			enabledModels: [...provider.enabledModels]
		};
	}

	function cloneSettings(settings: ProviderSettings): ProviderSettings {
		return {
			providers: settings.providers.map(cloneProvider),
			activeProviderId: settings.activeProviderId
		};
	}

	let draft = $state<ProviderSettings>(cloneSettings(settingsStore.settings));
	let selectedProviderId = $state<string>(settingsStore.settings.activeProviderId || draft.providers[0]?.id || '');
	let status = $state('');
	let modelSearch = $state('');

	let selectedProvider = $derived(
		draft.providers.find((p) => p.id === selectedProviderId) ?? draft.providers[0]
	);

	let filteredModels = $derived.by(() => {
		if (!selectedProvider) return [];
		const query = modelSearch.trim().toLowerCase();
		if (!query) return selectedProvider.models;
		return selectedProvider.models.filter((model) => model.toLowerCase().includes(query));
	});

	let enabledProviderCount = $derived(draft.providers.filter((provider) => provider.enabled).length);
	let enabledModelCount = $derived(
		draft.providers.reduce((total, provider) => total + (provider.enabled ? provider.enabledModels.length : 0), 0)
	);

	function updateProvider(providerId: string, patch: Partial<ProviderConfig>) {
		draft = {
			...draft,
			providers: draft.providers.map((provider) => {
				if (provider.id !== providerId) return provider;
				const models = patch.models ? [...patch.models] : [...provider.models];
				const enabledModels = (patch.enabledModels ? [...patch.enabledModels] : [...provider.enabledModels]).filter(
					(model) => models.includes(model)
				);
				return {
					...provider,
					...patch,
					models,
					enabledModels,
					selectedModel: enabledModels[0] ?? patch.selectedModel ?? provider.selectedModel
				};
			})
		};
	}

	function updateSelected(patch: Partial<ProviderConfig>) {
		if (!selectedProvider) return;
		updateProvider(selectedProvider.id, patch);
	}

	function setSelectedMethod(method: ProviderMethod) {
		if (method === 'opencode-go') {
			updateSelected({
				method,
				baseURL: 'https://opencode.ai/zen/go/v1',
				models: [...OPENCODE_GO_DEFAULT_MODELS],
				enabledModels: [...OPENCODE_GO_DEFAULT_MODELS],
				selectedModel: OPENCODE_GO_DEFAULT_MODELS[0]
			});
			return;
		}
		updateSelected({ method });
	}

	function toggleProvider(providerId: string) {
		const provider = draft.providers.find((p) => p.id === providerId);
		if (!provider) return;
		updateProvider(providerId, { enabled: !provider.enabled });
	}

	function toggleModel(model: string) {
		if (!selectedProvider) return;
		const nextEnabledModels = selectedProvider.enabledModels.includes(model)
			? selectedProvider.enabledModels.filter((enabledModel) => enabledModel !== model)
			: [...selectedProvider.enabledModels, model];
		updateSelected({ enabled: nextEnabledModels.length > 0 ? true : selectedProvider.enabled, enabledModels: nextEnabledModels });
	}

	async function fetchModels() {
		if (!selectedProvider) return;
		status = '';
		const updated = await settingsStore.fetchModelsFor(selectedProvider);
		updateSelected({
			models: updated.models,
			enabledModels: updated.enabledModels,
			selectedModel: updated.selectedModel
		});
		status = `Fetched ${updated.models.length} model${updated.models.length === 1 ? '' : 's'} for ${selectedProvider.name}.`;
	}

	function addProvider() {
		const id = `provider-${Date.now()}`;
		draft = {
			...draft,
			providers: [
				...draft.providers,
				{
					id,
					name: 'Custom Provider',
					method: 'openai-compatible',
					enabled: false,
					baseURL: '',
					apiKey: '',
					selectedModel: '',
					models: [],
					enabledModels: []
				}
			]
		};
		selectedProviderId = id;
	}

	function removeProvider(providerId: string) {
		if (DEFAULT_PROVIDER_IDS.has(providerId)) return;
		const nextProviders = draft.providers.filter((p) => p.id !== providerId);
		draft = {
			providers: nextProviders,
			activeProviderId: nextProviders.find((provider) => provider.enabled)?.id ?? nextProviders[0]?.id ?? ''
		};
		selectedProviderId = nextProviders[0]?.id ?? '';
	}

	async function save() {
		status = '';
		const activeProvider =
			draft.providers.find((provider) => provider.enabled && provider.enabledModels.length > 0) ??
			draft.providers.find((provider) => provider.enabled) ??
			draft.providers[0];
		const saved = await settingsStore.save({
			providers: draft.providers.map(cloneProvider),
			activeProviderId: activeProvider?.id ?? ''
		});
		draft = cloneSettings(saved);
		selectedProviderId = selectedProvider?.id ?? saved.activeProviderId;
		status = 'Saved. CometMind is restarting with enabled providers.';
	}

	function methodNeedsFetch(method: ProviderMethod) {
		return method !== 'opencode-go';
	}
</script>

<div class="settings-layer" transition:fade={{ duration: 120 }}>
	<button class="scrim" aria-label="Close settings" onclick={shellStore.closeSettings}></button>
	<div class="modal" role="dialog" aria-modal="true" aria-labelledby="settings-title" transition:scale={{ start: 0.97, duration: 140 }}>
		<header>
			<div class="title-mark"><Settings size={16} /></div>
			<div>
				<h2 id="settings-title">Providers</h2>
				<p>Enable providers, fetch models, then choose which models appear in the composer.</p>
			</div>
			<button class="icon-button" aria-label="Close settings" onclick={shellStore.closeSettings}>
				<X size={16} />
			</button>
		</header>

		<div class="provider-shell">
			<aside class="provider-sidebar">
				<div class="provider-sidebar-title">
					<span>{enabledProviderCount} enabled</span>
					<button class="icon-button inline" aria-label="Add provider" onclick={addProvider}>
						<Plus size={15} />
					</button>
				</div>

				<div class="provider-list">
					{#each draft.providers as provider (provider.id)}
						<button
							class="provider-card"
							class:selected={selectedProviderId === provider.id}
							class:enabled={provider.enabled}
							onclick={() => {
								selectedProviderId = provider.id;
								modelSearch = '';
							}}
							transition:fly={{ y: 4, duration: 100 }}
						>
							<span>
								<strong>{provider.name}</strong>
								<small>{METHOD_LABELS[provider.method]}</small>
							</span>
							<span class="provider-dot" aria-hidden="true"></span>
						</button>
					{:else}
						<p class="empty-providers">No providers configured.</p>
					{/each}
				</div>
			</aside>

			{#if selectedProvider}
				<section class="provider-detail">
					<div class="detail-heading">
						<div>
							<h3>{selectedProvider.name}</h3>
							<p>{METHOD_LABELS[selectedProvider.method]} · {selectedProvider.enabledModels.length} enabled models</p>
						</div>
						<div class="detail-actions">
							{#if !DEFAULT_PROVIDER_IDS.has(selectedProvider.id)}
								<button class="secondary danger" aria-label="Delete provider" onclick={() => removeProvider(selectedProvider.id)}>
									<Trash2 size={14} />
								</button>
							{/if}
							<button
								class="switch"
								class:on={selectedProvider.enabled}
								role="switch"
								aria-checked={selectedProvider.enabled}
								onclick={() => toggleProvider(selectedProvider.id)}
							>
								<span></span>
							</button>
						</div>
					</div>

					<div class="form-grid">
						<label>
							<span>Name</span>
							<input
								value={selectedProvider.name}
								oninput={(e) => updateSelected({ name: e.currentTarget.value })}
								placeholder="Provider name"
								spellcheck="false"
							/>
						</label>

						<label>
							<span>Method</span>
							<select
								value={selectedProvider.method}
								onchange={(e) => setSelectedMethod(e.currentTarget.value as ProviderMethod)}
							>
								<option value="openai-compatible">OpenAI-compatible</option>
								<option value="anthropic">Anthropic</option>
								<option value="openai">OpenAI</option>
								<option value="opencode-go">OpenCode Go</option>
							</select>
						</label>

						<label>
							<span>Base URL</span>
							<input
								value={selectedProvider.baseURL}
								oninput={(e) => updateSelected({ baseURL: e.currentTarget.value })}
								placeholder="https://example.com/v1"
								spellcheck="false"
							/>
						</label>

						<label>
							<span>API Key</span>
							<input
								value={selectedProvider.apiKey}
								oninput={(e) => updateSelected({ apiKey: e.currentTarget.value })}
								type="password"
								placeholder="sk-..."
								spellcheck="false"
							/>
						</label>
					</div>

					<div class="model-section">
						<div class="model-heading">
							<div>
								<h3>Models</h3>
								{#if methodNeedsFetch(selectedProvider.method)}
									<p>Use Fetch models to refresh the latest list from <code>/models</code>.</p>
								{:else}
									<p>OpenCode Go models are available by default.</p>
								{/if}
							</div>
							{#if methodNeedsFetch(selectedProvider.method)}
								<button
									class="secondary"
									onclick={fetchModels}
									disabled={settingsStore.isFetchingModels || !selectedProvider.baseURL.trim() || !selectedProvider.apiKey.trim()}
								>
									{#if settingsStore.isFetchingModels}<LoaderCircle size={14} class="spin" />{/if}
									Fetch models
								</button>
							{/if}
						</div>

						<input
							class="model-search"
							bind:value={modelSearch}
							placeholder="Search models..."
							spellcheck="false"
						/>

						<div class="models">
							{#each filteredModels as model (model)}
								<button
									class="model-row"
									class:enabled={selectedProvider.enabledModels.includes(model)}
									onclick={() => toggleModel(model)}
									transition:fly={{ y: 4, duration: 100 }}
								>
									<span>
										<strong>{model}</strong>
										<small>{selectedProvider.id}:{model}</small>
									</span>
									<span class="model-toggle" aria-hidden="true">
										{#if selectedProvider.enabledModels.includes(model)}<Check size={13} />{/if}
									</span>
								</button>
							{:else}
								<p class="empty-models">
									{selectedProvider.models.length === 0 ? 'No models loaded yet.' : 'No models match your search.'}
								</p>
							{/each}
						</div>
					</div>
				</section>
			{/if}
		</div>

		{#if settingsStore.error}
			<p class="message error">{settingsStore.error}</p>
		{:else if status}
			<p class="message success">{status}</p>
		{/if}

		<footer>
			<p>{enabledModelCount} model{enabledModelCount === 1 ? '' : 's'} enabled</p>
			<button class="secondary" onclick={shellStore.closeSettings}>Cancel</button>
			<button class="primary" onclick={save} disabled={settingsStore.isSaving || settingsStore.isFetchingModels || enabledModelCount === 0}>
				{#if settingsStore.isSaving}<LoaderCircle size={14} class="spin" />{/if}
				Save
			</button>
		</footer>
	</div>
</div>

<style>
	.settings-layer {
		position: fixed;
		inset: 0;
		z-index: 80;
		display: grid;
		place-items: center;
		padding: 30px;
	}

	.scrim {
		position: absolute;
		inset: 0;
		border: none;
		background: rgba(17, 24, 39, 0.18);
		backdrop-filter: blur(12px);
	}

	.modal {
		position: relative;
		width: min(980px, 100%);
		max-height: min(760px, 100%);
		overflow: auto;
		background: rgba(255, 255, 255, 0.96);
		border: 1px solid rgba(229, 231, 235, 0.95);
		border-radius: 22px;
		box-shadow: 0 22px 70px rgba(15, 23, 42, 0.18);
		padding: 18px;
	}

	header,
	footer,
	.detail-heading,
	.detail-actions,
	.model-heading,
	.provider-sidebar-title,
	.provider-card,
	.model-row {
		display: flex;
		align-items: center;
	}

	header {
		gap: 12px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--border-soft);
	}

	.title-mark {
		width: 32px;
		height: 32px;
		border-radius: 11px;
		background: rgba(0, 102, 204, 0.09);
		color: var(--accent);
		display: grid;
		place-items: center;
	}

	header h2,
	header p,
	.detail-heading h3,
	.detail-heading p,
	.model-heading h3,
	.model-heading p,
	footer p,
	.message {
		margin: 0;
	}

	header h2 {
		font-size: 17px;
		font-weight: 700;
	}

	header p,
	.detail-heading p,
	.model-heading p,
	.empty-models,
	.empty-providers,
	footer p {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.icon-button {
		margin-left: auto;
		width: 30px;
		height: 30px;
		border: none;
		border-radius: 9px;
		background: transparent;
		color: var(--text-muted);
		display: grid;
		place-items: center;
	}

	.icon-button.inline {
		margin-left: 0;
	}

	.icon-button:hover,
	.secondary:hover,
	.provider-card:hover,
	.model-row:hover {
		background: rgba(15, 23, 42, 0.05);
	}

	.provider-shell {
		display: grid;
		grid-template-columns: 270px 1fr;
		gap: 16px;
		padding: 16px 0;
	}

	.provider-sidebar,
	.provider-detail {
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		background: rgba(251, 251, 250, 0.72);
	}

	.provider-sidebar {
		padding: 12px;
	}

	.provider-sidebar-title {
		justify-content: space-between;
		padding: 0 2px 10px;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-muted);
	}

	.provider-list {
		display: grid;
		gap: 8px;
	}

	.provider-card {
		justify-content: space-between;
		gap: 12px;
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.72);
		padding: 10px 12px;
		color: var(--text-main);
		text-align: left;
	}

	.provider-card.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.provider-card span:first-child,
	.model-row span:first-child {
		display: grid;
		gap: 2px;
		min-width: 0;
	}

	.provider-card strong,
	.model-row strong {
		overflow: hidden;
		font-size: 13px;
		font-weight: 650;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.provider-card small,
	.model-row small {
		overflow: hidden;
		font-size: 11px;
		color: var(--text-soft);
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.provider-dot {
		width: 10px;
		height: 10px;
		border-radius: 999px;
		background: #cbd5e1;
		flex-shrink: 0;
	}

	.provider-card.enabled .provider-dot {
		background: #7aa1aa;
	}

	.provider-detail {
		padding: 16px;
	}

	.detail-heading,
	.model-heading {
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 14px;
	}

	.detail-heading h3,
	.model-heading h3 {
		font-size: 15px;
		font-weight: 700;
	}

	.detail-actions {
		gap: 8px;
	}

	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 12px;
		margin-bottom: 16px;
	}

	label {
		display: grid;
		gap: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	input,
	select {
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.76);
		padding: 10px 11px;
		font: inherit;
		font-size: 13px;
		color: var(--text-main);
		outline: none;
	}

	input:focus,
	select:focus {
		border-color: rgba(0, 102, 204, 0.35);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
	}

	.model-section {
		border-top: 1px solid var(--border-soft);
		padding-top: 14px;
	}

	.model-search {
		margin-bottom: 10px;
	}

	.models {
		display: grid;
		max-height: 260px;
		overflow: auto;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(255, 255, 255, 0.55);
	}

	.model-row {
		justify-content: space-between;
		gap: 12px;
		width: 100%;
		border: none;
		border-bottom: 1px solid var(--border-soft);
		background: transparent;
		padding: 10px 12px;
		color: var(--text-main);
		text-align: left;
	}

	.model-row:last-child {
		border-bottom: none;
	}

	.model-row.enabled {
		background: rgba(0, 102, 204, 0.07);
	}

	.model-toggle {
		width: 34px;
		height: 24px;
		border-radius: 999px;
		background: rgba(203, 213, 225, 0.65);
		color: white;
		display: grid;
		place-items: center;
		flex-shrink: 0;
	}

	.model-row.enabled .model-toggle {
		background: #7aa1aa;
	}

	.switch {
		width: 44px;
		height: 28px;
		border: none;
		border-radius: 999px;
		background: rgba(203, 213, 225, 0.72);
		padding: 3px;
		display: flex;
		align-items: center;
		justify-content: flex-start;
	}

	.switch span {
		width: 22px;
		height: 22px;
		border-radius: 999px;
		background: white;
		box-shadow: 0 1px 5px rgba(15, 23, 42, 0.16);
	}

	.switch.on {
		justify-content: flex-end;
		background: #7aa1aa;
	}

	.secondary,
	.primary {
		border: none;
		border-radius: 10px;
		padding: 8px 11px;
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		display: inline-flex;
		align-items: center;
		gap: 7px;
	}

	.secondary {
		background: rgba(15, 23, 42, 0.04);
		color: var(--text-main);
	}

	.secondary.danger:hover {
		background: rgba(180, 35, 24, 0.08);
		color: #b42318;
	}

	.primary {
		background: var(--text-main);
		color: white;
	}

	button:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.message {
		padding: 0 2px 12px;
		font-size: 12px;
	}

	.message.error {
		color: #b42318;
	}

	.message.success {
		color: #027a48;
	}

	.empty-models,
	.empty-providers {
		padding: 12px;
	}

	footer {
		justify-content: flex-end;
		gap: 8px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
	}

	footer p {
		margin-right: auto;
	}

	.spin {
		animation: spin 0.9s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (max-width: 780px) {
		.provider-shell,
		.form-grid {
			grid-template-columns: 1fr;
		}

		.modal {
			max-height: calc(100vh - 40px);
		}
	}
</style>
