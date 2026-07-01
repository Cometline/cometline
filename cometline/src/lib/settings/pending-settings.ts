import { cloneCometMindSettings, normalizeCometMindSettings } from '$lib/cometmind-settings';
import type { ProviderConfig, ProviderSettings } from '$lib/types';

export type SettingsSection = 'models' | 'memory' | 'agent' | 'appearance' | 'shortcuts' | 'app';

function cloneProvider(provider: ProviderConfig): ProviderConfig {
	return {
		...provider,
		models: [...provider.models],
		enabledModels: [...provider.enabledModels]
	};
}

/** Fields that require Save changes (excludes instant-tier: shortcuts, openAtLogin, discord.enabled). */
export function pendingSettingsSnapshot(settings: ProviderSettings): string {
	const cometmind = cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind));
	const { enabled: _discordEnabled, ...discordPending } = cometmind.gateway.discord;

	return JSON.stringify({
		providers: settings.providers.map(cloneProvider),
		activeProviderId: settings.activeProviderId,
		defaultModelId: settings.defaultModelId ?? '',
		defaultProviderId: settings.defaultProviderId ?? '',
		appearance: {
			heroComposer: { ...settings.appearance.heroComposer },
			caretTrail: { ...settings.appearance.caretTrail }
		},
		app: {
			personaId: settings.app?.personaId ?? 'minako',
			personas: settings.app?.personas?.custom ?? [],
			miniWindowInactivityTimeoutMinutes:
				settings.app?.miniWindowInactivityTimeoutMinutes ?? 30
		},
		cometmind: {
			...cometmind,
			gateway: { discord: discordPending }
		}
	});
}

export function settingsPendingDirty(
	draft: ProviderSettings,
	persisted: ProviderSettings
): boolean {
	return pendingSettingsSnapshot(draft) !== pendingSettingsSnapshot(persisted);
}

function modelsSectionSnapshot(settings: ProviderSettings): string {
	const cometmind = cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind));
	return JSON.stringify({
		providers: settings.providers.map(cloneProvider),
		activeProviderId: settings.activeProviderId,
		defaultModelId: settings.defaultModelId ?? '',
		defaultProviderId: settings.defaultProviderId ?? '',
		titleProviderId: cometmind.titleProviderId,
		titleModelId: cometmind.titleModelId,
		extractionProviderId: cometmind.memory.extractionProviderId,
		extractionModel: cometmind.memory.extractionModel
	});
}

function agentSectionSnapshot(settings: ProviderSettings): string {
	const cometmind = cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind));
	const { enabled: _discordEnabled, ...discordPending } = cometmind.gateway.discord;
	return JSON.stringify({
		systemPromptPath: cometmind.systemPromptPath,
		maxTokens: cometmind.maxTokens,
		logLevel: cometmind.logLevel,
		acp: cometmind.acp,
		skills: cometmind.skills,
		mcp: cometmind.mcp,
		gateway: { discord: discordPending }
	});
}

function appearanceSectionSnapshot(settings: ProviderSettings): string {
	return JSON.stringify({
		appearance: {
			heroComposer: { ...settings.appearance.heroComposer },
			caretTrail: { ...settings.appearance.caretTrail }
		},
		personaId: settings.app?.personaId ?? 'minako',
		personas: settings.app?.personas?.custom ?? []
	});
}

function appSectionSnapshot(settings: ProviderSettings): string {
	const cometmind = cloneCometMindSettings(normalizeCometMindSettings(settings.cometmind));
	return JSON.stringify({
		storage: cometmind.storage,
		miniWindowInactivityTimeoutMinutes: settings.app?.miniWindowInactivityTimeoutMinutes ?? 30
	});
}

export function sectionPendingDirty(
	section: SettingsSection,
	draft: ProviderSettings,
	persisted: ProviderSettings
): boolean {
	switch (section) {
		case 'models':
			return modelsSectionSnapshot(draft) !== modelsSectionSnapshot(persisted);
		case 'agent':
			return agentSectionSnapshot(draft) !== agentSectionSnapshot(persisted);
		case 'appearance':
			return appearanceSectionSnapshot(draft) !== appearanceSectionSnapshot(persisted);
		case 'app':
			return appSectionSnapshot(draft) !== appSectionSnapshot(persisted);
		case 'shortcuts':
			return false;
		case 'memory':
			return false;
		default:
			return false;
	}
}

export const SECTION_PERSISTENCE_HINTS: Record<
	SettingsSection,
	{ pending?: string; instant?: string; action?: string }
> = {
	models: {
		pending: 'Provider config and model roles',
		action: 'Fetch models and Codex sign-in'
	},
	memory: {
		pending: 'Retrieval, lifecycle, and embedding settings',
		instant: 'Adding or deleting individual memories',
		action: 'Compaction preview and run'
	},
	agent: {
		pending: 'Runtime, MCP, skills, and Discord fields',
		instant: 'Discord gateway Run toggle',
		action: 'Refresh skills, sync symlinks, MCP test'
	},
	appearance: {
		pending: 'Hero composer, caret trail, and persona'
	},
	shortcuts: {
		instant: 'Every shortcut binding'
	},
	app: {
		pending: 'Session storage, retention, and mini window timeout',
		instant: 'Open at login',
		action: 'Export, import, workspace, and updates'
	}
};
