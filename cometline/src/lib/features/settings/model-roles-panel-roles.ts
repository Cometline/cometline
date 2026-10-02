import type { CometMindSettings } from '#lib/cometmind-settings.js';
import type { ProviderConfig } from '#lib/types.js';

export function modelsForProvider(provider: ProviderConfig | undefined): string[] {
	if (!provider) return [];
	return [...provider.enabledModels];
}

function firstModelForProvider(provider: ProviderConfig | undefined): string {
	return provider?.enabledModels[0] ?? '';
}

export function providerById(providers: ProviderConfig[], providerId: string) {
	return providers.find((provider) => provider.id === providerId);
}

export function providerOptionLabel(provider: ProviderConfig): string {
	return provider.method === 'ollama' ? `${provider.name} (Local)` : provider.name;
}

export function withTitleProvider(
	cometmind: CometMindSettings,
	providers: ProviderConfig[],
	providerId: string
): CometMindSettings {
	if (!providerId) {
		return { ...cometmind, titleProviderId: '', titleModelId: '' };
	}
	const modelId = firstModelForProvider(providerById(providers, providerId));
	return { ...cometmind, titleProviderId: providerId, titleModelId: modelId };
}

export function withTitleModel(cometmind: CometMindSettings, modelId: string): CometMindSettings {
	return { ...cometmind, titleModelId: modelId };
}

export function withExtractionProvider(
	cometmind: CometMindSettings,
	providers: ProviderConfig[],
	providerId: string
): CometMindSettings {
	if (!providerId) {
		return {
			...cometmind,
			memory: { ...cometmind.memory, extractionProviderId: '', extractionModel: '' }
		};
	}
	const modelId = firstModelForProvider(providerById(providers, providerId));
	return {
		...cometmind,
		memory: {
			...cometmind.memory,
			extractionProviderId: providerId,
			extractionModel: modelId
		}
	};
}

export function withExtractionModel(
	cometmind: CometMindSettings,
	modelId: string
): CometMindSettings {
	return {
		...cometmind,
		memory: { ...cometmind.memory, extractionModel: modelId }
	};
}

export function withAutonomyProvider(
	cometmind: CometMindSettings,
	providers: ProviderConfig[],
	providerId: string
): CometMindSettings {
	if (!providerId) {
		return {
			...cometmind,
			autonomy: { ...cometmind.autonomy, providerId: '', modelId: '' }
		};
	}
	const provider = providerById(providers, providerId) ?? providers[0];
	return {
		...cometmind,
		autonomy: {
			...cometmind.autonomy,
			providerId: provider?.id ?? '',
			modelId: firstModelForProvider(provider)
		}
	};
}

export function withAutonomyModel(
	cometmind: CometMindSettings,
	modelId: string
): CometMindSettings {
	return { ...cometmind, autonomy: { ...cometmind.autonomy, modelId } };
}

export function withSynthesisProvider(
	cometmind: CometMindSettings,
	providers: ProviderConfig[],
	providerId: string
): CometMindSettings {
	if (!providerId) {
		return {
			...cometmind,
			skills: { ...cometmind.skills, synthesisProviderId: '', synthesisModel: '' }
		};
	}
	const provider = providerById(providers, providerId) ?? providers[0];
	return {
		...cometmind,
		skills: {
			...cometmind.skills,
			synthesisProviderId: provider?.id ?? '',
			synthesisModel: firstModelForProvider(provider)
		}
	};
}

export function withSynthesisModel(
	cometmind: CometMindSettings,
	modelId: string
): CometMindSettings {
	return { ...cometmind, skills: { ...cometmind.skills, synthesisModel: modelId } };
}
