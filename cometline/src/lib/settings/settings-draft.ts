import { cloneCometMindSettings, normalizeCometMindSettings } from '$lib/cometmind-settings';
import type { MemorySettings } from '$lib/client/cometmind';
import type { ProviderConfig, ProviderSettings } from '$lib/types';

export function cloneProvider(provider: ProviderConfig): ProviderConfig {
	return {
		...provider,
		models: [...provider.models],
		enabledModels: [...provider.enabledModels]
	};
}

export function cloneShortcuts(settings: ProviderSettings): ProviderSettings['shortcuts'] {
	return Object.fromEntries(
		Object.entries(settings.shortcuts).map(([id, binding]) => [
			id,
			binding ? { ...binding } : binding
		])
	) as ProviderSettings['shortcuts'];
}

export function cloneSettings(settings: ProviderSettings): ProviderSettings {
	return {
		providers: settings.providers.map(cloneProvider),
		activeProviderId: settings.activeProviderId,
		defaultModelId: settings.defaultModelId ?? '',
		defaultProviderId: settings.defaultProviderId ?? '',
		appearance: {
			heroComposer: { ...settings.appearance.heroComposer },
			caretTrail: { ...settings.appearance.caretTrail }
		},
		shortcuts: cloneShortcuts(settings),
		app: {
			openAtLogin: settings.app?.openAtLogin ?? false,
			hasSeenIntro: settings.app?.hasSeenIntro ?? false,
			hasCompletedSetup: settings.app?.hasCompletedSetup ?? false,
			hasDismissedSetupWizard: settings.app?.hasDismissedSetupWizard ?? false,
			iconVariant: settings.app?.iconVariant ?? 'default',
			miniWindowSessionId: settings.app?.miniWindowSessionId ?? '',
			miniWindowLastActiveAt: settings.app?.miniWindowLastActiveAt ?? 0,
			miniWindowInactivityTimeoutMinutes:
				settings.app?.miniWindowInactivityTimeoutMinutes ?? 30,
			webPanelWidth: settings.app?.webPanelWidth ?? 0
		},
		cometmind: cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind))
	};
}

export function providerPayloadFromDraft(draft: ProviderSettings): ProviderSettings {
	const activeProvider =
		draft.providers.find(
			(provider) => provider.enabled && provider.enabledModels.length > 0
		) ??
		draft.providers.find((provider) => provider.enabled) ??
		draft.providers[0];
	return {
		providers: draft.providers.map(cloneProvider),
		activeProviderId: activeProvider?.id ?? '',
		defaultModelId: draft.defaultModelId ?? '',
		defaultProviderId: draft.defaultProviderId ?? '',
		appearance: {
			heroComposer: { ...draft.appearance.heroComposer },
			caretTrail: { ...draft.appearance.caretTrail }
		},
		shortcuts: cloneShortcuts(draft),
		app: { ...draft.app },
		cometmind: cloneCometMindSettings(draft.cometmind)
	};
}

export function applyMemoryEmbeddingToDraft(
	draft: ProviderSettings,
	embedding: MemorySettings['embedding']
): ProviderSettings {
	let providerId = embedding.provider_id.trim();
	const model = embedding.model.trim();
	if ((!providerId || providerId === '__saved__') && model) {
		const matched = draft.providers.find(
			(p) => p.enabledModels.includes(model) || p.models.includes(model)
		);
		if (matched) providerId = matched.id;
	}
	let nextProviders = draft.providers.map(cloneProvider);
	if (providerId && model) {
		nextProviders = nextProviders.map((provider) => {
			if (provider.id !== providerId) return provider;
			const enabledModels = provider.enabledModels.includes(model)
				? provider.enabledModels
				: [...provider.enabledModels, model];
			const models = provider.models.includes(model)
				? provider.models
				: [...provider.models, model];
			return { ...provider, enabled: true, models, enabledModels };
		});
	}
	return {
		...draft,
		providers: nextProviders,
		cometmind: {
			...draft.cometmind,
			memory: {
				...draft.cometmind.memory,
				embedding: {
					providerId: providerId || embedding.provider_id,
					provider: embedding.provider,
					model: embedding.model,
					baseURL: embedding.base_url,
					apiKey: embedding.api_key ?? ''
				}
			}
		}
	};
}
