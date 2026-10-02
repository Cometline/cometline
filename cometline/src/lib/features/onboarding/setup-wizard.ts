import type { MemorySettings } from '#lib/client/cometmind.js';
import {
	embeddingOptionKey,
	embeddingProviderForMethod,
	type EmbeddingModelOption
} from '#lib/embedding-models.js';
import { cloneProvider } from '#lib/features/settings/schema.js';
import { getOllamaCatalogEntry } from '#lib/ollama/catalog.js';
import type { ProviderConfig, ProviderMethod, ProviderSettings } from '#lib/types.js';

export type SetupWizardStep =
	| 'provider'
	| 'apikey'
	| 'model'
	| 'embedding'
	| 'permissions'
	| 'connect';

export const STEP_ORDER: SetupWizardStep[] = [
	'provider',
	'apikey',
	'model',
	'embedding',
	'connect',
	'permissions'
];

export const STEP_TITLES: Record<SetupWizardStep, string> = {
	provider: 'Choose a provider',
	apikey: 'Connect your account',
	model: 'Pick a model',
	embedding: 'Memory embeddings (optional)',
	connect: 'Ready to go',
	permissions: 'Screen capture (optional)'
};

export const METHOD_LABELS: Record<ProviderMethod, string> = {
	openai: 'OpenAI',
	anthropic: 'Anthropic',
	'opencode-go': 'OpenCode Go',
	codex: 'ChatGPT Codex',
	xai: 'xAI Grok Subscription',
	ollama: 'Ollama Local',
	'openai-compatible': 'Advanced / Custom endpoint'
};

export const PRIVATE_MEMORY = getOllamaCatalogEntry('private-memory')!;
export const PRIVATE_MEMORY_KEY = `ollama:${PRIVATE_MEMORY.pullName}`;

type MemoryEmbedding = MemorySettings['embedding'];

export function providerLabel(provider: ProviderConfig): string {
	return provider.name || METHOD_LABELS[provider.method];
}

export function initialSelectedProviderIds(providers: ProviderConfig[]): string[] {
	const enabled = providers.filter((provider) => provider.enabled).map((provider) => provider.id);
	if (enabled.length > 0) return enabled;
	const initial = providers.find((provider) => provider.id === 'openai') ?? providers[0];
	return initial ? [initial.id] : [];
}

export function initialDefaultProviderId(
	draftDefaultProviderId: string,
	selectedProviderIds: string[]
): string {
	if (selectedProviderIds.includes(draftDefaultProviderId)) return draftDefaultProviderId;
	return selectedProviderIds[0] ?? '';
}

export function filterModels(models: string[], filter: string): string[] {
	const query = filter.trim().toLowerCase();
	if (!query) return models;
	return models.filter((model) => model.toLowerCase().includes(query));
}

export function toggleEnabledModel(
	provider: ProviderConfig,
	model: string
): Pick<ProviderConfig, 'enabledModels' | 'selectedModel'> {
	const enabledModels = provider.enabledModels.includes(model)
		? provider.enabledModels.filter((enabledModel) => enabledModel !== model)
		: [...provider.enabledModels, model];
	return {
		enabledModels,
		selectedModel: enabledModels.includes(provider.selectedModel)
			? provider.selectedModel
			: (enabledModels[0] ?? '')
	};
}

export function canAdvanceStep(
	step: SetupWizardStep,
	selectedProviderIds: string[],
	selectedProviders: ProviderConfig[],
	isConnected: (provider: ProviderConfig) => boolean
): boolean {
	if (step === 'provider') return selectedProviderIds.length > 0;
	if (step === 'apikey') {
		return selectedProviders.length > 0 && selectedProviders.every(isConnected);
	}
	if (step === 'model')
		return (
			selectedProviders.length > 0 &&
			selectedProviders.every((provider) => provider.enabledModels.length > 0)
		);
	// Embedding and permissions steps are always skippable.
	return true;
}

export function privateMemoryEmbedding(baseURL: string): MemoryEmbedding {
	return {
		provider_id: 'ollama',
		provider: 'ollama',
		model: PRIVATE_MEMORY.pullName,
		base_url: baseURL,
		api_key: ''
	};
}

export function withPrivateMemoryModel(
	providers: ProviderConfig[],
	baseURL: string | undefined,
	pulledModels: string[]
): ProviderConfig[] {
	return providers.map((p) =>
		p.id === 'ollama'
			? cloneProvider({
					...p,
					enabled: true,
					baseURL: baseURL || p.baseURL,
					models: Array.from(new Set([...p.models, ...pulledModels])),
					enabledModels: Array.from(
						new Set([...p.enabledModels, PRIVATE_MEMORY.pullName])
					)
				})
			: p
	);
}

export function embeddingSelectionPayload(
	memorySettings: MemorySettings | null,
	selectedEmbeddingKey: string,
	options: EmbeddingModelOption[],
	privateMemoryBaseURL: string
): MemorySettings | null {
	if (!memorySettings) return null;
	if (!selectedEmbeddingKey) {
		// No selection — clear embedding fields.
		return {
			...memorySettings,
			embedding: { provider_id: '', provider: '', model: '', base_url: '', api_key: '' }
		};
	}
	if (selectedEmbeddingKey === PRIVATE_MEMORY_KEY) {
		return {
			...memorySettings,
			embedding: privateMemoryEmbedding(privateMemoryBaseURL)
		};
	}
	const option = options.find((opt) => embeddingOptionKey(opt) === selectedEmbeddingKey);
	if (!option) return memorySettings;
	return {
		...memorySettings,
		embedding: {
			provider_id: option.providerId,
			provider: embeddingProviderForMethod(option.method),
			model: option.model,
			base_url: option.baseURL,
			api_key: option.apiKey
		}
	};
}

export function embeddingReviewLabel(
	selectedEmbeddingKey: string,
	options: EmbeddingModelOption[]
): string {
	if (selectedEmbeddingKey === PRIVATE_MEMORY_KEY) return `Local · ${PRIVATE_MEMORY.pullName}`;
	if (!selectedEmbeddingKey) return 'Skipped';
	return options.find((o) => embeddingOptionKey(o) === selectedEmbeddingKey)?.model ?? '—';
}

export function completedSetupSettings(
	draft: ProviderSettings,
	selectedProviderIds: string[],
	defaultProvider: ProviderConfig
): ProviderSettings {
	// The provider selection is authoritative for the completed setup.
	const finalProviders = draft.providers.map((p) =>
		cloneProvider({ ...p, enabled: selectedProviderIds.includes(p.id) })
	);
	return {
		...draft,
		providers: finalProviders,
		defaultProviderId: defaultProvider.id,
		defaultModelId: defaultProvider.selectedModel || defaultProvider.enabledModels[0] || ''
	};
}
