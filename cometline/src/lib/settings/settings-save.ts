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
	if (JSON.stringify(persisted.cometmind.autonomy) !== JSON.stringify(next.cometmind.autonomy)) {
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

/**
 * Builds the runtime-status suffix ("CometMind reloaded.") appended to a save
 * confirmation. When `reload` is provided (Electron path), it reflects what
 * actually happened to the sidecar instead of assuming the requested
 * `runtimeAction` always succeeded silently — a confirmed in-place reload, a
 * fallback restart with a surfaced error, or an unhealthy process are now all
 * distinguishable in the UI (previously every case rendered the same
 * "CometMind reloaded." regardless of outcome).
 */
function runtimeNoteFor(runtimeAction: RuntimeApplyAction, reload?: RuntimeReloadOutcome | null): string {
	if (reload === undefined) {
		// No reload info available (e.g. browser/dev fallback without Electron
		// IPC) — fall back to the pre-#4 behavior of trusting runtimeAction.
		if (runtimeAction === 'restart') return ' CometMind restarted.';
		if (runtimeAction === 'reload') return ' CometMind reloaded.';
		return '';
	}
	if (reload === null) return '';
	if (reload.action === 'restart-fallback') {
		return reload.healthy
			? ` CometMind reload failed and restarted instead (${reload.error ?? 'unknown error'}).`
			: ` CometMind reload failed and the restart did not come back up (${reload.error ?? 'unknown error'}). Check the CometMind log.`;
	}
	if (!reload.healthy) {
		return reload.action === 'restart'
			? ' CometMind restarted but is not responding yet. Check the CometMind log.'
			: ' CometMind reloaded but is not responding yet. Check the CometMind log.';
	}
	return reload.action === 'restart' ? ' CometMind restarted.' : ' CometMind reloaded.';
}

export function saveStatusMessage(
	section: SettingsSection,
	runtimeAction: RuntimeApplyAction,
	personaIdChanged = false,
	reload?: RuntimeReloadOutcome | null
): string {
	const runtimeNote = runtimeNoteFor(runtimeAction, reload);
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
