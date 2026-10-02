import { isEmbeddingModelName } from '#lib/embedding-models.js';
import { defaultCometMindSettings, type CometMindSettings } from '#lib/cometmind-settings.js';
import type { ProviderConfig, ProviderSettings } from '#lib/types.js';

export type ModelRoleSelection = {
	providerId: string;
	modelId: string;
};

export function runtimeRoleProviders(providers: ProviderConfig[]): ProviderConfig[] {
	return providers.filter((provider) => provider.enabled && provider.enabledModels.length > 0);
}

export function defaultModelOptions(providers: ProviderConfig[]): ModelRoleSelection[] {
	const options: ModelRoleSelection[] = [];
	for (const provider of providers) {
		if (!provider.enabled) continue;
		for (const modelId of provider.enabledModels) {
			if (isEmbeddingModelName(modelId)) continue;
			options.push({ providerId: provider.id, modelId });
		}
	}
	return options;
}

export function resolveDefaultModelSelection(
	options: ModelRoleSelection[],
	providerId: string,
	modelId: string
): ModelRoleSelection {
	const selected = options.find(
		(option) => option.providerId === providerId && option.modelId === modelId
	);
	if (selected) return { providerId, modelId };
	const first = options[0];
	if (first) return { providerId: first.providerId, modelId: first.modelId };
	return { providerId, modelId };
}

function clearPin(
	cometmind: CometMindSettings,
	isRuntime: (providerId: string) => boolean
): { next: CometMindSettings; changed: boolean } {
	let next = cometmind;
	let changed = false;

	function replace(updated: CometMindSettings) {
		next = updated;
		changed = true;
	}

	if (next.titleProviderId && !isRuntime(next.titleProviderId)) {
		replace({ ...next, titleProviderId: '', titleModelId: '' });
	}
	if (next.memory.extractionProviderId && !isRuntime(next.memory.extractionProviderId)) {
		replace({
			...next,
			memory: { ...next.memory, extractionProviderId: '', extractionModel: '' }
		});
	}
	if (next.autonomy.providerId && !isRuntime(next.autonomy.providerId)) {
		replace({
			...next,
			autonomy: { ...next.autonomy, providerId: '', modelId: '' }
		});
	}
	if (next.skills.synthesisProviderId && !isRuntime(next.skills.synthesisProviderId)) {
		replace({
			...next,
			skills: { ...next.skills, synthesisProviderId: '', synthesisModel: '' }
		});
	}
	return { next, changed };
}

export function normalizeModelRoleDraft(draft: ProviderSettings): ProviderSettings {
	const providers = runtimeRoleProviders(draft.providers);
	const isRuntime = (providerId: string) =>
		providers.some((provider) => provider.id === providerId);
	const selection = resolveDefaultModelSelection(
		defaultModelOptions(draft.providers),
		draft.defaultProviderId,
		draft.defaultModelId
	);
	const pins = clearPin(draft.cometmind ?? defaultCometMindSettings(), isRuntime);
	const defaultChanged =
		selection.providerId !== draft.defaultProviderId ||
		selection.modelId !== draft.defaultModelId;
	if (!defaultChanged && !pins.changed) return draft;
	return {
		...draft,
		defaultProviderId: selection.providerId,
		defaultModelId: selection.modelId,
		cometmind: pins.next
	};
}
