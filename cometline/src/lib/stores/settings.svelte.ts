import {
	cloneCometMindSettings,
	cloneProvider,
	defaultSettings,
	migrateSingleProvider,
	newProvider,
	normalizeCometMindSettings,
	normalizeProvider,
	normalizeProviders,
	normalizeSettings,
	parseAndNormalizeSettings,
	runtimeProviders,
	runtimeSlice,
	validateSettings,
	type CometMindSettings,
	type RuntimeSettingsSlice
} from '$lib/settings/schema';
import type { MemorySettings } from '$lib/client/cometmind';
import type { ProviderConfig, ProviderSettings } from '$lib/types';
import { defaultKeyboardShortcuts } from '$lib/keyboard-shortcuts';
import { modelStore } from './model.svelte';
import { connectionState } from './runtime.svelte';
import { persistSettings } from '$lib/settings/persist';

const LOCAL_SETTINGS_KEY = 'cometline-settings';

function readLocalSettings(): ProviderSettings {
	try {
		const raw = localStorage.getItem(LOCAL_SETTINGS_KEY);
		if (!raw) return defaultSettings();
		return parseAndNormalizeSettings(JSON.parse(raw) as Partial<ProviderSettings>);
	} catch {
		return defaultSettings();
	}
}

function writeLocalSettings(settings: ProviderSettings) {
	localStorage.setItem(LOCAL_SETTINGS_KEY, JSON.stringify(settings));
}

function createSettingsStore() {
	let settings = $state<ProviderSettings>(defaultSettings());
	let isLoading = $state(false);
	let isSaving = $state(false);
	let isFetchingModels = $state(false);
	let error = $state('');

	function apply(next: ProviderSettings) {
		settings = normalizeSettings(next);
		modelStore.setProviders(
			settings.providers,
			settings.defaultProviderId,
			settings.defaultModelId
		);
	}

	async function load() {
		isLoading = true;
		error = '';
		try {
			const next = (await window.electronAPI?.getProviderSettings?.()) ?? readLocalSettings();
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
			const models =
				(await window.electronAPI?.fetchProviderModels?.(cloneProvider(provider))) ?? [];
			const enabledModels = provider.enabledModels.filter((model) => models.includes(model));
			const selectedModel =
				enabledModels[0] ??
				(models.includes(provider.selectedModel) ? provider.selectedModel : '');
			return { ...provider, models, enabledModels, selectedModel };
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to fetch models';
			throw err;
		} finally {
			isFetchingModels = false;
		}
	}

	async function save(
		draft: ProviderSettings,
		options: { restartCometMind?: boolean; memory?: MemorySettings } = {}
	) {
		isSaving = true;
		error = '';
		try {
			const result = await persistSettings(draft, options);
			apply(result.settings);
			return result;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save settings';
			throw err;
		} finally {
			isSaving = false;
		}
	}

	async function markIntroSeen() {
		if (settings.app.hasSeenIntro) return;
		const next: ProviderSettings = {
			...settings,
			app: { ...settings.app, hasSeenIntro: true }
		};
		await save(next, { restartCometMind: false });
	}

	async function saveShortcuts(shortcuts: ProviderSettings['shortcuts']) {
		error = '';
		try {
			const normalized = normalizeSettings({ ...settings, shortcuts });
			if (window.electronAPI?.saveProviderSettings) {
				apply(
					await window.electronAPI.saveProviderSettings(normalized, {
						restartCometMind: false
					})
				);
				return;
			}
			writeLocalSettings(normalized);
			apply(normalized);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save shortcuts';
			throw err;
		}
	}

	function setActiveProvider(providerId: string) {
		settings = { ...settings, activeProviderId: providerId };
		const provider = settings.providers.find((p) => p.id === providerId);
		if (provider) {
			modelStore.selectByProviderModel(
				provider.id,
				provider.enabledModels[0] ?? provider.selectedModel
			);
		}
	}

	function updateProvider(providerId: string, patch: Partial<ProviderConfig>) {
		settings = {
			...settings,
			providers: settings.providers.map((p) =>
				p.id === providerId ? normalizeProvider({ ...p, ...patch }, p) : p
			)
		};
		const updated = settings.providers.find((p) => p.id === providerId);
		if (updated) {
			modelStore.setProviders(
				settings.providers,
				settings.defaultProviderId,
				settings.defaultModelId
			);
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
			...settings,
			providers: nextProviders,
			activeProviderId: nextActive
		};
		modelStore.setProviders(
			settings.providers,
			settings.defaultProviderId,
			settings.defaultModelId
		);
	}

	function getActiveProvider() {
		return (
			settings.providers.find((p) => p.id === settings.activeProviderId) ??
			settings.providers[0]
		);
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
		markIntroSeen,
		saveShortcuts,
		setActiveProvider,
		updateProvider,
		addProvider,
		removeProvider,
		getActiveProvider
	};
}

export const settingsStore = createSettingsStore();

export {
	cloneCometMindSettings,
	cloneProvider,
	defaultSettings,
	defaultKeyboardShortcuts,
	migrateSingleProvider,
	newProvider,
	normalizeCometMindSettings,
	normalizeProvider,
	normalizeProviders,
	normalizeSettings,
	runtimeProviders,
	runtimeSlice,
	type CometMindSettings,
	type RuntimeSettingsSlice
};
