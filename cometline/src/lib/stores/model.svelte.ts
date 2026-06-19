import type { ProviderConfig, ProviderMethod, Session } from '$lib/types';
import { isEmbeddingModelName } from '$lib/embedding-models';

export interface ModelOption {
	id: string;
	label: string;
	providerId: string;
	providerName: string;
	providerMethod: ProviderMethod;
	modelId: string;
}

function labelForModel(modelID: string) {
	return modelID
		.split(/[_/]+/)
		.filter(Boolean)
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1).toUpperCase())
		.join(' ');
}

function optionsFromProvider(provider: ProviderConfig): ModelOption[] {
	if (!provider.enabled) return [];
	return provider.enabledModels
		.filter((modelId) => !isEmbeddingModelName(modelId))
		.map((modelId) => ({
			id: `${provider.id}:${modelId}`,
			label: labelForModel(modelId),
			providerId: provider.id,
			providerName: provider.name || provider.id,
			providerMethod: provider.method,
			modelId
		}));
}

function createModelStore() {
	let options = $state<ModelOption[]>([]);
	let selected = $state<ModelOption | null>(null);
	let defaultProviderId = '';
	let defaultModelId = '';

	function select(option: ModelOption) {
		selected = option;
	}

	function selectDefault() {
		if (defaultProviderId && defaultModelId) {
			const match = options.find(
				(o) => o.providerId === defaultProviderId && o.modelId === defaultModelId
			);
			if (match) {
				selected = match;
				return;
			}
		}
		selected = options[0] ?? null;
	}

	function selectByProviderModel(providerId: string, modelId: string) {
		if (!modelId) {
			selected = options[0] ?? null;
			return;
		}
		const match = options.find(
			(option) => option.providerId === providerId && option.modelId === modelId
		);
		selected = match ?? {
			id: `${providerId}:${modelId}`,
			label: labelForModel(modelId),
			providerId,
			providerName: providerId,
			providerMethod: 'openai-compatible',
			modelId
		};
	}

	function selectFromSession(session: Session) {
		selectByProviderModel(session.provider_id, session.model_id);
	}

	function setProviders(providers: ProviderConfig[], nextDefaultProviderId?: string, nextDefaultModelId?: string) {
		defaultProviderId = nextDefaultProviderId ?? '';
		defaultModelId = nextDefaultModelId ?? '';
		const nextOptions = providers.flatMap(optionsFromProvider);
		options = nextOptions;

		if (selected && options.some((option) => option.id === selected?.id)) {
			return;
		}
		if (defaultProviderId && defaultModelId) {
			const defaultOption = options.find(
				(option) => option.providerId === defaultProviderId && option.modelId === defaultModelId
			);
			if (defaultOption) {
				selected = defaultOption;
				return;
			}
		}
		selected = options[0] ?? null;
	}

	function updateProviderModels(provider: ProviderConfig) {
		const withoutProvider = options.filter((option) => option.providerId !== provider.id);
		options = [...withoutProvider, ...optionsFromProvider(provider)];
		if (!selected || !options.some((option) => option.id === selected?.id)) {
			selected = options[0] ?? null;
		}
	}

	return {
		get options() {
			return options;
		},
		get selected() {
			return selected;
		},
		select,
		selectDefault,
		selectByProviderModel,
		selectFromSession,
		setProviders,
		updateProviderModels
	};
}

export const modelStore = createModelStore();
