import { settingsStore } from '#lib/stores/settings.svelte.js';
import { isFixedBuiltinProvider } from '#lib/features/settings/schema.js';
import type { ProviderConfig, ProviderMethod } from '#lib/types.js';
import type {
	CodexAuthStatus,
	SettingsPanelControllerDeps,
	XaiAuthStatus
} from './settings-panel-types';

const DEFAULT_PROVIDER_IDS = new Set([
	'anthropic',
	'openai',
	'opencode-go',
	'codex',
	'xai',
	'ollama',
	'openai-compatible'
]);

export function createSettingsPanelProviders(
	deps: Pick<
		SettingsPanelControllerDeps,
		| 'getDraft'
		| 'setDraft'
		| 'setSelectedProviderId'
		| 'getSelectedProvider'
		| 'settingsController'
	>
) {
	let codexAuthStatus = $state<CodexAuthStatus | undefined>();
	let checkingCodexAuth = $state(false);
	let startingCodexLogin = $state(false);
	let xaiAuthStatus = $state<XaiAuthStatus | undefined>();
	let checkingXaiAuth = $state(false);
	let startingXaiLogin = $state(false);

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
		let nextDefaultProviderId = draft.defaultProviderId;
		let nextDefaultModelId = draft.defaultModelId;
		if (draft.defaultProviderId === providerId) {
			const fallback =
				nextProviders.find(
					(provider) => provider.enabled && provider.enabledModels.length > 0
				) ?? nextProviders[0];
			nextDefaultProviderId = fallback?.id ?? '';
			nextDefaultModelId = fallback?.enabledModels[0] ?? '';
		}
		deps.setDraft({
			...draft,
			providers: nextProviders,
			defaultProviderId: nextDefaultProviderId,
			defaultModelId: nextDefaultModelId
		});
		deps.setSelectedProviderId(nextProviders[0]?.id ?? '');
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
		updateProvider,
		updateSelected,
		setSelectedMethod,
		toggleProvider,
		toggleModel,
		fetchModels,
		refreshCodexAuthStatus,
		startCodexLogin,
		refreshXaiAuthStatus,
		startXaiLogin,
		addProvider,
		removeProvider
	};
}
