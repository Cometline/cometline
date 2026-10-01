import {
	buildEmbeddingDropdownOptions,
	embeddingKeyForFields,
	embeddingOptionKey,
	mergeEmbeddingFields,
	savedEmbeddingFromApi,
	type SavedEmbeddingRef
} from '$lib/embedding-models';
import {
	defaultMemorySettings,
	getMemorySettings,
	type MemorySettings
} from '$lib/client/cometmind';
import type { ProviderConfig, ProviderSettings } from '$lib/types';
import { OLLAMA_DEFAULT_NATIVE_BASE } from '$lib/ollama/catalog';
import { checkOllamaHealth, pullOllamaModel, type OllamaHealthResult } from '$lib/ollama/client';
import {
	embeddingSelectionPayload,
	PRIVATE_MEMORY,
	PRIVATE_MEMORY_KEY,
	privateMemoryEmbedding,
	withPrivateMemoryModel
} from './setup-wizard';

export function createSetupWizardMemory(deps: {
	getDraft: () => ProviderSettings;
	setDraft: (draft: ProviderSettings) => void;
	getSelectedProviderIds: () => string[];
	patchProvider: (id: string, patch: Partial<ProviderConfig>) => void;
}) {
	let memorySettings = $state<MemorySettings | null>(null);
	let memoryLoading = $state(false);
	let memoryError = $state('');
	let selectedEmbeddingKey = $state('');
	let savedEmbedding = $state<SavedEmbeddingRef | undefined>();
	let ollamaHealth = $state<OllamaHealthResult | null>(null);
	let checkingOllama = $state(false);
	let pullingPrivateMemory = $state(false);
	let ollamaWizardError = $state('');

	// Embedding dropdown options derived from enabled providers.
	const embeddingOptions = $derived(
		buildEmbeddingDropdownOptions(
			deps
				.getDraft()
				.providers.map((provider) =>
					deps.getSelectedProviderIds().includes(provider.id)
						? { ...provider, enabled: true }
						: provider
				),
			savedEmbedding,
			memorySettings?.embedding
		)
	);
	const availableEmbeddingOptions = $derived(
		embeddingOptions.filter(
			(option) =>
				(option.method !== 'ollama' || ollamaHealth?.ok) &&
				embeddingOptionKey(option) !== PRIVATE_MEMORY_KEY
		)
	);

	async function refreshOllamaHealth() {
		checkingOllama = true;
		ollamaWizardError = '';
		try {
			const ollamaProvider = deps
				.getDraft()
				.providers.find((provider) => provider.method === 'ollama');
			ollamaHealth = await checkOllamaHealth(
				ollamaProvider?.baseURL || OLLAMA_DEFAULT_NATIVE_BASE
			);
			if (ollamaHealth.ok && ollamaProvider) {
				deps.patchProvider(ollamaProvider.id, { baseURL: ollamaHealth.baseURL });
			} else if (!ollamaHealth.ok && selectedEmbeddingKey.startsWith('ollama:')) {
				selectedEmbeddingKey = '';
			}
		} catch (err) {
			ollamaWizardError = err instanceof Error ? err.message : String(err);
		} finally {
			checkingOllama = false;
		}
	}

	async function recommendPrivateMemory() {
		pullingPrivateMemory = true;
		ollamaWizardError = '';
		try {
			if (!ollamaHealth?.ok) {
				await refreshOllamaHealth();
			}
			if (!ollamaHealth?.ok) {
				throw new Error('Start Ollama first, then pull Private Memory.');
			}
			const result = await pullOllamaModel({
				baseURL: ollamaHealth.baseURL,
				catalogId: PRIVATE_MEMORY.id
			});
			const names = result.models.map((m) => m.name);
			const draft = deps.getDraft();
			const ollamaProvider = draft.providers.find((p) => p.id === 'ollama');
			if (ollamaProvider) {
				deps.setDraft({
					...draft,
					providers: withPrivateMemoryModel(draft.providers, ollamaHealth?.baseURL, names)
				});
			}
			selectedEmbeddingKey = PRIVATE_MEMORY_KEY;
			if (memorySettings) {
				memorySettings = {
					...memorySettings,
					embedding: privateMemoryEmbedding(ollamaHealth.baseURL)
				};
			}
		} catch (err) {
			ollamaWizardError = err instanceof Error ? err.message : String(err);
		} finally {
			pullingPrivateMemory = false;
		}
	}

	async function loadMemorySettings() {
		if (memoryLoading || memorySettings) return;
		memoryLoading = true;
		memoryError = '';
		try {
			const s = await getMemorySettings();
			memorySettings = s;
			savedEmbedding = savedEmbeddingFromApi(s.embedding);
			selectedEmbeddingKey = embeddingKeyForFields(
				deps.getDraft().providers,
				mergeEmbeddingFields(s.embedding, savedEmbedding),
				savedEmbedding
			);
			if (ollamaHealth && !ollamaHealth.ok && selectedEmbeddingKey.startsWith('ollama:')) {
				selectedEmbeddingKey = '';
			}
		} catch (err) {
			memoryError = err instanceof Error ? err.message : 'Failed to load memory settings';
			memorySettings = defaultMemorySettings();
		} finally {
			memoryLoading = false;
		}
	}

	function selectEmbedding(key: string) {
		selectedEmbeddingKey = key;
	}

	function applyEmbeddingSelection(): MemorySettings | null {
		return embeddingSelectionPayload(
			memorySettings,
			selectedEmbeddingKey,
			availableEmbeddingOptions,
			ollamaHealth?.baseURL || OLLAMA_DEFAULT_NATIVE_BASE
		);
	}

	return {
		get memoryLoading() {
			return memoryLoading;
		},
		get memoryError() {
			return memoryError;
		},
		get selectedEmbeddingKey() {
			return selectedEmbeddingKey;
		},
		get ollamaHealth() {
			return ollamaHealth;
		},
		get checkingOllama() {
			return checkingOllama;
		},
		get pullingPrivateMemory() {
			return pullingPrivateMemory;
		},
		get ollamaWizardError() {
			return ollamaWizardError;
		},
		get availableEmbeddingOptions() {
			return availableEmbeddingOptions;
		},
		refreshOllamaHealth,
		recommendPrivateMemory,
		loadMemorySettings,
		selectEmbedding,
		applyEmbeddingSelection
	};
}

export type SetupWizardMemory = ReturnType<typeof createSetupWizardMemory>;
