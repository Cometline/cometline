import type { ProviderSettings } from '$lib/types';
import { modelStore } from './model.svelte';

const defaults: ProviderSettings = {
	provider: 'openai',
	baseURL: 'https://opencode.ai/zen/go/v1',
	apiKey: '',
	selectedModel: 'deepseek-v4-flash',
	models: ['deepseek-v4-flash']
};

function fallbackSettings() {
	return { ...defaults, models: [...defaults.models] };
}

function createSettingsStore() {
	let settings = $state<ProviderSettings>(fallbackSettings());
	let isLoading = $state(false);
	let isSaving = $state(false);
	let isFetchingModels = $state(false);
	let error = $state('');

	function apply(next: ProviderSettings) {
		settings = {
			...fallbackSettings(),
			...next,
			models: Array.isArray(next.models) && next.models.length > 0 ? next.models : [next.selectedModel]
		};
		modelStore.setProviderModels(settings.provider, settings.models, settings.selectedModel);
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

	async function fetchModels(draft: ProviderSettings) {
		isFetchingModels = true;
		error = '';
		try {
			const models = (await window.electronAPI?.fetchProviderModels?.(draft)) ?? [];
			const selectedModel = models.includes(draft.selectedModel) ? draft.selectedModel : (models[0] ?? '');
			const next = { ...draft, models, selectedModel };
			apply(next);
			return next;
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

	function setSelectedModel(modelID: string) {
		settings = { ...settings, selectedModel: modelID };
		modelStore.selectByProviderModel(settings.provider, modelID);
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
		fetchModels,
		save,
		setSelectedModel
	};
}

export const settingsStore = createSettingsStore();
