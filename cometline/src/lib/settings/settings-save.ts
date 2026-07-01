import type { ProviderSettings } from '$lib/types';
import type { SettingsSection } from '$lib/components/settings/settings-controller.svelte';

export type RuntimeApplyAction = 'none' | 'reload' | 'restart';

function providerChangedIds(persisted: ProviderSettings, next: ProviderSettings): Set<string> {
	const changed = new Set<string>();
	const before = new Map(persisted.providers.map((provider) => [provider.id, JSON.stringify(provider)]));
	const after = new Map(next.providers.map((provider) => [provider.id, JSON.stringify(provider)]));
	for (const id of new Set([...before.keys(), ...after.keys()])) {
		if (before.get(id) !== after.get(id)) changed.add(id);
	}
	return changed;
}

function memoryProviderIds(settings: ProviderSettings): Set<string> {
	const ids = new Set<string>();
	const extraction = settings.cometmind.memory.extractionProviderId;
	const embedding = settings.cometmind.memory.embedding.providerId;
	if (extraction) ids.add(extraction);
	if (embedding) ids.add(embedding);
	return ids;
}

export function runtimeActionForSettingsSave(
	persisted: ProviderSettings,
	next: ProviderSettings
): RuntimeApplyAction {
	const providersChanged = JSON.stringify(persisted.providers) !== JSON.stringify(next.providers);
	const cometmindChanged = JSON.stringify(persisted.cometmind) !== JSON.stringify(next.cometmind);
	if (!providersChanged && !cometmindChanged) return 'none';

	if (JSON.stringify(persisted.cometmind.memory) !== JSON.stringify(next.cometmind.memory)) {
		return 'restart';
	}
	if (
		persisted.cometmind.storage.cleanupIntervalMinutes !==
			next.cometmind.storage.cleanupIntervalMinutes
	) {
		return 'restart';
	}
	if (
		persisted.cometmind.jobs.reconcileIntervalSeconds !==
			next.cometmind.jobs.reconcileIntervalSeconds
	) {
		return 'restart';
	}

	const changedProviderIds = providerChangedIds(persisted, next);
	if (changedProviderIds.size > 0) {
		const memoryProviders = new Set([
			...memoryProviderIds(persisted),
			...memoryProviderIds(next)
		]);
		for (const id of changedProviderIds) {
			if (memoryProviders.has(id)) return 'restart';
		}
	}

	return 'reload';
}

export function saveStatusMessage(
	section: SettingsSection,
	runtimeAction: RuntimeApplyAction,
	personaIdChanged = false
): string {
	const runtimeNote =
		runtimeAction === 'restart'
			? ' CometMind restarted.'
			: runtimeAction === 'reload'
				? ' CometMind reloaded.'
				: '';
	switch (section) {
		case 'models':
			return runtimeAction === 'none' ? 'Changes saved.' : `Changes saved.${runtimeNote}`;
		case 'agent':
			return `Changes saved.${runtimeNote}`;
		case 'appearance':
			return personaIdChanged || runtimeAction !== 'none'
				? `Changes saved.${runtimeNote}`
				: 'Changes saved.';
		case 'app':
			return 'Changes saved.';
		case 'memory':
			return `Changes saved.${runtimeNote}`;
		default:
			return 'Changes saved.';
	}
}
