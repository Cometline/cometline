import { settingsStore } from '#lib/stores/settings.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import { connectionState } from '#lib/stores/runtime.svelte.js';
import { cloneProvider } from '#lib/features/settings/schema.js';
import type { ProviderConfig, ProviderSettings } from '#lib/types.js';
import {
	canAdvanceStep,
	completedSetupSettings,
	filterModels,
	initialDefaultProviderId,
	initialSelectedProviderIds,
	STEP_ORDER,
	toggleEnabledModel,
	type SetupWizardStep
} from './setup-wizard';
import { createSetupWizardAuth } from './setup-wizard-auth.svelte';
import { createSetupWizardMemory } from './setup-wizard-memory.svelte';

function cloneCurrentSettings(): ProviderSettings {
	return JSON.parse(JSON.stringify(settingsStore.settings)) as ProviderSettings;
}

async function waitForReady(maxAttempts: number): Promise<boolean> {
	for (let i = 0; i < maxAttempts; i++) {
		if (connectionState.status === 'ready') return true;
		await new Promise((resolve) => setTimeout(resolve, 500));
	}
	return false;
}

export function createSetupWizardController() {
	// Draft built from current settings so the wizard edits don't leak if
	// the user cancels.
	const initialDraft = cloneCurrentSettings();
	const initialSelectedIds = initialSelectedProviderIds(initialDraft.providers);
	const initialDefaultId = initialDefaultProviderId(
		initialDraft.defaultProviderId,
		initialSelectedIds
	);

	let step = $state<SetupWizardStep>('provider');
	let saving = $state(false);
	let saveError = $state('');
	let settingsSaved = $state(false);
	let connecting = $state(false);
	let modelFilter = $state('');
	let showApiKey = $state(false);

	let draft = $state<ProviderSettings>(initialDraft);
	let selectedProviderIds = $state<string[]>(initialSelectedIds);
	let defaultProviderId = $state(initialDefaultId);
	let activeProviderId = $state(initialDefaultId);

	let screenCapturePreferred = $state(false);
	let screenCaptureStatus = $state('unknown');
	let screenCaptureBusy = $state(false);

	const auth = createSetupWizardAuth();
	const memory = createSetupWizardMemory({
		getDraft: () => draft,
		setDraft: (next) => (draft = next),
		getSelectedProviderIds: () => selectedProviderIds,
		patchProvider
	});

	const selectedProvider = $derived(draft.providers.find((p) => p.id === activeProviderId));
	const defaultProvider = $derived(
		draft.providers.find((provider) => provider.id === defaultProviderId)
	);
	const selectedProviders = $derived(
		draft.providers.filter((provider) => selectedProviderIds.includes(provider.id))
	);
	const filteredModels = $derived(
		selectedProvider ? filterModels(selectedProvider.models, modelFilter) : []
	);
	const stepIndex = $derived(STEP_ORDER.indexOf(step));
	const canAdvance = $derived(
		canAdvanceStep(step, selectedProviderIds, selectedProviders, providerIsConnected)
	);

	function canFetchModels(provider: ProviderConfig) {
		if (settingsStore.isFetchingModels || !provider.baseURL.trim()) return false;
		return (
			provider.method === 'codex' ||
			provider.method === 'opencode-go' ||
			provider.method === 'ollama' ||
			(provider.method === 'xai'
				? Boolean(auth.xaiAuthStatus?.authenticated)
				: provider.apiKey.trim().length > 0)
		);
	}

	async function refreshScreenCaptureAccess() {
		const current = await window.electronAPI?.getScreenCaptureAccess?.();
		if (!current) return;
		screenCapturePreferred = Boolean(current.preferred);
		screenCaptureStatus = current.status ?? 'unknown';
	}

	async function setWizardScreenCapture(enabled: boolean) {
		screenCaptureBusy = true;
		try {
			const result = await window.electronAPI?.setScreenCapturePreferred?.(enabled);
			if (result) {
				screenCapturePreferred = Boolean(result.preferred);
				screenCaptureStatus = result.status ?? 'unknown';
			} else {
				screenCapturePreferred = enabled;
			}
		} finally {
			screenCaptureBusy = false;
		}
	}

	function patchProvider(id: string, patch: Partial<ProviderConfig>) {
		draft = {
			...draft,
			providers: draft.providers.map((p) =>
				p.id === id ? cloneProvider({ ...p, ...patch }) : p
			)
		};
	}

	function toggleProvider(id: string) {
		if (selectedProviderIds.includes(id)) {
			selectedProviderIds = selectedProviderIds.filter((providerId) => providerId !== id);
			if (activeProviderId === id) activeProviderId = selectedProviderIds[0] ?? '';
			if (defaultProviderId === id) defaultProviderId = selectedProviderIds[0] ?? '';
			return;
		}
		selectedProviderIds = [...selectedProviderIds, id];
		if (!activeProviderId) activeProviderId = id;
		if (!defaultProviderId) defaultProviderId = id;
	}

	function showProvider(id: string) {
		activeProviderId = id;
		modelFilter = '';
	}

	function setDefaultProvider(id: string) {
		defaultProviderId = id;
	}

	function providerIsConnected(provider: ProviderConfig) {
		if (provider.method === 'codex') return Boolean(auth.codexAuthStatus?.authenticated);
		if (provider.method === 'xai') return Boolean(auth.xaiAuthStatus?.authenticated);
		if (provider.method === 'ollama') return Boolean(memory.ollamaHealth?.ok);
		return provider.apiKey.trim().length > 0;
	}

	function next() {
		const idx = STEP_ORDER.indexOf(step);
		if (idx < STEP_ORDER.length - 1) {
			const nextStep = STEP_ORDER[idx + 1];
			step = nextStep;
			// Auto-check Codex auth status when entering the apikey step for codex.
			if (
				nextStep === 'apikey' &&
				selectedProviders.some((provider) => provider.method === 'codex')
			) {
				void auth.refreshCodexAuthStatus();
			}
			if (nextStep === 'permissions') {
				void refreshScreenCaptureAccess();
			}
			if (nextStep === 'embedding') {
				void memory.loadMemorySettings();
			}
			if (
				nextStep === 'apikey' &&
				selectedProviders.some((provider) => provider.method === 'xai')
			) {
				void auth.refreshXaiAuthStatus();
			}
			if (
				(nextStep === 'apikey' || nextStep === 'embedding') &&
				(selectedProviders.some((provider) => provider.method === 'ollama') ||
					nextStep === 'embedding')
			) {
				void memory.refreshOllamaHealth();
			}
		}
	}

	function back() {
		const idx = STEP_ORDER.indexOf(step);
		if (idx > 0) {
			step = STEP_ORDER[idx - 1];
		}
	}

	async function fetchModels() {
		if (!selectedProvider || !canFetchModels(selectedProvider)) return;
		try {
			const updated = await settingsStore.fetchModelsFor(selectedProvider);
			patchProvider(selectedProvider.id, {
				models: updated.models,
				enabledModels: updated.enabledModels,
				selectedModel: updated.selectedModel
			});
		} catch {
			// Error surfaced via settingsStore.error; keep the wizard usable.
		}
	}

	function selectModel(model: string) {
		if (!selectedProvider) return;
		patchProvider(selectedProvider.id, toggleEnabledModel(selectedProvider, model));
	}

	async function saveAndConnect() {
		if (!defaultProvider || selectedProviderIds.length === 0) return;
		const finalDraft = completedSetupSettings(draft, selectedProviderIds, defaultProvider);

		// Apply embedding selection (may be empty = skipped).
		const memoryPayload = memory.applyEmbeddingSelection();
		const hasEmbedding = Boolean(memoryPayload?.embedding.model.trim());

		saving = true;
		saveError = '';
		try {
			await settingsStore.save(finalDraft, {
				restartCometMind: true,
				memory: hasEmbedding ? (memoryPayload ?? undefined) : undefined
			});
			draft = cloneCurrentSettings();
			await settingsStore.markSetupComplete();
			settingsSaved = true;
			// Poll until the sidecar is healthy after restart.
			connecting = true;
			connectionState.reconnect();
			const ready = await waitForReady(20);
			connecting = false;
			if (!ready) {
				saveError =
					'CometMind is still starting up. Your settings were saved — try sending a message in a moment.';
			}
			step = 'permissions';
			void refreshScreenCaptureAccess();
		} catch (err) {
			saveError = err instanceof Error ? err.message : 'Failed to save settings.';
		} finally {
			saving = false;
			connecting = false;
		}
	}

	function continueToPermissions() {
		step = 'permissions';
		void refreshScreenCaptureAccess();
	}

	function finishSetup() {
		shellStore.closeSetup();
	}

	function skip() {
		// Persist the dismissal so the wizard doesn't re-open on next launch.
		void settingsStore.markSetupDismissed();
		shellStore.closeSetup();
	}

	function toggleShowApiKey() {
		showApiKey = !showApiKey;
	}

	return {
		auth,
		memory,
		get step() {
			return step;
		},
		get stepIndex() {
			return stepIndex;
		},
		get canAdvance() {
			return canAdvance;
		},
		get saving() {
			return saving;
		},
		get saveError() {
			return saveError;
		},
		get settingsSaved() {
			return settingsSaved;
		},
		get connecting() {
			return connecting;
		},
		get modelFilter() {
			return modelFilter;
		},
		set modelFilter(value: string) {
			modelFilter = value;
		},
		get showApiKey() {
			return showApiKey;
		},
		get draft() {
			return draft;
		},
		get selectedProviderIds() {
			return selectedProviderIds;
		},
		get defaultProviderId() {
			return defaultProviderId;
		},
		get activeProviderId() {
			return activeProviderId;
		},
		get selectedProvider() {
			return selectedProvider;
		},
		get defaultProvider() {
			return defaultProvider;
		},
		get selectedProviders() {
			return selectedProviders;
		},
		get filteredModels() {
			return filteredModels;
		},
		get screenCapturePreferred() {
			return screenCapturePreferred;
		},
		get screenCaptureStatus() {
			return screenCaptureStatus;
		},
		get screenCaptureBusy() {
			return screenCaptureBusy;
		},
		canFetchModels,
		providerIsConnected,
		patchProvider,
		toggleProvider,
		showProvider,
		setDefaultProvider,
		next,
		back,
		fetchModels,
		selectModel,
		saveAndConnect,
		continueToPermissions,
		finishSetup,
		skip,
		toggleShowApiKey,
		setWizardScreenCapture
	};
}

export type SetupWizardController = ReturnType<typeof createSetupWizardController>;
