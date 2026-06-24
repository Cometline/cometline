<script lang="ts">
	import { fade, scale } from 'svelte/transition';
	import {
		Download,
		FolderOpen,
		Keyboard,
		LoaderCircle,
		Palette,
		Power,
		RefreshCw,
		Settings,
		Trash2,
		Upload,
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
	import SettingsAppearancePanel from './SettingsAppearancePanel.svelte';
	import SettingsGeneralPanel from './SettingsGeneralPanel.svelte';
	import SettingsCometMindPanel from './SettingsCometMindPanel.svelte';
	import SettingsModelRolesPanel from './SettingsModelRolesPanel.svelte';
	import SettingsMemoryPanel from './SettingsMemoryPanel.svelte';
	import SettingsShortcutsPanel from './SettingsShortcutsPanel.svelte';
	import SettingsProvidersPanel from './SettingsProvidersPanel.svelte';
	import SettingsButton from './SettingsButton.svelte';
	import SettingsTabPersistence from './SettingsTabPersistence.svelte';
	import { cloneCometMindSettings, normalizeCometMindSettings } from '$lib/cometmind-settings';
	import { ICON_VARIANT_OPTIONS, projectAvatarSrc } from '$lib/project-icon';
	import type { IconVariant } from '$lib/types';
	import type { MemorySettings } from '$lib/client/cometmind';
	import { pruneWorkspaces } from '$lib/client/cometmind';
	import { heroComposerCssVars } from '$lib/hero-composer-appearance';
	import {
		sectionPendingDirty,
		settingsPendingDirty,
		type SettingsSection as PendingSettingsSection
	} from '$lib/settings/pending-settings';
	import { onMount } from 'svelte';

	type SettingsSection = 'models' | 'memory' | 'agent' | 'appearance' | 'shortcuts' | 'app';
	type CodexAuthStatus = {
		authenticated: boolean;
		authPath: string;
		accountID?: string;
		error?: string;
	};

	const DEFAULT_PROVIDER_IDS = new Set([
		'anthropic',
		'openai',
		'opencode-go',
		'codex',
		'openai-compatible'
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
				hasCompletedSetup: settings.app?.hasCompletedSetup ?? false,
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
	let exportingSettings = $state(false);
	let importingSettings = $state(false);
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

	let draftPendingDirty = $derived(settingsPendingDirty(draft, settingsStore.settings));

	let memoryPendingDirty = $derived(memoryPanel?.isDirty?.() ?? false);

	let hasPendingChanges = $derived(draftPendingDirty || memoryPendingDirty);

	function navSectionDirty(section: PendingSettingsSection): boolean {
		if (section === 'memory') return memoryPendingDirty;
		return sectionPendingDirty(section, draft, settingsStore.settings);
	}

	let saveDisabled = $derived(
		settingsStore.isSaving ||
			settingsStore.isFetchingModels ||
			!hasPendingChanges ||
			(activeSection === 'memory' && memoryPanel?.isBusy?.())
	);

	// Soft warning when no enabled model is selected. We no longer hard-block
	// save (the backend validates on persist), but we surface the reason so the
	// user understands why messages can't be sent yet.
	let modelsSectionWarning = $derived(
		activeSection === 'models' && hasPendingChanges && enabledModelCount === 0
			? 'Enable at least one model to send messages.'
			: ''
	);

	$effect(() => {
		const vars = heroComposerCssVars(draft.appearance.heroComposer);
		const root = document.documentElement;
		for (const [key, value] of Object.entries(vars)) {
			root.style.setProperty(key, value);
		}
		return () => {
			const saved = heroComposerCssVars(settingsStore.settings.appearance.heroComposer);
			for (const [key, value] of Object.entries(saved)) {
				root.style.setProperty(key, value);
			}
		};
	});

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
		shellStore.setDefaultWorkspacePath(selected);
	}

	async function cleanupWorkspaces() {
		if (workspacePruning) return;
		workspacePruning = true;
		workspacePruneMessage = '';
		try {
			const [{ pruned }, storeResult] = await Promise.all([
				pruneWorkspaces(),
				window.electronAPI?.pruneWorkspaceStore?.() ?? {
					removedRecent: 0,
					clearedCurrent: false
				}
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
			workspacePruneMessage =
				parts.length > 0 ? parts.join('. ') + '.' : 'Nothing to clean up.';
		} catch (error) {
			workspacePruneMessage =
				error instanceof Error ? error.message : 'Failed to clean up workspaces.';
		} finally {
			workspacePruning = false;
		}
	}

	async function exportSettings() {
		if (exportingSettings) return;
		const api = window.electronAPI;
		if (!api?.exportProviderSettings) {
			status = 'Settings export is only available in the desktop app.';
			return;
		}
		exportingSettings = true;
		status = '';
		try {
			const result = await api.exportProviderSettings();
			if (!result.canceled) {
				status = `Settings exported to ${result.path}. Keep this file private because it may include API keys.`;
			}
		} catch (error) {
			status = error instanceof Error ? error.message : 'Failed to export settings.';
		} finally {
			exportingSettings = false;
		}
	}

	async function importSettings() {
		if (importingSettings) return;
		const api = window.electronAPI;
		if (!api?.importProviderSettings) {
			status = 'Settings import is only available in the desktop app.';
			return;
		}
		importingSettings = true;
		status = '';
		try {
			const result = await api.importProviderSettings();
			if (!result.canceled && result.settings) {
				settingsStore.apply(result.settings);
				draft = cloneSettings(result.settings);
				selectedProviderId =
					result.settings.activeProviderId || result.settings.providers[0]?.id || '';
				cometmindPanelKey += 1;
				memoryPanelKey += 1;
				status =
					'Settings imported. CometMind is restarting with the imported configuration.';
			}
		} catch (error) {
			status = error instanceof Error ? error.message : 'Failed to import settings.';
		} finally {
			importingSettings = false;
		}
	}

	function replayIntro() {
		shellStore.closeSettings();
		shellStore.openIntro();
	}

	function runSetupWizard() {
		shellStore.closeSettings();
		shellStore.openSetup();
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
		const restartNote = restartCometMind ? ' CometMind restarted.' : '';
		switch (section) {
			case 'models':
				return restartCometMind ? `Changes saved.${restartNote}` : 'Changes saved.';
			case 'agent':
				return `Changes saved.${restartNote}`;
			case 'appearance':
				return iconVariantChanged || restartCometMind
					? `Changes saved.${restartNote}`
					: 'Changes saved.';
			case 'app':
				return 'Changes saved.';
			case 'memory':
				return `Changes saved.${restartNote}`;
			default:
				return 'Changes saved.';
		}
	}

	function cometmindNeedsRestart(next: ProviderSettings) {
		return JSON.stringify(settingsStore.settings.cometmind) !== JSON.stringify(next.cometmind);
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
				status = error instanceof Error ? error.message : 'Failed to save memory settings';
			}
			return;
		}

		const activeProvider =
			draft.providers.find(
				(provider) => provider.enabled && provider.enabledModels.length > 0
			) ?? draft.providers[0];
		const payload: ProviderSettings = providerPayloadFromDraft();
		payload.activeProviderId = activeProvider?.id ?? '';
		const iconVariantChanged = settingsStore.settings.app.iconVariant !== draft.app.iconVariant;
		const restartCometMind =
			providersNeedRestart(payload) || cometmindNeedsRestart(payload) || iconVariantChanged;
		const { settings: saved } = await settingsStore.save(payload, { restartCometMind });
		draft = cloneSettings(saved);
		cometmindPanelKey += 1;
		activeSection = preservedSection;
		selectedProviderId = draft.providers.some((provider) => provider.id === preservedProviderId)
			? preservedProviderId
			: (draft.providers[0]?.id ?? '');
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
				)?.id ??
				nextProviders[0]?.id ??
				''
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

	function discardSettings() {
		shellStore.closeSettings();
	}
</script>

<div class="settings-layer" transition:fade={{ duration: 120 }}>
	<button class="scrim" aria-label="Close settings" onclick={shellStore.closeSettings}></button>
	<div
		class="modal settings-ui"
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
						Customize keyboard shortcuts.
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
					class:has-pending={navSectionDirty('models')}
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
					class:has-pending={navSectionDirty('memory')}
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
					class:has-pending={navSectionDirty('agent')}
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
					class:has-pending={navSectionDirty('appearance')}
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
					class:has-pending={navSectionDirty('app')}
					onclick={() => {
						activeSection = 'app';
						status = '';
					}}
				>
					<Power size={15} />
					<span>App</span>
				</button>
			</nav>

			<div class="settings-pane scrollbar-none">
				{#if activeSection === 'models'}
					<div class="settings-panel-stack">
						<SettingsTabPersistence section="models" />
						<SettingsProvidersPanel
							providers={draft.providers}
							bind:selectedProviderId
							bind:modelSearch
							{enabledProviderCount}
							{filteredModels}
							{selectedProvider}
							{codexAuthStatus}
							{checkingCodexAuth}
							{startingCodexLogin}
							onAddProvider={addProvider}
							onRemoveProvider={removeProvider}
							onToggleProvider={toggleProvider}
							onUpdateSelected={updateSelected}
							onSetMethod={setSelectedMethod}
							onFetchModels={fetchModels}
							onToggleModel={toggleModel}
							onStartCodexLogin={startCodexLogin}
							onRefreshCodexAuth={refreshCodexAuthStatus}
						/>
						<SettingsModelRolesPanel
							bind:cometmind={draft.cometmind}
							bind:defaultModelId={draft.defaultModelId}
							bind:defaultProviderId={draft.defaultProviderId}
							providers={draft.providers}
						/>
					</div>
				{:else if activeSection === 'memory'}
					<SettingsTabPersistence section="memory" />
					{#key memoryPanelKey}
						<SettingsMemoryPanel
							bind:this={memoryPanel}
							providers={draft.providers}
							savedEmbedding={draft.cometmind.memory.embedding}
							onEmbeddingSaved={persistMemoryEmbedding}
						/>
					{/key}
				{:else if activeSection === 'agent'}
					<SettingsTabPersistence section="agent" />
					{#key cometmindPanelKey}
						<SettingsCometMindPanel
							bind:this={cometmindPanel}
							bind:cometmind={draft.cometmind}
							providers={draft.providers}
							onPickWorkspace={pickGatewayWorkspace}
						/>
					{/key}
				{:else if activeSection === 'appearance'}
					<div class="settings-panel-stack">
						<SettingsTabPersistence section="appearance" />
						<SettingsAppearancePanel
							bind:appearance={draft.appearance.heroComposer}
							bind:caretTrail={draft.appearance.caretTrail}
						/>
						<section class="settings-panel-frame">
							<div class="settings-section">
								<div class="settings-section-heading">
									<div>
										<h3>Project icon</h3>
										<p>
											Chat avatar, intro animation, Dock, menu bar, and SOUL
											persona
										</p>
									</div>
								</div>
								<div
									class="icon-variant-options"
									role="radiogroup"
									aria-label="Project icon style"
								>
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
						</section>
					</div>
				{:else if activeSection === 'shortcuts'}
					<SettingsTabPersistence section="shortcuts" />
					<SettingsShortcutsPanel shortcuts={draft.shortcuts} onChange={updateShortcut} />
				{:else}
					<SettingsTabPersistence section="app" />
					<div class="settings-panel-stack">
						<SettingsGeneralPanel
							bind:openAtLogin={draft.app.openAtLogin}
							bind:storage={draft.cometmind.storage}
							onOpenAtLoginChange={setOpenAtLogin}
						/>
						<section class="settings-panel-frame">
							<div class="settings-panel-body">
								<div class="settings-section">
									<div class="settings-section-heading">
										<div>
											<h3>Settings backup</h3>
											<p>
												Export or import all Cometline settings. Exports may
												include provider API keys.
											</p>
										</div>
									</div>
									<div class="settings-row-actions mb-1">
										<button
											class="secondary"
											onclick={exportSettings}
											disabled={exportingSettings}
										>
											{#if exportingSettings}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{:else}
												<Download size={14} />
											{/if}
											Export
										</button>
										<button
											class="secondary"
											onclick={importSettings}
											disabled={importingSettings}
										>
											{#if importingSettings}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{:else}
												<Upload size={14} />
											{/if}
											Import
										</button>
									</div>
								</div>
								<div class="settings-row align-start">
									<div class="settings-row-copy">
										<span class="settings-row-label">Workspace</span>
										<span
											class="settings-row-value workspace-path"
											title={shellStore.defaultWorkspacePath}
										>
											{shellStore.defaultWorkspacePath}
										</span>
									</div>
									<div class="settings-row-actions">
										<button class="secondary" onclick={changeWorkspace}>
											<FolderOpen size={14} />
											Change
										</button>
									</div>
								</div>

								<div class="settings-row align-start">
									<div class="settings-row-copy">
										<span class="settings-row-label">Workspace cleanup</span>
										<span class="settings-row-hint">
											Remove deleted workspace folders from /change and
											CometMind registrations.
										</span>
										{#if workspacePruneMessage}
											<span class="workspace-prune-message"
												>{workspacePruneMessage}</span
											>
										{/if}
									</div>
									<div class="settings-row-actions">
										<button
											class="secondary"
											onclick={cleanupWorkspaces}
											disabled={workspacePruning}
										>
											{#if workspacePruning}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{:else}
												<Trash2 size={14} />
											{/if}
											Clean up
										</button>
									</div>
								</div>

								<div class="settings-row align-start">
									<div class="settings-row-copy">
										<span class="settings-row-label">Updates</span>
										<span
											class="update-status"
											class:update-error={updateState.status === 'error'}
											class:update-ready={updateState.status === 'ready'}
										>
											{#if checkingUpdates || updateState.status === 'checking' || updateState.status === 'downloading'}
												<span class="spin small"
													><LoaderCircle size={14} /></span
												>
											{/if}
											{updateStatusText}
										</span>
									</div>
									<div class="settings-row-actions">
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

								<div class="settings-row">
									<div class="settings-row-copy">
										<span class="settings-row-label">Intro</span>
										<span class="settings-row-hint"
											>Replay the first-run animation</span
										>
									</div>
									<div class="settings-row-actions">
										<button class="secondary" onclick={replayIntro}>
											<Sparkles size={14} />
											Replay intro
										</button>
									</div>
								</div>
								<div class="settings-row">
									<div class="settings-row-copy">
										<span class="settings-row-label">Setup wizard</span>
										<span class="settings-row-hint"
											>Guided provider and model configuration</span
										>
									</div>
									<div class="settings-row-actions">
										<button class="secondary" onclick={runSetupWizard}>
											<Sparkles size={14} />
											Run setup wizard
										</button>
									</div>
								</div>
								<div class="settings-row">
									<span class="settings-row-label">Version</span>
									<span class="settings-row-value mr-2">{appVersion || '—'}</span>
								</div>
							</div>
						</section>
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
			<p class="settings-footer-copy">
				{#if settingsStore.isSaving}
					Saving changes…
				{:else}
					{#if hasPendingChanges}<strong>Unsaved changes ·</strong>{/if}
					Save applies all tabs. Close without saving discards pending edits.
				{/if}
				{#if modelsSectionWarning}<span class="settings-footer-warning"
						>{modelsSectionWarning}</span
					>{/if}
			</p>
			<SettingsButton variant="secondary" onclick={discardSettings}>Discard</SettingsButton>
			<SettingsButton variant="primary" onclick={save} disabled={saveDisabled}>
				{#if settingsStore.isSaving}<span class="spin"><LoaderCircle size={14} /></span
					>{/if}
				Save changes
			</SettingsButton>
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
	footer {
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
	footer p,
	.message {
		margin: 0;
	}

	.settings-footer-warning {
		display: block;
		margin-top: 4px;
		color: #b42318;
		font-size: 12px;
	}

	header h2 {
		font-size: 17px;
		font-weight: 700;
	}

	header p,
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
		cursor: pointer;
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
		cursor: pointer;
	}

	.settings-nav-item.selected {
		border-color: rgba(0, 102, 204, 0.4);
		box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.08);
	}

	.settings-pane {
		min-width: 0;
		min-height: 0;
		overflow-y: auto;
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

	.workspace-path {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 420px;
	}

	.workspace-prune-message {
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-muted);
		max-width: 420px;
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
		background: rgba(15, 23, 42, 0.08);
	}

	@media (max-width: 780px) {
		.settings-body {
			grid-template-columns: 1fr;
		}

		.modal {
			height: calc(100vh - 40px);
			max-height: calc(100vh - 40px);
		}
	}
</style>
