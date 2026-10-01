import type { ProviderConfig } from '$lib/types';
import { isEmbeddingModelName } from '$lib/embedding-models';

export interface ModelEntry {
	id: string;
	label: string;
	providerId: string;
	providerName: string;
	modelId: string;
}

export interface ModelEntryGroup {
	providerId: string;
	providerName: string;
	options: ModelEntry[];
}

export function labelForModel(modelID: string) {
	return modelID
		.split(/[_/]+/)
		.filter(Boolean)
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1).toUpperCase())
		.join(' ');
}

export function buildModelOptions(providers: ProviderConfig[]): ModelEntry[] {
	const options: ModelEntry[] = [];
	for (const provider of providers) {
		if (!provider.enabled) continue;
		for (const modelId of provider.enabledModels) {
			if (isEmbeddingModelName(modelId)) continue;
			options.push({
				id: `${provider.id}:${modelId}`,
				label: labelForModel(modelId),
				providerId: provider.id,
				providerName: provider.name || provider.id,
				modelId
			});
		}
	}
	return options;
}

export function filterModelOptions(options: ModelEntry[], search: string): ModelEntry[] {
	const query = search.trim().toLowerCase();
	if (!query) return options;
	return options.filter(
		(option) =>
			option.label.toLowerCase().includes(query) ||
			option.modelId.toLowerCase().includes(query) ||
			option.providerName.toLowerCase().includes(query)
	);
}

export function groupModelOptions(options: ModelEntry[]): ModelEntryGroup[] {
	const groups: ModelEntryGroup[] = [];
	for (const option of options) {
		let group = groups.find((item) => item.providerId === option.providerId);
		if (!group) {
			group = {
				providerId: option.providerId,
				providerName: option.providerName,
				options: []
			};
			groups.push(group);
		}
		group.options.push(option);
	}
	return groups;
}

export function selectedModelLabel(
	options: ModelEntry[],
	providerId: string,
	modelId: string
): string {
	const match = options.find((o) => o.providerId === providerId && o.modelId === modelId);
	return match ? `${match.providerName} · ${match.modelId}` : 'No model selected';
}
