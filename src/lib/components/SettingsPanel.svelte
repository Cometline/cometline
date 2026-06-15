<script lang="ts">
	import { fade, fly, scale } from 'svelte/transition';
	import {
		Check,
		Download,
		FolderOpen,
		Info,
		Keyboard,
		LoaderCircle,
		Palette,
		Plus,
		RefreshCw,
		Settings,
		Trash2,
		X
	} from '@lucide/svelte';
	import type {
		ProviderConfig,
		ProviderMethod,
		ProviderSettings,
		ShortcutAction,
		ShortcutBinding
	} from '$lib/types';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import SettingsAppearancePanel from '$lib/components/SettingsAppearancePanel.svelte';
	import SettingsShortcutsPanel from '$lib/components/SettingsShortcutsPanel.svelte';
	import { onMount } from 'svelte';

	type SettingsSection = 'providers' | 'appearance' | 'shortcuts' | 'about';

	const METHOD_LABELS: Record<ProviderMethod, string> = {
		'openai-compatible': 'OpenAI-compatible',
		openai: 'OpenAI',
		anthropic: 'Anthropic',
		'opencode-go': 'OpenCode Go'
	};

	const DEFAULT_PROVIDER_IDS = new Set([
		'openai-compatible',
		'anthropic',
		'openai',
		'opencode-go'
	]);
	const OPENCODE_GO_AVAILABLE_MODELS = [
		'deepseek-v4-flash',
		'deepseek-v4-pro',
		'glm-5',
		'glm-5.1',
		'kimi-k2.6',
		'kimi-k2.7-code',
		'mimo-v2.5',
		'mimo-v2.5-pro',
		'minimax-m2.7',
		'minimax-m3',
		'qwen3.6-plus',
		'qwen3.7-max',
		'qwen3.7-plus'
	];

	function cloneProvider(provider: ProviderConfig): ProviderConfig {
		return {
			...provider,
			models: [...provider.models],
			enabledModels: [...provider.enabledModels]
		};
	}

	function cloneShortcuts(settings: ProviderSettings): ProviderSettings['shortcuts'] {
		return Object.fromEntries(
			Object.entries(settings.shortcuts).map(([id, binding]) => [
				id,
				binding ? { ...binding } : binding
			])
		) as ProviderSettings['shortcuts'];
	}

	function cloneSettings(settings: ProviderSettings): ProviderSettings {
		return {
			providers: settings.providers.map(cloneProvider),
			activeProviderId: settings.activeProviderId,
			appearance: {
				heroComposer: { ...settings.appearance.heroComposer }
			},
			shortcuts: cloneShortcuts(settings)
		};
	}

	let activeSection = $state<SettingsSection>('providers');
	let draft = $state<ProviderSettings>(cloneSettings(settingsStore.settings));
	let selectedProviderId = $state<string>(
		settingsStore.settings.activeProviderId || settingsStore.settings.providers[0]?.id || ''
	);
	let status = $state('');
	let modelSearch = $state('');
	let appVersion = $state('');
	let updateState = $state<UpdateState>({ status: 'idle' });
	let checkingUpdates = $state(false);
	let installingUpdate = $state(false);

	let selectedProvider = $derived(
		draft.providers.find((p) => p.id === selectedProviderId) ?? draft.providers[0]
	);

	let filteredModels = $derived.by(() => {
		if (!selectedProvider) return [];
		const query = modelSearch.trim().toLowerCase();
		if (!query) return selectedProvider.models;
		return selectedProvider.models.filter((model) => model.toLowerCase().includes(query));
	});

	let enabledProviderCount = $derived(
		draft.providers.filter((provider) => provider.enabled).length
	);
	let enabledModelCount = $derived(
		draft.providers.reduce(
			(total, provider) => total + (provider.enabled ? provider.enabledModels.length : 0),
			0
		)
	);

	let updateStatusText = $derived.by(() => {
		switch (updateState.status) {
			case 'checking':
				return 'Checking for updates…';
			case 'downloading':
				return updateState.percent != null
					? `Downloading update ${updateState.percent}%`
					: 'Downloading update…';
			case 'ready':
				return updateState.version
					? `Update available (v${updateState.version})`
					: 'Update available';
			case 'error':
				return updateState.message ?? 'Update check failed';
			default:
				return 'Cometline is up to date';
		}
	});

	let canCheckUpdates = $derived(
		!checkingUpdates && updateState.status !== 'downloading' && !installingUpdate
	);

	onMount(() => {
		const api = window.electronAPI;
		if (!api) return;

		void api.getAppVersion?.().then((version) => {
			if (version) appVersion = version;
		});
		void api.getUpdateState?.().then((current) => {
			if (current) updateState = current;
		});

		const unsubscribe = api.onUpdateState?.((next) => {
			updateState = next;
			if (next.status !== 'checking') checkingUpdates = false;
		});
		return () => unsubscribe?.();
	});

	async function checkForUpdates() {
		const api = window.electronAPI;
		if (!api?.checkForUpdates || !canCheckUpdates) return;
		checkingUpdates = true;
		try {
			const next = await api.checkForUpdates();
			updateState = next;
		} catch (error) {
			updateState = {
				status: 'error',
				message: error instanceof Error ? error.message : 'Update check failed'
			};
		} finally {
			checkingUpdates = false;
		}
	}

	async function installUpdate() {
		const api = window.electronAPI;
		if (!api?.installUpdate || updateState.status !== 'ready' || installingUpdate) return;
		installingUpdate = true;
		try {
			await api.installUpdate();
		} catch (error) {
			console.error('Failed to install update:', error);
			installingUpdate = false;
		}
	}

	async function changeWorkspace() {
		const api = window.electronAPI;
		if (!api?.selectWorkspacePath) return;
		const selected = await api.selectWorkspacePath();
		if (!selected) return;
		shellStore.setWorkspacePath(selected);
	}

	function updateProvider(providerId: string, patch: Partial<ProviderConfig>) {
		draft = {
			...draft,
			providers: draft.providers.map((provider) => {
				if (provider.id !== providerId) return provider;
				const models = patch.models ? [...patch.models] : [...provider.models];
				const enabledModels = (
					patch.enabledModels ? [...patch.enabledModels] : [...provider.enabledModels]
				).filter((model) => models.includes(model));
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

	function updateShortcut(action: ShortcutAction, binding: ShortcutBinding) {
		const shortcuts = {
			...draft.shortcuts,
			[action]: binding
		};
		draft = {
			...draft,
			shortcuts
		};
		void settingsStore.saveShortcuts(shortcuts).then(() => {
			status = 'Shortcut updated and saved.';
		});
	}

	function providersNeedRestart(next: ProviderSettings) {
		return JSON.stringify(settingsStore.settings.providers) !== JSON.stringify(next.providers);
	}

	function saveStatusMessage(section: SettingsSection, restartCometMind: boolean) {
		switch (section) {
			case 'providers':
				return restartCometMind
					? 'Saved. CometMind is restarting with enabled providers.'
					: 'Saved provider settings.';
			case 'shortcuts':
				return 'Shortcuts saved.';
			case 'appearance':
				return 'Appearance saved.';
			default:
				return 'Saved settings.';
		}
	}

	async function save() {
		status = '';
		const preservedSection = activeSection;
		const preservedProviderId = selectedProviderId;
		const preservedModelSearch = modelSearch;
		const activeProvider =
			draft.providers.find(
				(provider) => provider.enabled && provider.enabledModels.length > 0
			) ??
			draft.providers.find((provider) => provider.enabled) ??
			draft.providers[0];
		const payload: ProviderSettings = {
			providers: draft.providers.map(cloneProvider),
			activeProviderId: activeProvider?.id ?? '',
			appearance: {
				heroComposer: { ...draft.appearance.heroComposer }
			},
			shortcuts: cloneShortcuts(draft)
		};
		const restartCometMind = providersNeedRestart(payload);
		const saved = await settingsStore.save(payload, { restartCometMind });
		draft = cloneSettings(saved);
		activeSection = preservedSection;
		selectedProviderId = draft.providers.some((provider) => provider.id === preservedProviderId)
			? preservedProviderId
			: saved.activeProviderId || draft.providers[0]?.id || '';
		modelSearch = preservedModelSearch;
		status = saveStatusMessage(preservedSection, restartCometMind);
	}

	function setSelectedMethod(method: ProviderMethod) {
		if (method === 'opencode-go') {
			updateSelected({
				method,
				baseURL: 'https://opencode.ai/zen/go/v1',
				models: [...OPENCODE_GO_AVAILABLE_MODELS]
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
		updateSelected({
			enabled: nextEnabledModels.length > 0 ? true : selectedProvider.enabled,
			enabledModels: nextEnabledModels
		});
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
			...draft,
			providers: nextProviders,
			activeProviderId:
				nextProviders.find((provider) => provider.enabled)?.id ?? nextProviders[0]?.id ?? ''
		};
		selectedProviderId = nextProviders[0]?.id ?? '';
	}

	function methodNeedsFetch(method: ProviderMethod) {
		return method !== 'opencode-go';
	}
</script>

<div class="settings-layer" transition:fade={{ duration: 120 }}>
	<button class="scrim" aria-label="Close settings" onclick={shellStore.closeSettings}></button>
	<div
		class="modal"
		role="dialog"
		aria-modal="true"
		aria-labelledby="settings-title"
		transition:scale={{ start: 0.97, duration: 140 }}
	>
		<header>
			<div class="title-mark"><Settings size={16} /></div>
			<div>
				<h2 id="settings-title">Settings</h2>
				<p>
					{#if activeSection === 'providers'}
						Enable providers, fetch models, then choose which models appear in the
						composer.
					{:else if activeSection === 'appearance'}
						Customize hero composer glow and border colors for new-chat screens.
					{:else}
						Customize keyboard shortcuts used across Cometline.
					{/if}
				</p>
			</div>
			<button
				class="icon-button"
				aria-label="Close settings"
				onclick={shellStore.closeSettings}
			>
				<X size={16} />
			</button>
		</header>

		<div class="settings-body">
			<nav class="settings-nav" aria-label="Settings sections">
				<button
					class="settings-nav-item"
					class:selected={activeSection === 'providers'}
					onclick={() => {
						activeSection = 'providers';
						status = '';
					}}
				>
					<Settings size={15} />
					<span>Providers</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={activeSection === 'appearance'}
					onclick={() => {
						activeSection = 'appearance';
						status = '';
					}}
				>
					<Palette size={15} />
					<span>Hero glow</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={activeSection === 'shortcuts'}
					onclick={() => {
						activeSection = 'shortcuts';
						status = '';
					}}
				>
					<Keyboard size={15} />
					<span>Shortcuts</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={activeSection === 'about'}
					onclick={() => {
						activeSection = 'about';
						status = '';
					}}
				>
					<Info size={15} />
					<span>About</span>
				</button>
			</nav>

			<div class="settings-pane">
				{#if activeSection === 'providers'}
					<div class="provider-shell">
						<aside class="provider-sidebar">
							<div class="provider-sidebar-title">
								<span>{enabledProviderCount} enabled</span>
								<button
									class="icon-button inline"
									aria-label="Add provider"
									onclick={addProvider}
								>
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
										<p>
											{METHOD_LABELS[selectedProvider.method]} · {selectedProvider
												.enabledModels.length} enabled models
										</p>
									</div>
									<div class="detail-actions">
										{#if !DEFAULT_PROVIDER_IDS.has(selectedProvider.id)}
											<button
												class="secondary danger"
												aria-label="Delete provider"
												onclick={() => removeProvider(selectedProvider.id)}
											>
												<Trash2 size={14} />
											</button>
										{/if}
										<button
											class="switch"
											class:on={selectedProvider.enabled}
											role="switch"
											aria-checked={selectedProvider.enabled}
											aria-label={`${selectedProvider.enabled ? 'Disable' : 'Enable'} ${selectedProvider.name}`}
											title={`${selectedProvider.enabled ? 'Disable' : 'Enable'} ${selectedProvider.name}`}
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
											oninput={(e) =>
												updateSelected({ name: e.currentTarget.value })}
											placeholder="Provider name"
											spellcheck="false"
										/>
									</label>

									<label>
										<span>Method</span>
										<select
											value={selectedProvider.method}
											onchange={(e) =>
												setSelectedMethod(
													e.currentTarget.value as ProviderMethod
												)}
										>
											<option value="openai-compatible"
												>OpenAI-compatible</option
											>
											<option value="anthropic">Anthropic</option>
											<option value="openai">OpenAI</option>
											<option value="opencode-go">OpenCode Go</option>
										</select>
									</label>

									<label>
										<span>Base URL</span>
										<input
											value={selectedProvider.baseURL}
											oninput={(e) =>
												updateSelected({ baseURL: e.currentTarget.value })}
											placeholder="https://example.com/v1"
											spellcheck="false"
										/>
									</label>

									<label>
										<span>API Key</span>
										<input
											value={selectedProvider.apiKey}
											oninput={(e) =>
												updateSelected({ apiKey: e.currentTarget.value })}
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
												<p>
													Use Fetch models to refresh the latest list from <code
														>/models</code
													>.
												</p>
											{:else}
												<p>OpenCode Go models are available by default.</p>
											{/if}
										</div>
										{#if methodNeedsFetch(selectedProvider.method)}
											<button
												class="secondary"
												onclick={fetchModels}
												disabled={settingsStore.isFetchingModels ||
													!selectedProvider.baseURL.trim() ||
													!selectedProvider.apiKey.trim()}
											>
												{#if settingsStore.isFetchingModels}<span
														class="spin"
														><LoaderCircle size={14} /></span
													>{/if}
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
												class:enabled={selectedProvider.enabledModels.includes(
													model
												)}
												onclick={() => toggleModel(model)}
												transition:fly={{ y: 4, duration: 100 }}
											>
												<span>
													<strong>{model}</strong>
													<small>{selectedProvider.id}:{model}</small>
												</span>
												<span class="model-toggle" aria-hidden="true">
													{#if selectedProvider.enabledModels.includes(model)}<Check
															size={13}
														/>{/if}
												</span>
											</button>
										{:else}
											<p class="empty-models">
												{selectedProvider.models.length === 0
													? 'No models loaded yet.'
													: 'No models match your search.'}
											</p>
										{/each}
									</div>
								</div>
							</section>
						{/if}
					</div>
				{:else if activeSection === 'appearance'}
					<SettingsAppearancePanel bind:appearance={draft.appearance.heroComposer} />
				{:else if activeSection === 'shortcuts'}
					<SettingsShortcutsPanel shortcuts={draft.shortcuts} onChange={updateShortcut} />
				{:else}
					<div class="about-pane">
						<h3>About Cometline</h3>
						<div class="about-row">
							<span class="about-label">Version</span>
							<span class="about-value">{appVersion || '—'}</span>
						</div>
						<div class="about-row workspace-row">
							<div class="workspace-info">
								<span class="about-label">Workspace</span>
								<span class="about-value workspace-path" title={shellStore.workspacePath}>
									{shellStore.workspacePath}
								</span>
							</div>
							<button class="secondary" onclick={changeWorkspace}>
								<FolderOpen size={14} />
								Change
							</button>
						</div>
						<div class="about-row update-row">
							<div class="update-info">
								<span class="about-label">Updates</span>
								<span
									class="update-status"
									class:update-error={updateState.status === 'error'}
									class:update-ready={updateState.status === 'ready'}
								>
									{#if checkingUpdates || updateState.status === 'checking' || updateState.status === 'downloading'}
										<span class="spin small"><LoaderCircle size={14} /></span>
									{/if}
									{updateStatusText}
								</span>
							</div>
							{#if updateState.status === 'ready'}
								<button
									class="primary"
									onclick={installUpdate}
									disabled={installingUpdate}
								>
									{#if installingUpdate}<span class="spin"
											><LoaderCircle size={14} /></span
										>{:else}<Download size={14} />{/if}
									Install update
								</button>
							{:else}
								<button
									class="secondary"
									onclick={checkForUpdates}
									disabled={!canCheckUpdates}
								>
									{#if checkingUpdates || updateState.status === 'checking'}<span
											class="spin"
											><LoaderCircle size={14} /></span
										>{:else}<RefreshCw size={14} />{/if}
									Check for updates
								</button>
							{/if}
						</div>
					</div>
				{/if}
			</div>
		</div>

		{#if settingsStore.error}
			<p class="message error">{settingsStore.error}</p>
		{:else if status}
			<p class="message success">{status}</p>
		{/if}

		<footer>
			<p>{enabledModelCount} model{enabledModelCount === 1 ? '' : 's'} enabled</p>
			<button class="secondary" onclick={shellStore.closeSettings}>Cancel</button>
			<button
				class="primary"
				onclick={save}
				disabled={settingsStore.isSaving ||
					settingsStore.isFetchingModels ||
					(activeSection === 'providers' && enabledModelCount === 0)}
			>
				{#if settingsStore.isSaving}<span class="spin"><LoaderCircle size={14} /></span
					>{/if}
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
		display: flex;
		flex-direction: column;
		width: min(980px, 100%);
		height: min(760px, calc(100vh - 60px));
		max-height: min(760px, calc(100vh - 60px));
		overflow: hidden;
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
		position: sticky;
		top: 0;
		z-index: 2;
		flex-shrink: 0;
		gap: 12px;
		padding-bottom: 16px;
		border-bottom: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.96);
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

	header p {
		min-height: calc(1.45em * 2);
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
		align-items: start;
		min-height: 0;
	}

	.settings-body {
		display: grid;
		grid-template-columns: 168px 1fr;
		gap: 16px;
		flex: 1;
		min-height: 0;
		overflow: hidden;
		padding: 16px 0;
	}

	.settings-nav {
		display: grid;
		gap: 8px;
		align-content: start;
	}

	.settings-nav-item {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.72);
		padding: 10px 12px;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
		text-align: left;
	}

	.settings-nav-item.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.settings-nav-item:hover {
		background: rgba(15, 23, 42, 0.05);
	}

	.settings-pane {
		min-width: 0;
		min-height: 0;
		overflow-y: auto;
		scrollbar-gutter: stable;
	}

	.provider-sidebar,
	.provider-detail {
		border: 1px solid var(--border-soft);
		border-radius: 18px;
		background: rgba(251, 251, 250, 0.72);
		min-height: 0;
	}

	.provider-sidebar {
		padding: 12px;
		align-self: start;
	}

	.provider-detail {
		padding: 16px;
		overflow-y: auto;
		scrollbar-gutter: stable;
		max-height: min(560px, calc(100vh - 220px));
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
		gap: 6px;
		align-content: start;
		max-height: min(420px, calc(100vh - 280px));
		overflow-y: auto;
		scrollbar-gutter: stable;
	}

	.provider-card {
		justify-content: space-between;
		gap: 12px;
		width: 100%;
		align-self: start;
		border: 1px solid var(--border-soft);
		border-radius: 13px;
		background: rgba(255, 255, 255, 0.72);
		padding: 8px 12px;
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
		scrollbar-gutter: stable;
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
		flex-shrink: 0;
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
		position: sticky;
		bottom: 0;
		z-index: 2;
		flex-shrink: 0;
		justify-content: flex-end;
		gap: 8px;
		padding-top: 16px;
		border-top: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.96);
	}

	footer p {
		margin-right: auto;
	}

	.spin {
		display: inline-grid;
		place-items: center;
		animation: spin 0.9s linear infinite;
	}

	.spin.small {
		width: 14px;
		height: 14px;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.about-pane {
		display: grid;
		gap: 16px;
		padding: 4px 2px;
	}

	.about-pane h3 {
		font-size: 15px;
		font-weight: 700;
		margin: 0;
	}

	.about-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 14px 16px;
		border: 1px solid var(--border-soft);
		border-radius: 16px;
		background: rgba(251, 251, 250, 0.72);
	}

	.about-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}

	.about-value {
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
		font-variant-numeric: tabular-nums;
	}

	.update-row {
		align-items: flex-start;
	}

	.update-info {
		display: grid;
		gap: 6px;
	}

	.update-status {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.update-status.update-error {
		color: #b42318;
	}

	.update-status.update-ready {
		color: #027a48;
	}

	.workspace-row {
		align-items: flex-start;
	}

	.workspace-info {
		display: grid;
		gap: 6px;
		min-width: 0;
	}

	.workspace-path {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 420px;
	}

	@media (max-width: 780px) {
		.settings-body,
		.provider-shell,
		.form-grid {
			grid-template-columns: 1fr;
		}

		.modal {
			height: calc(100vh - 40px);
			max-height: calc(100vh - 40px);
		}
	}
</style>
