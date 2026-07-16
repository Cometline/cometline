import { pruneWorkspaces, type MemorySettings } from '$lib/client/cometmind';
import { shellStore } from '$lib/stores/shell.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import {
	applyMemoryEmbeddingToDraft,
	applyMemorySettingsToDraft,
	cloneSettings,
	providerPayloadFromDraft
} from '$lib/settings/settings-draft';
import { isFixedBuiltinProvider } from '$lib/settings/schema';
import { runtimeActionForSettingsSave, saveStatusMessage } from '$lib/settings/settings-save';
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

type XaiAuthStatus = {
	authenticated: boolean;
	authPath: string;
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
	'xai',
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
	closeSettings: () => void;
	/**
	 * How the panel is presented. In `'window'` mode Settings runs in its own
	 * Electron window whose route is not wrapped in AppShell, so the intro and
	 * setup wizard (which only mount inside AppShell in the main window) must be
	 * triggered there via IPC instead of the local shell store.
	 */
	getMode: () => 'modal' | 'window';
	settingsController: ReturnType<typeof createSettingsController>;
}) {
	let codexAuthStatus = $state<CodexAuthStatus | undefined>();
	let checkingCodexAuth = $state(false);
	let startingCodexLogin = $state(false);
	let xaiAuthStatus = $state<XaiAuthStatus | undefined>();
	let checkingXaiAuth = $state(false);
	let startingXaiLogin = $state(false);
	let updateState = $state<UpdateState>({ status: 'idle' });
	let checkingUpdates = $state(false);
	let installingUpdate = $state(false);
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
		void refreshXaiAuthStatus();
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

	function replayIntro() {
		// In a separate Settings window the intro has no AppShell to render into,
		// so ask the main window to play it and reveal itself.
		if (deps.getMode() === 'window' && window.electronAPI?.replayIntroInMainWindow) {
			void window.electronAPI.replayIntroInMainWindow();
			return;
		}
		shellStore.closeSettings();
		shellStore.openIntro();
	}

	function runSetupWizard() {
		if (deps.getMode() === 'window' && window.electronAPI?.runSetupWizardInMainWindow) {
			void window.electronAPI.runSetupWizardInMainWindow();
			return;
		}
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
				const draft = applyMemorySettingsToDraft(deps.getDraft(), memoryPayload);
				deps.setDraft(draft);
				const payload = providerPayloadFromDraft(draft);
				const runtimeAction = runtimeActionForSettingsSave(settingsStore.settings, payload);
				const {
					settings: saved,
					memory,
					reload
				} = await settingsStore.save(payload, {
					runtimeAction,
					memory: memoryPayload
				});
				if (memory) {
					deps.getMemoryPanel()?.applySavedMemory?.(memory);
				}
				deps.setDraft(cloneSettings(saved));
				deps.settingsController.status = saveStatusMessage(
					'memory',
					runtimeAction,
					false,
					reload
				);
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
		const runtimeAction = personaIdChanged
			? 'reload'
			: runtimeActionForSettingsSave(settingsStore.settings, payload);
		const { settings: saved, reload } = await settingsStore.save(payload, { runtimeAction });
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
			personaIdChanged,
			reload
		);
		if (personaIdChanged) {
			setTimeout(replayIntro, 600);
		}
	}

	async function persistDraftForRuntime(
		overrides: Partial<Pick<ProviderSettings['cometmind'], 'mcp'>> = {}
	) {
		deps.getCometmindPanel()?.syncFields?.();
		const draft = deps.getDraft();
		const activeProvider =
			draft.providers.find(
				(provider) => provider.enabled && provider.enabledModels.length > 0
			) ?? draft.providers[0];
		const payload: ProviderSettings = providerPayloadFromDraft(draft);
		payload.activeProviderId = activeProvider?.id ?? '';
		payload.cometmind = { ...payload.cometmind, ...overrides };
		const runtimeAction = runtimeActionForSettingsSave(settingsStore.settings, payload);
		const { settings: saved, reload } = await settingsStore.save(payload, { runtimeAction });
		deps.setDraft(cloneSettings(saved));
		// Surface a failed-but-fallback-worked reload so a caller like the MCP
		// OAuth pre-save flow doesn't silently proceed to open a browser against
		// config that may not have actually applied. A hard failure (unhealthy)
		// already throws from settingsStore.save/persistSettings.
		if (reload && reload.action === 'restart-fallback') {
			throw new Error(
				`Settings saved, but CometMind had to restart instead of reloading in place: ${reload.error ?? 'unknown error'}`
			);
		}
	}

	function setSelectedMethod(method: ProviderMethod) {
		const selectedProvider = deps.getSelectedProvider();
		if (selectedProvider && isFixedBuiltinProvider(selectedProvider.id)) return;
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
		if (method === 'xai') {
			updateSelected({
				method,
				baseURL: 'https://api.x.ai/v1',
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

	async function refreshXaiAuthStatus() {
		if (!window.electronAPI?.getXaiAuthStatus || checkingXaiAuth) return;
		checkingXaiAuth = true;
		try {
			xaiAuthStatus = await window.electronAPI.getXaiAuthStatus();
		} finally {
			checkingXaiAuth = false;
		}
	}

	async function startXaiLogin() {
		if (!window.electronAPI?.startXaiLogin || startingXaiLogin) return;
		startingXaiLogin = true;
		deps.settingsController.status = '';
		try {
			const result = await window.electronAPI.startXaiLogin();
			deps.settingsController.status = result.message;
			setTimeout(() => void refreshXaiAuthStatus(), 1500);
		} catch (error) {
			xaiAuthStatus = {
				authenticated: false,
				authPath: '',
				error: error instanceof Error ? error.message : 'Failed to start Grok login.'
			};
		} finally {
			startingXaiLogin = false;
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
			setTimeout(replayIntro, 600);
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
			setTimeout(replayIntro, 600);
		}
		return result;
	}

	function discardSettings() {
		const saved = cloneSettings(settingsStore.settings);
		const selectedProviderId = deps.getSelectedProviderId();
		deps.setDraft(saved);
		deps.setSelectedProviderId(
			saved.providers.some((provider) => provider.id === selectedProviderId)
				? selectedProviderId
				: (saved.providers[0]?.id ?? '')
		);
		deps.setModelSearch('');
		cometmindPanelKey += 1;
		memoryPanelKey += 1;
		workspacePruneMessage = '';
		deps.settingsController.status = 'Discarded unsaved changes.';
		deps.closeSettings();
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
		get xaiAuthStatus() {
			return xaiAuthStatus;
		},
		get checkingXaiAuth() {
			return checkingXaiAuth;
		},
		get startingXaiLogin() {
			return startingXaiLogin;
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
		replayIntro,
		runSetupWizard,
		updateProvider,
		updateSelected,
		updateShortcut,
		setOpenAtLogin,
		save,
		persistDraftForRuntime,
		setSelectedMethod,
		toggleProvider,
		toggleModel,
		fetchModels,
		refreshCodexAuthStatus,
		startCodexLogin,
		refreshXaiAuthStatus,
		startXaiLogin,
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
