import type { ProviderConfig, ProviderMethod, Session } from '$lib/types';

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
	return provider.enabledModels.map((modelId) => ({
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

	function select(option: ModelOption) {
		selected = option;
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

	function setProviders(providers: ProviderConfig[]) {
		const nextOptions = providers.flatMap(optionsFromProvider);
		options = nextOptions;

		if (selected && options.some((option) => option.id === selected?.id)) {
			return;
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
		selectByProviderModel,
		selectFromSession,
		setProviders,
		updateProviderModels
	};
}

export const modelStore = createModelStore();
