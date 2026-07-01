import { pruneWorkspaces, type MemorySettings } from '$lib/client/cometmind';
import { shellStore } from '$lib/stores/shell.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import {
	applyMemoryEmbeddingToDraft,
	cloneSettings,
	providerPayloadFromDraft
} from '$lib/settings/settings-draft';
import {
	runtimeActionForSettingsSave,
	saveStatusMessage
} from '$lib/settings/settings-save';
import type {
	ProviderConfig,
	ProviderMethod,
	ProviderSettings,
	ShortcutAction,
	ShortcutBinding
} from '$lib/types';
import type { createSettingsController, SettingsSection } from './settings-controller.svelte';

type CodexAuthStatus = {
	authenticated: boolean;
	authPath: string;
	accountID?: string;
	error?: string;
};

type CometMindPanelRef = {
	syncFields?: () => void;
};

type MemoryPanelRef = {
	isDirty?: () => boolean;
	isBusy?: () => boolean;
	buildSavePayload?: () => MemorySettings;
	applySavedMemory?: (memory: MemorySettings) => void;
};

const DEFAULT_PROVIDER_IDS = new Set([
	'anthropic',
	'openai',
	'opencode-go',
	'codex',
	'openai-compatible'
]);

export function createSettingsPanelController(deps: {
	getDraft: () => ProviderSettings;
	setDraft: (draft: ProviderSettings) => void;
	getSelectedProviderId: () => string;
	setSelectedProviderId: (id: string) => void;
	getModelSearch: () => string;
	setModelSearch: (search: string) => void;
	getSelectedProvider: () => ProviderConfig | undefined;
	getCometmindPanel: () => CometMindPanelRef | undefined;
	getMemoryPanel: () => MemoryPanelRef | undefined;
	settingsController: ReturnType<typeof createSettingsController>;
}) {
	let codexAuthStatus = $state<CodexAuthStatus | undefined>();
	let checkingCodexAuth = $state(false);
	let startingCodexLogin = $state(false);
	let updateState = $state<UpdateState>({ status: 'idle' });
	let checkingUpdates = $state(false);
	let installingUpdate = $state(false);
	let exportingSettings = $state(false);
	let importingSettings = $state(false);
	let workspacePruning = $state(false);
	let workspacePruneMessage = $state('');
	let appVersion = $state('');
	let cometmindPanelKey = $state(0);
	let memoryPanelKey = $state(0);

	const updateStatusText = $derived.by(() => {
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

	const canCheckUpdates = $derived(
		!checkingUpdates && updateState.status !== 'downloading' && !installingUpdate
	);

	function initElectron() {
		const api = window.electronAPI;
		if (!api) return () => {};

		void api.getAppVersion?.().then((version) => {
			if (version) appVersion = version;
		});
		void api.getUpdateState?.().then((current) => {
			if (current) updateState = current;
		});
		void api.getOpenAtLogin?.().then((current) => {
			if (current) {
				deps.setDraft({
					...deps.getDraft(),
					app: { ...deps.getDraft().app, openAtLogin: current.openAtLogin }
				});
			}
		});

		const unsubscribe = api.onUpdateState?.((next) => {
			updateState = next;
			if (next.status !== 'checking') checkingUpdates = false;
		});
		void refreshCodexAuthStatus();
		return () => unsubscribe?.();
	}

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
			deps.settingsController.status =
				'Settings export is only available in the desktop app.';
			return;
		}
		exportingSettings = true;
		deps.settingsController.status = '';
		try {
			const result = await api.exportProviderSettings();
			if (!result.canceled) {
				deps.settingsController.status = `Settings exported to ${result.path}. Keep this file private because it may include API keys.`;
			}
		} catch (error) {
			deps.settingsController.status =
				error instanceof Error ? error.message : 'Failed to export settings.';
		} finally {
			exportingSettings = false;
		}
	}

	async function importSettings() {
		if (importingSettings) return;
		const api = window.electronAPI;
		if (!api?.importProviderSettings) {
			deps.settingsController.status =
				'Settings import is only available in the desktop app.';
			return;
		}
		importingSettings = true;
		deps.settingsController.status = '';
		try {
			const result = await api.importProviderSettings();
			if (!result.canceled && result.settings) {
				settingsStore.apply(result.settings);
				deps.setDraft(cloneSettings(result.settings));
				deps.setSelectedProviderId(
					result.settings.activeProviderId || result.settings.providers[0]?.id || ''
				);
				cometmindPanelKey += 1;
				memoryPanelKey += 1;
				deps.settingsController.status =
					'Settings imported. CometMind is restarting with the imported configuration.';
			}
		} catch (error) {
			deps.settingsController.status =
				error instanceof Error ? error.message : 'Failed to import settings.';
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
		const draft = deps.getDraft();
		deps.setDraft({
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
		});
	}

	function updateSelected(patch: Partial<ProviderConfig>) {
		const selectedProvider = deps.getSelectedProvider();
		if (!selectedProvider) return;
		updateProvider(selectedProvider.id, patch);
	}

	function updateShortcut(action: ShortcutAction, binding: ShortcutBinding) {
		const draft = deps.getDraft();
		const shortcuts = {
			...draft.shortcuts,
			[action]: binding
		};
		deps.setDraft({
			...draft,
			shortcuts
		});
		void settingsStore.saveShortcuts(shortcuts).then(() => {
			deps.settingsController.status = 'Shortcut updated and saved.';
		});
	}

	async function setOpenAtLogin(enabled: boolean) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, openAtLogin: enabled } });
		const result = await window.electronAPI?.setOpenAtLogin?.(enabled);
		if (!result) return;

		deps.setDraft({
			...deps.getDraft(),
			app: { ...deps.getDraft().app, openAtLogin: result.openAtLogin }
		});

		if (result.openedSettings) {
			const devNote = result.isDev ? ' In dev mode it may appear as Electron.' : '';
			deps.settingsController.status = result.needsApproval
				? `macOS needs your approval in System Settings → Login Items. Enable Cometline there.${devNote}`
				: `Opened System Settings → Login Items. Confirm Cometline is allowed to open at login.${devNote}`;
		} else if (!enabled) {
			deps.settingsController.status = 'Cometline will no longer open at login.';
		} else if (result.openAtLogin) {
			deps.settingsController.status = 'Cometline will open at login.';
		}
	}

	async function save() {
		deps.settingsController.status = '';
		deps.getCometmindPanel()?.syncFields?.();
		const preservedSection = deps.settingsController.activeSection;
		const preservedProviderId = deps.getSelectedProviderId();
		const preservedModelSearch = deps.getModelSearch();

		if (deps.settingsController.activeSection === 'memory') {
			try {
				const memoryPayload = deps.getMemoryPanel()?.buildSavePayload?.();
				if (!memoryPayload) {
					throw new Error('Memory settings are not available');
				}
				const draft = applyMemoryEmbeddingToDraft(deps.getDraft(), memoryPayload.embedding);
				deps.setDraft(draft);
				const payload = providerPayloadFromDraft(draft);
				const runtimeAction = runtimeActionForSettingsSave(settingsStore.settings, payload);
				const { settings: saved, memory } = await settingsStore.save(payload, {
					runtimeAction,
					memory: memoryPayload
				});
				if (memory) {
					deps.getMemoryPanel()?.applySavedMemory?.(memory);
				}
				deps.setDraft(cloneSettings(saved));
				deps.settingsController.status = saveStatusMessage('memory', runtimeAction);
			} catch (error) {
				deps.settingsController.status =
					error instanceof Error ? error.message : 'Failed to save memory settings';
			}
			return;
		}

		const draft = deps.getDraft();
		const activeProvider =
			draft.providers.find(
				(provider) => provider.enabled && provider.enabledModels.length > 0
			) ?? draft.providers[0];
		const payload: ProviderSettings = providerPayloadFromDraft(draft);
		payload.activeProviderId = activeProvider?.id ?? '';
		const personaIdChanged = settingsStore.settings.app.personaId !== draft.app.personaId;
		const runtimeAction = runtimeActionForSettingsSave(settingsStore.settings, payload);
		const { settings: saved } = await settingsStore.save(payload, { runtimeAction });
		deps.setDraft(cloneSettings(saved));
		cometmindPanelKey += 1;
		deps.settingsController.activeSection = preservedSection;
		deps.setSelectedProviderId(
			saved.providers.some((provider) => provider.id === preservedProviderId)
				? preservedProviderId
				: (saved.providers[0]?.id ?? '')
		);
		deps.setModelSearch(preservedModelSearch);
		deps.settingsController.status = saveStatusMessage(
			preservedSection,
			runtimeAction,
			personaIdChanged
		);
		if (personaIdChanged) {
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
		const provider = deps.getDraft().providers.find((p) => p.id === providerId);
		if (!provider) return;
		updateProvider(providerId, { enabled: !provider.enabled });
	}

	function toggleModel(model: string) {
		const selectedProvider = deps.getSelectedProvider();
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
		const selectedProvider = deps.getSelectedProvider();
		if (!selectedProvider) return;
		deps.settingsController.status = '';
		const updated = await settingsStore.fetchModelsFor(selectedProvider);
		updateSelected({
			models: updated.models,
			enabledModels: updated.enabledModels,
			selectedModel: updated.selectedModel
		});
		deps.settingsController.status = `Fetched ${updated.models.length} model${updated.models.length === 1 ? '' : 's'} for ${selectedProvider.name}.`;
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
		deps.settingsController.status = '';
		try {
			const result = await window.electronAPI.startCodexLogin();
			deps.settingsController.status = result.message;
			setTimeout(() => void refreshCodexAuthStatus(), 1500);
		} catch (error) {
			deps.settingsController.status =
				error instanceof Error ? error.message : 'Failed to start Codex login.';
		} finally {
			startingCodexLogin = false;
		}
	}

	function addProvider() {
		const id = `provider-${Date.now()}`;
		const draft = deps.getDraft();
		deps.setDraft({
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
		});
		deps.setSelectedProviderId(id);
	}

	function removeProvider(providerId: string) {
		if (DEFAULT_PROVIDER_IDS.has(providerId)) return;
		const draft = deps.getDraft();
		const nextProviders = draft.providers.filter((p) => p.id !== providerId);
		deps.setDraft({
			...draft,
			providers: nextProviders,
			activeProviderId:
				nextProviders.find(
					(provider) => provider.enabled && provider.enabledModels.length > 0
				)?.id ??
				nextProviders[0]?.id ??
				''
		});
		deps.setSelectedProviderId(nextProviders[0]?.id ?? '');
	}

	async function pickGatewayWorkspace() {
		const picked = await window.electronAPI?.selectWorkspacePath?.();
		if (!picked) return;
		const draft = deps.getDraft();
		deps.setDraft({
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
		});
	}

	async function persistMemoryEmbedding(embedding: MemorySettings['embedding']) {
		const draft = applyMemoryEmbeddingToDraft(deps.getDraft(), embedding);
		deps.setDraft(draft);
		await settingsStore.save(providerPayloadFromDraft(draft), { restartCometMind: false });
	}

	function setPersonaId(personaId: string) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, personaId } });
	}

	async function saveCustomPersona(payload: {
		id?: string;
		name: string;
		soulMarkdown: string;
		avatarDataUrl?: string;
	}): Promise<SaveCustomPersonaResult> {
		if (!window.electronAPI?.saveCustomPersona) {
			return { ok: false, error: 'Custom personas are only available in the desktop app.' };
		}
		const result = await window.electronAPI.saveCustomPersona(payload);
		if (result.ok) {
			const settings = await window.electronAPI.getProviderSettings?.();
			if (settings) {
				settingsStore.apply(settings);
				deps.setDraft(cloneSettings(settings));
			}
		}
		return result;
	}

	async function deleteCustomPersona(id: string): Promise<DeleteCustomPersonaResult> {
		if (!window.electronAPI?.deleteCustomPersona) {
			return { ok: false, error: 'Custom personas are only available in the desktop app.' };
		}
		const result = await window.electronAPI.deleteCustomPersona(id);
		if (result.ok) {
			const settings = await window.electronAPI.getProviderSettings?.();
			if (settings) {
				settingsStore.apply(settings);
				deps.setDraft(cloneSettings(settings));
			}
		}
		return result;
	}

	function discardSettings() {
		shellStore.closeSettings();
	}

	function selectSection(section: SettingsSection) {
		deps.settingsController.activeSection = section;
		deps.settingsController.status = '';
	}

	return {
		get codexAuthStatus() {
			return codexAuthStatus;
		},
		get checkingCodexAuth() {
			return checkingCodexAuth;
		},
		get startingCodexLogin() {
			return startingCodexLogin;
		},
		get updateState() {
			return updateState;
		},
		get checkingUpdates() {
			return checkingUpdates;
		},
		get installingUpdate() {
			return installingUpdate;
		},
		get exportingSettings() {
			return exportingSettings;
		},
		get importingSettings() {
			return importingSettings;
		},
		get workspacePruning() {
			return workspacePruning;
		},
		get workspacePruneMessage() {
			return workspacePruneMessage;
		},
		get appVersion() {
			return appVersion;
		},
		get cometmindPanelKey() {
			return cometmindPanelKey;
		},
		get memoryPanelKey() {
			return memoryPanelKey;
		},
		get updateStatusText() {
			return updateStatusText;
		},
		get canCheckUpdates() {
			return canCheckUpdates;
		},
		initElectron,
		checkForUpdates,
		installUpdate,
		changeWorkspace,
		cleanupWorkspaces,
		exportSettings,
		importSettings,
		replayIntro,
		runSetupWizard,
		updateProvider,
		updateSelected,
		updateShortcut,
		setOpenAtLogin,
		save,
		setSelectedMethod,
		toggleProvider,
		toggleModel,
		fetchModels,
		refreshCodexAuthStatus,
		startCodexLogin,
		addProvider,
		removeProvider,
		pickGatewayWorkspace,
		persistMemoryEmbedding,
		setPersonaId,
		saveCustomPersona,
		deleteCustomPersona,
		discardSettings,
		selectSection
	};
}
