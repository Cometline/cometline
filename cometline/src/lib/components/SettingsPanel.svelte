<script lang="ts">
	import { fade, fly, scale } from 'svelte/transition';
	import {
		Check,
		Download,
		FolderOpen,
		Keyboard,
		LogIn,
		LoaderCircle,
		Palette,
		Plus,
		Power,
		RefreshCw,
		Settings,
		Trash2,
		Workflow,
		X,
		Brain,
		Sparkles
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
	import SettingsGeneralPanel from '$lib/components/SettingsGeneralPanel.svelte';
	import SettingsCometMindPanel from '$lib/components/SettingsCometMindPanel.svelte';
	import SettingsModelRolesPanel from '$lib/components/SettingsModelRolesPanel.svelte';
	import SettingsMemoryPanel from '$lib/components/SettingsMemoryPanel.svelte';
	import SettingsShortcutsPanel from '$lib/components/SettingsShortcutsPanel.svelte';
	import { cloneCometMindSettings, normalizeCometMindSettings } from '$lib/cometmind-settings';
	import { ICON_VARIANT_OPTIONS, projectAvatarSrc } from '$lib/project-icon';
	import type { IconVariant } from '$lib/types';
	import type { MemorySettings } from '$lib/client/cometmind';
	import { pruneWorkspaces } from '$lib/client/cometmind';
	import { onMount } from 'svelte';

	type SettingsSection = 'models' | 'memory' | 'agent' | 'appearance' | 'shortcuts' | 'app';
	type CodexAuthStatus = {
		authenticated: boolean;
		authPath: string;
		accountID?: string;
		error?: string;
	};

	const METHOD_LABELS: Record<ProviderMethod, string> = {
		openai: 'OpenAI',
		anthropic: 'Anthropic',
		'opencode-go': 'OpenCode Go',
		codex: 'ChatGPT Codex',
		'openai-compatible': 'OpenAI-compatible',

	};

	const DEFAULT_PROVIDER_IDS = new Set([
		'anthropic',
		'openai',
		'opencode-go',
		'codex',
		'openai-compatible',

	]);

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
			defaultModelId: settings.defaultModelId ?? '',
			defaultProviderId: settings.defaultProviderId ?? '',
			appearance: {
				heroComposer: { ...settings.appearance.heroComposer },
				caretTrail: { ...settings.appearance.caretTrail }
			},
			shortcuts: cloneShortcuts(settings),
			app: {
				openAtLogin: settings.app?.openAtLogin ?? false,
				hasSeenIntro: settings.app?.hasSeenIntro ?? false,
				iconVariant: settings.app?.iconVariant ?? 'default'
			},
			cometmind: cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind))
		};
	}

	let activeSection = $state<SettingsSection>('models');
	let draft = $state<ProviderSettings>(cloneSettings(settingsStore.settings));
	let selectedProviderId = $state<string>(settingsStore.settings.providers[0]?.id || '');
	let status = $state('');
	let codexAuthStatus = $state<CodexAuthStatus | undefined>();
	let checkingCodexAuth = $state(false);
	let startingCodexLogin = $state(false);
	let modelSearch = $state('');
	let appVersion = $state('');
	let workspacePruning = $state(false);
	let workspacePruneMessage = $state('');
	let updateState = $state<UpdateState>({ status: 'idle' });
	let checkingUpdates = $state(false);
	let installingUpdate = $state(false);
	let cometmindPanelKey = $state(0);
	let cometmindPanel = $state<SettingsCometMindPanel | undefined>();
	let memoryPanelKey = $state(0);
	let memoryPanel = $state<SettingsMemoryPanel | undefined>();

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
		void api.getOpenAtLogin?.().then((current) => {
			if (current) {
				draft = {
					...draft,
					app: { ...draft.app, openAtLogin: current.openAtLogin }
				};
			}
		});

		const unsubscribe = api.onUpdateState?.((next) => {
			updateState = next;
			if (next.status !== 'checking') checkingUpdates = false;
		});
		void refreshCodexAuthStatus();
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

	async function cleanupWorkspaces() {
		if (workspacePruning) return;
		workspacePruning = true;
		workspacePruneMessage = '';
		try {
			const [{ pruned }, storeResult] = await Promise.all([
				pruneWorkspaces(),
				window.electronAPI?.pruneWorkspaceStore?.() ?? { removedRecent: 0, clearedCurrent: false }
			]);
			const parts: string[] = [];
			if (pruned > 0) {
				parts.push(
					`Removed ${pruned} stale workspace registration${pruned === 1 ? '' : 's'} from CometMind`
				);
			}
			if (storeResult.removedRecent > 0) {
				parts.push(
					`Cleared ${storeResult.removedRecent} recent path${storeResult.removedRecent === 1 ? '' : 's'}`
				);
			}
			if (storeResult.clearedCurrent) {
				parts.push('Cleared invalid current workspace path');
			}
			workspacePruneMessage = parts.length > 0 ? parts.join('. ') + '.' : 'Nothing to clean up.';
		} catch (error) {
			workspacePruneMessage =
				error instanceof Error ? error.message : 'Failed to clean up workspaces.';
		} finally {
			workspacePruning = false;
		}
	}

	function replayIntro() {
		shellStore.closeSettings();
		shellStore.openIntro();
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

	function saveStatusMessage(
		section: SettingsSection,
		restartCometMind: boolean,
		iconVariantChanged = false
	) {
		switch (section) {
			case 'models':
				return restartCometMind
					? 'Saved. CometMind is restarting with enabled providers.'
					: 'Saved model settings.';
			case 'agent':
				return restartCometMind
					? 'Agent runtime saved. Sidecar is restarting.'
					: 'Agent runtime saved.';
			case 'appearance':
				return iconVariantChanged || restartCometMind
					? 'Appearance saved. CometMind is restarting with the matching SOUL persona.'
					: 'Appearance saved.';
			case 'app':
				return 'App settings saved.';
			case 'memory':
				return 'Memory settings saved.';
			default:
				return 'Saved settings.';
		}
	}

	function cometmindNeedsRestart(next: ProviderSettings) {
		return (
			JSON.stringify(settingsStore.settings.cometmind) !== JSON.stringify(next.cometmind)
		);
	}

	async function setOpenAtLogin(enabled: boolean) {
		draft = { ...draft, app: { ...draft.app, openAtLogin: enabled } };
		const result = await window.electronAPI?.setOpenAtLogin?.(enabled);
		if (!result) return;

		draft = { ...draft, app: { ...draft.app, openAtLogin: result.openAtLogin } };

		if (result.openedSettings) {
			const devNote = result.isDev ? ' In dev mode it may appear as Electron.' : '';
			status = result.needsApproval
				? `macOS needs your approval in System Settings → Login Items. Enable Cometline there.${devNote}`
				: `Opened System Settings → Login Items. Confirm Cometline is allowed to open at login.${devNote}`;
		} else if (!enabled) {
			status = 'Cometline will no longer open at login.';
		} else if (result.openAtLogin) {
			status = 'Cometline will open at login.';
		}
	}

	async function save() {
		status = '';
		cometmindPanel?.syncFields();
		const preservedSection = activeSection;
		const preservedProviderId = selectedProviderId;
		const preservedModelSearch = modelSearch;

		if (activeSection === 'memory') {
			try {
				const memoryPayload = memoryPanel?.buildSavePayload();
				if (!memoryPayload) {
					throw new Error('Memory settings are not available');
				}
				applyMemoryEmbeddingToDraft(memoryPayload.embedding);
				const payload = providerPayloadFromDraft();
				const restartCometMind =
					providersNeedRestart(payload) || cometmindNeedsRestart(payload);
				const { settings: saved, memory } = await settingsStore.save(payload, {
					restartCometMind,
					memory: memoryPayload
				});
				if (memory) {
					memoryPanel?.applySavedMemory(memory);
				}
				draft = cloneSettings(saved);
				status = saveStatusMessage('memory', restartCometMind);
			} catch (error) {
				status =
					error instanceof Error ? error.message : 'Failed to save memory settings';
			}
			return;
		}

		const activeProvider =
			draft.providers.find(
				(provider) => provider.enabled && provider.enabledModels.length > 0
			) ?? draft.providers[0];
		const payload: ProviderSettings = providerPayloadFromDraft();
		payload.activeProviderId = activeProvider?.id ?? '';
		const iconVariantChanged =
			settingsStore.settings.app.iconVariant !== draft.app.iconVariant;
		const restartCometMind =
			providersNeedRestart(payload) ||
			cometmindNeedsRestart(payload) ||
			iconVariantChanged;
		const { settings: saved } = await settingsStore.save(payload, { restartCometMind });
		draft = cloneSettings(saved);
		cometmindPanelKey += 1;
		activeSection = preservedSection;
		selectedProviderId = draft.providers.some((provider) => provider.id === preservedProviderId)
			? preservedProviderId
			: draft.providers[0]?.id ?? '';
		modelSearch = preservedModelSearch;
		status = saveStatusMessage(preservedSection, restartCometMind, iconVariantChanged);
		if (iconVariantChanged) {
			setTimeout(replayIntro, 600);
		}
	}

	function setSelectedMethod(method: ProviderMethod) {
		if (method === 'opencode-go') {
			updateSelected({
				method,
				baseURL: 'https://opencode.ai/zen/go/v1',
				models: [],
				enabledModels: []
			});
			return;
		}
		if (method === 'codex') {
			updateSelected({
				method,
				baseURL: 'https://chatgpt.com/backend-api/codex',
				apiKey: '',
				models: [],
				enabledModels: []
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

	async function refreshCodexAuthStatus() {
		if (!window.electronAPI?.getCodexAuthStatus || checkingCodexAuth) return;
		checkingCodexAuth = true;
		try {
			codexAuthStatus = await window.electronAPI.getCodexAuthStatus();
		} finally {
			checkingCodexAuth = false;
		}
	}

	async function startCodexLogin() {
		if (!window.electronAPI?.startCodexLogin || startingCodexLogin) return;
		startingCodexLogin = true;
		status = '';
		try {
			const result = await window.electronAPI.startCodexLogin();
			status = result.message;
			setTimeout(() => void refreshCodexAuthStatus(), 1500);
		} catch (error) {
			status = error instanceof Error ? error.message : 'Failed to start Codex login.';
		} finally {
			startingCodexLogin = false;
		}
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
				nextProviders.find(
					(provider) => provider.enabled && provider.enabledModels.length > 0
				)?.id ?? nextProviders[0]?.id ?? ''
		};
		selectedProviderId = nextProviders[0]?.id ?? '';
	}

	async function pickGatewayWorkspace() {
		const picked = await window.electronAPI?.selectWorkspacePath?.();
		if (!picked) return;
		draft = {
			...draft,
			cometmind: {
				...draft.cometmind,
				gateway: {
					discord: {
						...draft.cometmind.gateway.discord,
						workspacePath: picked
					}
				}
			}
		};
	}

	function applyMemoryEmbeddingToDraft(embedding: MemorySettings['embedding']) {
		let providerId = embedding.provider_id.trim();
		const model = embedding.model.trim();
		if ((!providerId || providerId === '__saved__') && model) {
			const matched = draft.providers.find(
				(p) => p.enabledModels.includes(model) || p.models.includes(model)
			);
			if (matched) providerId = matched.id;
		}
		let nextProviders = draft.providers.map(cloneProvider);
		if (providerId && model) {
			nextProviders = nextProviders.map((provider) => {
				if (provider.id !== providerId) return provider;
				const enabledModels = provider.enabledModels.includes(model)
					? provider.enabledModels
					: [...provider.enabledModels, model];
				const models = provider.models.includes(model)
					? provider.models
					: [...provider.models, model];
				return { ...provider, enabled: true, models, enabledModels };
			});
		}
		draft = {
			...draft,
			providers: nextProviders,
			cometmind: {
				...draft.cometmind,
				memory: {
					...draft.cometmind.memory,
					embedding: {
						providerId: providerId || embedding.provider_id,
						provider: embedding.provider,
						model: embedding.model,
						baseURL: embedding.base_url,
						apiKey: embedding.api_key ?? ''
					}
				}
			}
		};
	}

	function providerPayloadFromDraft(): ProviderSettings {
		const activeProvider =
			draft.providers.find(
				(provider) => provider.enabled && provider.enabledModels.length > 0
			) ??
			draft.providers.find((provider) => provider.enabled) ??
			draft.providers[0];
		return {
			providers: draft.providers.map(cloneProvider),
			activeProviderId: activeProvider?.id ?? '',
			defaultModelId: draft.defaultModelId ?? '',
			defaultProviderId: draft.defaultProviderId ?? '',
			appearance: {
				heroComposer: { ...draft.appearance.heroComposer },
				caretTrail: { ...draft.appearance.caretTrail }
			},
			shortcuts: cloneShortcuts(draft),
			app: { ...draft.app },
			cometmind: cloneCometMindSettings(draft.cometmind)
		};
	}

	async function persistMemoryEmbedding(embedding: MemorySettings['embedding']) {
		applyMemoryEmbeddingToDraft(embedding);
		await settingsStore.save(providerPayloadFromDraft(), { restartCometMind: false });
	}

	function setIconVariant(iconVariant: IconVariant) {
		draft = { ...draft, app: { ...draft.app, iconVariant } };
	}

	function methodNeedsApiKey(method: ProviderMethod) {
		return method !== 'codex';
	}

	function canFetchModels(provider: ProviderConfig) {
		if (settingsStore.isFetchingModels || !provider.baseURL.trim()) return false;
		return (
			provider.method === 'codex' ||
			provider.method === 'opencode-go' ||
			provider.apiKey.trim().length > 0
		);
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
					{#if activeSection === 'models'}
						Enable providers, fetch models, and pick which models power each role.
					{:else if activeSection === 'appearance'}
						Customize hero composer glow, caret trail, and the project icon.
					{:else if activeSection === 'agent'}
						Configure the runtime, OpenCode subagents, skills, and the Discord gateway.
					{:else if activeSection === 'memory'}
						Manage global memories, retrieval thresholds, and compaction.
					{:else if activeSection === 'shortcuts'}
						Customize keyboard shortcuts. Changes apply immediately.
					{:else}
						Startup, storage, updates, and workspace.
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
					class:selected={activeSection === 'models'}
					onclick={() => {
						activeSection = 'models';
						status = '';
					}}
				>
					<Settings size={15} />
					<span>Models</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={activeSection === 'memory'}
					onclick={() => {
						activeSection = 'memory';
						status = '';
					}}
				>
					<Brain size={15} />
					<span>Memory</span>
				</button>
				<button
					class="settings-nav-item"
					class:selected={activeSection === 'agent'}
					onclick={() => {
						activeSection = 'agent';
						status = '';
					}}
				>
					<Workflow size={15} />
					<span>Agent</span>
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
					<span>Appearance</span>
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
					class:selected={activeSection === 'app'}
					onclick={() => {
						activeSection = 'app';
						status = '';
					}}
				>
					<Power size={15} />
					<span>App</span>
				</button>
			</nav>

			<div class="settings-pane">
				{#if activeSection === 'models'}
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
											<option value="codex">ChatGPT Codex</option>
											<option value="openai">OpenAI</option>
											<option value="anthropic">Anthropic</option>
											<option value="opencode-go">OpenCode Go</option>
											<option value="openai-compatible"
												>OpenAI-compatible</option
											>
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

									{#if methodNeedsApiKey(selectedProvider.method)}
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
									{:else}
										<div class="field-note">
											<span>Authentication</span>
											<p>
												Uses your ChatGPT Plus/Pro browser sign-in and stores a local
												Codex-compatible session at <code>~/.codex/auth.json</code>. No API
												key or Codex CLI install is required.
											</p>
											{#if codexAuthStatus}
												<p class:ok={codexAuthStatus.authenticated}>
													{codexAuthStatus.authenticated
														? 'Signed in with ChatGPT browser session.'
														: (codexAuthStatus.error ?? 'Not signed in.')}
												</p>
											{/if}
											<div class="inline-actions">
												<button
													class="secondary"
													type="button"
													onclick={startCodexLogin}
													disabled={startingCodexLogin || !window.electronAPI?.startCodexLogin}
												>
													{#if startingCodexLogin}<span class="spin"
															><LoaderCircle size={14} /></span
														>{:else}<LogIn size={14} />{/if}
													Sign in with ChatGPT
												</button>
												<button
													class="secondary"
													type="button"
													onclick={refreshCodexAuthStatus}
													disabled={checkingCodexAuth || !window.electronAPI?.getCodexAuthStatus}
												>
													{#if checkingCodexAuth}<span class="spin"
															><LoaderCircle size={14} /></span
														>{:else}<RefreshCw size={14} />{/if}
													Check session
												</button>
											</div>
										</div>
									{/if}
								</div>

								<div class="model-section">
									<div class="model-heading">
										<div>
											<h3>Models</h3>
											{#if selectedProvider.method === 'codex'}
												<p>
													Use Fetch models to refresh models from your ChatGPT browser
													session.
												</p>
											{:else if selectedProvider.method === 'opencode-go'}
												<p>
													Use Fetch models to refresh the latest list from <code
														>/models</code
													> at OpenCode Go.
												</p>
											{:else}
												<p>
													Use Fetch models to refresh the latest list from <code
														>/models</code
													>.
												</p>
											{/if}
										</div>
										<button
											class="secondary"
											onclick={fetchModels}
											disabled={!canFetchModels(selectedProvider)}
										>
											{#if settingsStore.isFetchingModels}<span
													class="spin"
													><LoaderCircle size={14} /></span
												>{/if}
											Fetch models
										</button>
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
					<SettingsModelRolesPanel
						bind:cometmind={draft.cometmind}
						bind:defaultModelId={draft.defaultModelId}
						bind:defaultProviderId={draft.defaultProviderId}
						providers={draft.providers}
					/>
				{:else if activeSection === 'memory'}
					{#key memoryPanelKey}
						<SettingsMemoryPanel
							bind:this={memoryPanel}
							providers={draft.providers}
							savedEmbedding={draft.cometmind.memory.embedding}
							onEmbeddingSaved={persistMemoryEmbedding}
						/>
					{/key}
				{:else if activeSection === 'agent'}
					{#key cometmindPanelKey}
						<SettingsCometMindPanel
							bind:this={cometmindPanel}
							bind:cometmind={draft.cometmind}
							providers={draft.providers}
							onPickWorkspace={pickGatewayWorkspace}
						/>
					{/key}
				{:else if activeSection === 'appearance'}
					<SettingsAppearancePanel
						bind:appearance={draft.appearance.heroComposer}
						bind:caretTrail={draft.appearance.caretTrail}
					/>
					<div class="about-pane appearance-icon">
						<div class="about-row icon-variant-row">
							<div class="icon-variant-copy">
								<span class="about-label">Project icon</span>
								<span class="about-value">
									Chat avatar, intro animation, Dock, menu bar, and SOUL persona
								</span>
							</div>
							<div class="icon-variant-options" role="radiogroup" aria-label="Project icon style">
								{#each ICON_VARIANT_OPTIONS as option (option.id)}
									<button
										type="button"
										class="icon-variant-chip"
										class:selected={draft.app.iconVariant === option.id}
										role="radio"
										aria-checked={draft.app.iconVariant === option.id}
										onclick={() => setIconVariant(option.id)}
									>
										<img
											src={projectAvatarSrc(option.id, 96)}
											alt=""
											width="40"
											height="40"
										/>
										<span>{option.label}</span>
									</button>
								{/each}
							</div>
						</div>
					</div>
				{:else if activeSection === 'shortcuts'}
					<SettingsShortcutsPanel shortcuts={draft.shortcuts} onChange={updateShortcut} />
				{:else}
					<div class="app-pane">
						<SettingsGeneralPanel
							bind:openAtLogin={draft.app.openAtLogin}
							bind:storage={draft.cometmind.storage}
							onOpenAtLoginChange={setOpenAtLogin}
						/>
						<div class="about-pane">
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
						<div class="about-row workspace-row">
							<div class="workspace-info">
								<span class="about-label">Workspace cleanup</span>
								<span class="about-hint">
									Remove deleted workspace folders from /change and CometMind registrations.
								</span>
								{#if workspacePruneMessage}
									<span class="workspace-prune-message">{workspacePruneMessage}</span>
								{/if}
							</div>
							<button class="secondary" onclick={cleanupWorkspaces} disabled={workspacePruning}>
								{#if workspacePruning}
									<span class="spin small"><LoaderCircle size={14} /></span>
								{:else}
									<Trash2 size={14} />
								{/if}
								Clean up
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
						<div class="about-row">
							<div class="update-info">
								<span class="about-label">Intro</span>
								<span class="about-value">Replay the first-run animation</span>
							</div>
							<button class="secondary" onclick={replayIntro}>
								<Sparkles size={14} />
								Replay intro
							</button>
						</div>
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
			<p>
				{#if activeSection === 'models'}
					{enabledModelCount} model{enabledModelCount === 1 ? '' : 's'} enabled
				{:else if activeSection === 'agent'}
					Runs Discord gateway while Cometline is open when enabled
				{:else if activeSection === 'memory'}
					Embedding and memory behavior save with Save below
				{:else if activeSection === 'shortcuts'}
					Shortcut changes apply immediately
				{:else}
					&nbsp;
				{/if}
			</p>
			<button class="secondary" onclick={shellStore.closeSettings}>Cancel</button>
			<button
				class="primary"
				onclick={save}
				disabled={settingsStore.isSaving ||
					settingsStore.isFetchingModels ||
					(activeSection === 'models' && enabledModelCount === 0) ||
					(activeSection === 'memory' && memoryPanel?.isBusy?.())}
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

	.field-note {
		display: grid;
		grid-column: 1 / -1;
		gap: 6px;
		border: 1px solid var(--border-soft);
		border-radius: 11px;
		background: rgba(255, 255, 255, 0.55);
		padding: 10px 11px;
		font-size: 12px;
		color: var(--text-muted);
	}

	.field-note span {
		font-weight: 700;
	}

	.field-note p {
		max-width: 640px;
		font-weight: 500;
		line-height: 1.45;
	}

	.field-note p.ok {
		color: #24745d;
		font-weight: 650;
	}

	.inline-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding-top: 2px;
	}

	.inline-actions .secondary {
		width: fit-content;
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

	.app-pane {
		display: grid;
		gap: 28px;
	}

	.appearance-icon {
		margin-top: 8px;
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

	.about-hint {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-soft);
		max-width: 420px;
	}

	.workspace-prune-message {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
		max-width: 420px;
	}

	.icon-variant-row {
		align-items: flex-start;
		flex-direction: column;
		gap: 14px;
	}

	.icon-variant-copy {
		display: grid;
		gap: 6px;
	}

	.icon-variant-options {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
	}

	.icon-variant-chip {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: rgba(255, 255, 255, 0.76);
		padding: 8px 12px 8px 8px;
		font: inherit;
		font-size: 13px;
		font-weight: 650;
		color: var(--text-main);
	}

	.icon-variant-chip img {
		width: 40px;
		height: 40px;
		border-radius: 999px;
		object-fit: cover;
		border: 1px solid rgba(15, 23, 42, 0.08);
	}

	.icon-variant-chip.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.icon-variant-chip:hover {
		background: rgba(15, 23, 42, 0.04);
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
