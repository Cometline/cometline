import type { Session } from '$lib/types';

export interface ModelOption {
	id: string;
	label: string;
	description: string;
	provider_id: string;
	model_id: string;
}

export const defaultModelOptions: ModelOption[] = [
	{
		id: 'openai:deepseek-v4-flash',
		label: 'DeepSeek V4 Flash',
		description: 'OpenAI-compatible endpoint',
		provider_id: 'openai',
		model_id: 'deepseek-v4-flash'
	},
	{
		id: 'anthropic:claude-sonnet-4-5',
		label: 'Claude Sonnet 4.5',
		description: 'Anthropic provider',
		provider_id: 'anthropic',
		model_id: 'claude-sonnet-4-5'
	}
];

function labelForModel(modelID: string) {
	return modelID
		.split(/[-_:/.]+/)
		.filter(Boolean)
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join(' ');
}

function createModelStore() {
	let options = $state<ModelOption[]>(defaultModelOptions);
	let selected = $state<ModelOption>(defaultModelOptions[0]);

	function select(option: ModelOption) {
		selected = option;
	}

	function selectByProviderModel(providerID: string, modelID: string) {
		const match = options.find(
			(option) => option.provider_id === providerID && option.model_id === modelID
		);
		selected =
			match ??
			{
				id: `${providerID}:${modelID}`,
				label: modelID,
				description: providerID,
				provider_id: providerID,
				model_id: modelID
			};
	}

	function selectFromSession(session: Session) {
		selectByProviderModel(session.provider_id, session.model_id);
	}

	function setProviderModels(providerID: string, models: string[], selectedModelID?: string) {
		const nextOptions = models.map((modelID) => ({
			id: `${providerID}:${modelID}`,
			label: labelForModel(modelID),
			description: providerID,
			provider_id: providerID,
			model_id: modelID
		}));
		options = nextOptions.length > 0 ? nextOptions : defaultModelOptions;
		selectByProviderModel(providerID, selectedModelID || options[0].model_id);
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
		setProviderModels
	};
}

export const modelStore = createModelStore();
