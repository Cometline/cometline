import { cloneCometMindSettings, normalizeCometMindSettings } from '$lib/cometmind-settings';
import type { MemorySettings } from '$lib/client/cometmind';
import { findProviderForSaved } from '$lib/embedding-models';
import { resolveDefaultModelPair } from '$lib/settings/schema';
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
		defaultModelId: settings.defaultModelId ?? '',
		defaultProviderId: settings.defaultProviderId ?? '',
		appearance: {
			heroComposer: {
				...settings.appearance.heroComposer,
				customPreset: settings.appearance.heroComposer.customPreset
					? { ...settings.appearance.heroComposer.customPreset }
					: undefined
			},
			caretTrail: { ...settings.appearance.caretTrail }
		},
		shortcuts: cloneShortcuts(settings),
		app: {
			openAtLogin: settings.app?.openAtLogin ?? false,
			hasSeenIntro: settings.app?.hasSeenIntro ?? false,
			hasCompletedSetup: settings.app?.hasCompletedSetup ?? false,
			hasDismissedSetupWizard: settings.app?.hasDismissedSetupWizard ?? false,
			personaId: settings.app?.personaId ?? 'minako',
			personas: { custom: settings.app?.personas?.custom ?? [] },
			miniWindowSessionId: settings.app?.miniWindowSessionId ?? '',
			miniWindowLastActiveAt: settings.app?.miniWindowLastActiveAt ?? 0,
			miniWindowInactivityTimeoutMinutes:
				settings.app?.miniWindowInactivityTimeoutMinutes ?? 30,
			webPanelWidth: settings.app?.webPanelWidth ?? 0,
			webPanelRatio: settings.app?.webPanelRatio ?? 0,
			confirmCloseOnCmdW: settings.app?.confirmCloseOnCmdW ?? true,
			confirmBeforeDeletingChats: settings.app?.confirmBeforeDeletingChats ?? true
		},
		cometmind: cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind))
	};
}

export function providerPayloadFromDraft(draft: ProviderSettings): ProviderSettings {
	const { defaultProviderId, defaultModelId } = resolveDefaultModelPair(
		draft.providers,
		draft.defaultProviderId,
		draft.defaultModelId
	);
	return {
		providers: draft.providers.map(cloneProvider),
		defaultModelId,
		defaultProviderId,
		appearance: {
			heroComposer: {
				...draft.appearance.heroComposer,
				customPreset: draft.appearance.heroComposer.customPreset
					? { ...draft.appearance.heroComposer.customPreset }
					: undefined
			},
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
	return applyMemorySettingsToDraft(draft, {
		enabled: draft.cometmind.memory.enabled,
		auto_extract: draft.cometmind.memory.autoExtract,
		auto_retrieve: draft.cometmind.memory.autoRetrieve,
		max_retrieved: draft.cometmind.memory.maxRetrieved,
		task_outcome_limit: draft.cometmind.memory.taskOutcomeLimit,
		similarity_threshold: draft.cometmind.memory.similarityThreshold,
		extraction_model: draft.cometmind.memory.extractionModel,
		lifecycle: {
			decay_half_life_days: draft.cometmind.memory.lifecycle.decayHalfLifeDays,
			forget_threshold: draft.cometmind.memory.lifecycle.forgetThreshold,
			usage_boost_factor: draft.cometmind.memory.lifecycle.usageBoostFactor,
			max_usage_boost: draft.cometmind.memory.lifecycle.maxUsageBoost,
			max_memories: draft.cometmind.memory.lifecycle.maxMemories,
			compaction_target_ratio: draft.cometmind.memory.lifecycle.compactionTargetRatio,
			compaction_on_extract: draft.cometmind.memory.lifecycle.compactionOnExtract
		},
		embedding: {
			provider_id: embedding.provider_id,
			provider: embedding.provider,
			model: embedding.model,
			base_url: embedding.base_url,
			api_key: embedding.api_key
		}
	});
}

export function applyMemorySettingsToDraft(
	draft: ProviderSettings,
	memory: MemorySettings
): ProviderSettings {
	const embedding = memory.embedding ?? {};
	let providerId = String(embedding.provider_id ?? '').trim();
	const model = String(embedding.model ?? '').trim();
	if (model) {
		const matched = findProviderForSaved(draft.providers, {
			providerId,
			provider: String(embedding.provider ?? '').trim(),
			model,
			baseURL: String(embedding.base_url ?? '').trim(),
			apiKey: String(embedding.api_key ?? '').trim()
		});
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
				enabled: memory.enabled ?? draft.cometmind.memory.enabled,
				autoExtract: memory.auto_extract ?? draft.cometmind.memory.autoExtract,
				autoRetrieve: memory.auto_retrieve ?? draft.cometmind.memory.autoRetrieve,
				maxRetrieved: memory.max_retrieved ?? draft.cometmind.memory.maxRetrieved,
				taskOutcomeLimit:
					memory.task_outcome_limit ?? draft.cometmind.memory.taskOutcomeLimit,
				similarityThreshold:
					memory.similarity_threshold ?? draft.cometmind.memory.similarityThreshold,
				extractionProviderId: draft.cometmind.memory.extractionProviderId,
				extractionModel: memory.extraction_model ?? draft.cometmind.memory.extractionModel,
				lifecycle: {
					decayHalfLifeDays:
						memory.lifecycle?.decay_half_life_days ??
						draft.cometmind.memory.lifecycle.decayHalfLifeDays,
					forgetThreshold:
						memory.lifecycle?.forget_threshold ??
						draft.cometmind.memory.lifecycle.forgetThreshold,
					usageBoostFactor:
						memory.lifecycle?.usage_boost_factor ??
						draft.cometmind.memory.lifecycle.usageBoostFactor,
					maxUsageBoost:
						memory.lifecycle?.max_usage_boost ??
						draft.cometmind.memory.lifecycle.maxUsageBoost,
					maxMemories:
						memory.lifecycle?.max_memories ??
						draft.cometmind.memory.lifecycle.maxMemories,
					compactionTargetRatio:
						memory.lifecycle?.compaction_target_ratio ??
						draft.cometmind.memory.lifecycle.compactionTargetRatio,
					compactionOnExtract:
						memory.lifecycle?.compaction_on_extract ??
						draft.cometmind.memory.lifecycle.compactionOnExtract
				},
				embedding: {
					providerId: providerId || embedding.provider_id,
					provider: embedding.provider ?? '',
					model: embedding.model ?? '',
					baseURL: embedding.base_url ?? '',
					apiKey: embedding.api_key ?? ''
				}
			}
		}
	};
}
