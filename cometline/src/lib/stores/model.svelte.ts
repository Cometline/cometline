import type { ProviderConfig, ProviderMethod, Session } from '$lib/types';
import { isEmbeddingModelName } from '$lib/embedding-models';
import type { InputModality } from '$lib/model-modalities';

export type ModelLimitSource = 'catalog' | 'fallback';

export interface ModelOption {
	id: string;
	label: string;
	providerId: string;
	providerName: string;
	providerMethod: ProviderMethod;
	modelId: string;
	/** Resolved context window tokens (models.dev or fallback). */
	context?: number;
	/** Catalog max output tokens when known; 0/undefined when unset. */
	output?: number;
	limitSource?: ModelLimitSource;
	vision?: boolean;
	visionKnown?: boolean;
	inputModalities?: InputModality[];
	/** Allowed reasoning effort values when the catalog knows them. */
	reasoningEffortOptions?: string[];
}

export type ModelLimitEntry = {
	providerId: string;
	modelId: string;
	context: number;
	output: number;
	limitSource: ModelLimitSource;
	vision: boolean;
	visionKnown: boolean;
	inputModalities: InputModality[];
	reasoningEffortOptions: string[];
};

function labelForModel(modelID: string) {
	return modelID
		.split(/[_/]+/)
		.filter(Boolean)
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1).toUpperCase())
		.join(' ');
}

function limitFields(limit: ModelLimitEntry) {
	return {
		context: limit.context,
		output: limit.output,
		limitSource: limit.limitSource,
		vision: limit.vision,
		visionKnown: limit.visionKnown,
		inputModalities: limit.inputModalities,
		reasoningEffortOptions: limit.reasoningEffortOptions
	};
}

function optionsFromProvider(
	provider: ProviderConfig,
	limitsByKey: Map<string, ModelLimitEntry>
): ModelOption[] {
	if (!provider.enabled) return [];
	return provider.enabledModels
		.filter((modelId) => !isEmbeddingModelName(modelId))
		.map((modelId) => {
			const limit = limitsByKey.get(`${provider.id}\0${modelId}`);
			return {
				id: `${provider.id}:${modelId}`,
				label: labelForModel(modelId),
				providerId: provider.id,
				providerName: provider.name || provider.id,
				providerMethod: provider.method,
				modelId,
				...(limit ? limitFields(limit) : {})
			};
		});
}

function createModelStore() {
	let options = $state<ModelOption[]>([]);
	let selected = $state<ModelOption | null>(null);
	let defaultProviderId = '';
	let defaultModelId = '';
	let limitsByKey = $state(new Map<string, ModelLimitEntry>());

	function syncSelected() {
		if (selected) {
			const match = options.find((option) => option.id === selected?.id);
			if (match) {
				selected = match;
				return;
			}
		}
		if (defaultProviderId && defaultModelId) {
			const defaultOption = options.find(
				(option) =>
					option.providerId === defaultProviderId && option.modelId === defaultModelId
			);
			if (defaultOption) {
				selected = defaultOption;
				return;
			}
		}
		selected = options[0] ?? null;
	}

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
		const limit = limitsByKey.get(`${providerId}\0${modelId}`);
		selected = match ?? {
			id: `${providerId}:${modelId}`,
			label: labelForModel(modelId),
			providerId,
			providerName: providerId,
			providerMethod: 'openai-compatible',
			modelId,
			...(limit ? limitFields(limit) : {})
		};
	}

	function selectFromSession(session: Session) {
		selectByProviderModel(session.provider_id, session.model_id);
	}

	function setProviders(
		providers: ProviderConfig[],
		nextDefaultProviderId?: string,
		nextDefaultModelId?: string
	) {
		defaultProviderId = nextDefaultProviderId ?? '';
		defaultModelId = nextDefaultModelId ?? '';
		options = providers.flatMap((provider) => optionsFromProvider(provider, limitsByKey));
		syncSelected();
	}

	function updateProviderModels(provider: ProviderConfig) {
		const withoutProvider = options.filter((option) => option.providerId !== provider.id);
		options = [...withoutProvider, ...optionsFromProvider(provider, limitsByKey)];
		if (!selected || !options.some((option) => option.id === selected?.id)) {
			selected = options[0] ?? null;
		} else {
			syncSelected();
		}
	}

	/** Merge catalog limits (fetch-time or reload); does not wipe other keys. */
	function applyLimits(entries: ModelLimitEntry[]) {
		const next = new Map(limitsByKey);
		for (const entry of entries) {
			const providerId = entry.providerId.trim();
			const modelId = entry.modelId.trim();
			if (!providerId || !modelId) continue;
		next.set(`${providerId}\0${modelId}`, {
			providerId,
			modelId,
			context: entry.context,
			output: entry.output,
			limitSource: entry.limitSource,
			vision: entry.vision,
			visionKnown: entry.visionKnown,
			inputModalities: [...entry.inputModalities],
			reasoningEffortOptions: entry.reasoningEffortOptions
				? [...entry.reasoningEffortOptions]
				: []
		});
		}
		limitsByKey = next;
		options = options.map((option) => {
			const limit = limitsByKey.get(`${option.providerId}\0${option.modelId}`);
			if (!limit) return option;
			return { ...option, ...limitFields(limit) };
		});
		syncSelected();
	}

	function limitFor(providerId: string, modelId: string): ModelLimitEntry | null {
		return limitsByKey.get(`${providerId}\0${modelId}`) ?? null;
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
		updateProviderModels,
		applyLimits,
		limitFor
	};
}

export const modelStore = createModelStore();
