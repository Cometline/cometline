import type { ProviderConfig, ProviderSettings } from '$lib/types';
import { modelStore } from './model.svelte';

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
const DEFAULT_OPENCODE_GO_ENABLED_MODELS = ['deepseek-v4-flash'];

const DEFAULT_PROVIDERS: ProviderConfig[] = [
	{
		id: 'openai-compatible',
		name: 'OpenAI Compatible',
		method: 'openai-compatible',
		enabled: false,
		baseURL: '',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'anthropic',
		name: 'Anthropic',
		method: 'anthropic',
		enabled: false,
		baseURL: 'https://api.anthropic.com',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'openai',
		name: 'OpenAI',
		method: 'openai',
		enabled: false,
		baseURL: 'https://api.openai.com/v1',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	},
	{
		id: 'opencode-go',
		name: 'OpenCode Go',
		method: 'opencode-go',
		enabled: true,
		baseURL: 'https://opencode.ai/zen/go/v1',
		apiKey: '',
		selectedModel: DEFAULT_OPENCODE_GO_ENABLED_MODELS[0],
		models: [...OPENCODE_GO_AVAILABLE_MODELS],
		enabledModels: [...DEFAULT_OPENCODE_GO_ENABLED_MODELS]
	}
];

function cloneProvider(provider: ProviderConfig): ProviderConfig {
	return {
		...provider,
		models: [...provider.models],
		enabledModels: [...provider.enabledModels]
	};
}

function normalizeProvider(provider: Partial<ProviderConfig>, fallback?: ProviderConfig): ProviderConfig {
	const method = provider.method ?? fallback?.method ?? 'openai-compatible';
	const models = Array.isArray(provider.models) ? provider.models.filter(Boolean) : fallback?.models ?? [];
	const modelList = method === 'opencode-go'
		? Array.from(new Set([...OPENCODE_GO_AVAILABLE_MODELS, ...models]))
		: models;
	const legacySelected = provider.selectedModel || fallback?.selectedModel || modelList[0] || '';
	const enabledModelsSource = Array.isArray(provider.enabledModels)
		? provider.enabledModels
		: legacySelected
			? [legacySelected]
			: [];
	const enabledModels = enabledModelsSource.filter((model) => modelList.includes(model));
	const selectedModel = enabledModels[0] ?? (modelList.includes(legacySelected) ? legacySelected : modelList[0]) ?? '';

	return {
		id: String(provider.id || fallback?.id || `provider-${Date.now()}`).trim(),
		name: String(provider.name || fallback?.name || 'Provider').trim(),
		method,
		enabled: typeof provider.enabled === 'boolean' ? provider.enabled : (fallback?.enabled ?? false),
		baseURL: String(provider.baseURL ?? fallback?.baseURL ?? '').trim(),
		apiKey: String(provider.apiKey ?? fallback?.apiKey ?? '').trim(),
		selectedModel,
		models: [...modelList],
		enabledModels
	};
}

function normalizeProviders(providers: Partial<ProviderConfig>[] | undefined): ProviderConfig[] {
	const saved = Array.isArray(providers) ? providers : [];
	const normalizedDefaults = DEFAULT_PROVIDERS.map((provider) => {
		const savedProvider = saved.find((p) => p.id === provider.id);
		return normalizeProvider(savedProvider ?? provider, provider);
	});
	const customProviders = saved
		.filter((provider) => !DEFAULT_PROVIDERS.some((p) => p.id === provider.id))
		.map((provider) => normalizeProvider(provider));
	return [...normalizedDefaults, ...customProviders];
}

function newProvider(id: string): ProviderConfig {
	return {
		id,
		name: 'New Provider',
		method: 'openai-compatible',
		enabled: false,
		baseURL: '',
		apiKey: '',
		selectedModel: '',
		models: [],
		enabledModels: []
	};
}

function defaultSettings(): ProviderSettings {
	const providers = DEFAULT_PROVIDERS.map(cloneProvider);
	const active = providers.find((provider) => provider.enabled && provider.enabledModels.length > 0) ?? providers[0];
	return {
		providers,
		activeProviderId: active.id
	};
}

function fallbackSettings() {
	const defaults = defaultSettings();
	return {
		providers: defaults.providers.map(cloneProvider),
		activeProviderId: defaults.activeProviderId
	};
}

function createSettingsStore() {
	let settings = $state<ProviderSettings>(fallbackSettings());
	let isLoading = $state(false);
	let isSaving = $state(false);
	let isFetchingModels = $state(false);
	let error = $state('');

	function apply(next: ProviderSettings) {
		const providers = normalizeProviders(next.providers);
		const firstEnabled = providers.find((provider) => provider.enabled && provider.enabledModels.length > 0);
		const activeProviderId = firstEnabled?.id ?? next.activeProviderId ?? providers[0]?.id ?? '';
		settings = {
			...fallbackSettings(),
			...next,
			providers,
			activeProviderId
		};
		modelStore.setProviders(settings.providers);
	}

	async function load() {
		isLoading = true;
		error = '';
		try {
			const next = (await window.electronAPI?.getProviderSettings?.()) ?? fallbackSettings();
			apply(next);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load settings';
		} finally {
			isLoading = false;
		}
	}

	async function fetchModelsFor(provider: ProviderConfig) {
		isFetchingModels = true;
		error = '';
		try {
			const models = (await window.electronAPI?.fetchProviderModels?.(provider)) ?? [];
			const enabledModels = provider.enabledModels.filter((model) => models.includes(model));
			const selectedModel = enabledModels[0] ?? (models.includes(provider.selectedModel)
				? provider.selectedModel
				: (models[0] ?? ''));
			return { ...provider, models, enabledModels, selectedModel };
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to fetch models';
			throw err;
		} finally {
			isFetchingModels = false;
		}
	}

	async function save(draft: ProviderSettings) {
		isSaving = true;
		error = '';
		try {
			const saved = (await window.electronAPI?.saveProviderSettings?.(draft)) ?? draft;
			apply(saved);
			return saved;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save settings';
			throw err;
		} finally {
			isSaving = false;
		}
	}

	function setActiveProvider(providerId: string) {
		settings = { ...settings, activeProviderId: providerId };
		const provider = settings.providers.find((p) => p.id === providerId);
		if (provider) {
			modelStore.selectByProviderModel(provider.id, provider.enabledModels[0] ?? provider.selectedModel);
		}
	}

	function updateProvider(providerId: string, patch: Partial<ProviderConfig>) {
		settings = {
			...settings,
			providers: settings.providers.map((p) =>
				p.id === providerId
					? normalizeProvider({ ...p, ...patch }, p)
					: p
			)
		};
		const updated = settings.providers.find((p) => p.id === providerId);
		if (updated) {
			modelStore.setProviders(settings.providers);
		}
	}

	function addProvider() {
		const id = `provider-${Date.now()}`;
		settings = {
			...settings,
			providers: [...settings.providers, newProvider(id)]
		};
		return id;
	}

	function removeProvider(providerId: string) {
		const nextProviders = settings.providers.filter((p) => p.id !== providerId);
		let nextActive = settings.activeProviderId;
		if (nextActive === providerId) {
			nextActive = nextProviders[0]?.id ?? '';
		}
		settings = {
			providers: nextProviders,
			activeProviderId: nextActive
		};
		modelStore.setProviders(settings.providers);
	}

	function getActiveProvider() {
		return settings.providers.find((p) => p.id === settings.activeProviderId) ?? settings.providers[0];
	}

	return {
		get settings() {
			return settings;
		},
		get isLoading() {
			return isLoading;
		},
		get isSaving() {
			return isSaving;
		},
		get isFetchingModels() {
			return isFetchingModels;
		},
		get error() {
			return error;
		},
		apply,
		load,
		fetchModelsFor,
		save,
		setActiveProvider,
		updateProvider,
		addProvider,
		removeProvider,
		getActiveProvider
	};
}

export const settingsStore = createSettingsStore();
