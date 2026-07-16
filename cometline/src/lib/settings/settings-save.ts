import type { ProviderSettings } from '$lib/types';
import type { SettingsSection } from '$lib/components/settings/settings-controller.svelte';

export type RuntimeApplyAction = 'none' | 'reload' | 'restart' | 'gateway';

function gatewayChanged(persisted: ProviderSettings, next: ProviderSettings): boolean {
	return (
		JSON.stringify(persisted.cometmind.gateway ?? null) !==
		JSON.stringify(next.cometmind.gateway ?? null)
	);
}

function processLevelServeChanged(persisted: ProviderSettings, next: ProviderSettings): boolean {
	// Host/port are process-level for the main sidecar; keep restart if present.
	const before = persisted.cometmind as ProviderSettings['cometmind'] & {
		host?: string;
		port?: number;
	};
	const after = next.cometmind as ProviderSettings['cometmind'] & {
		host?: string;
		port?: number;
	};
	return before.host !== after.host || before.port !== after.port;
}

/**
 * Classify how CometMind should apply a settings save.
 *
 * Most cometmind/provider changes hot-reload. Changes under `cometmind.gateway`
 * (any platform) trigger a gateway-only recycle (main serve stays up). Full
 * serve restart is reserved for true process-level bind changes (host/port).
 *
 * When gateway changes together with other runtime fields, return `reload`
 * (Electron still syncs gateways on every save).
 */
export function runtimeActionForSettingsSave(
	persisted: ProviderSettings,
	next: ProviderSettings
): RuntimeApplyAction {
	const providersChanged = JSON.stringify(persisted.providers) !== JSON.stringify(next.providers);
	const cometmindChanged = JSON.stringify(persisted.cometmind) !== JSON.stringify(next.cometmind);
	if (!providersChanged && !cometmindChanged) return 'none';

	if (processLevelServeChanged(persisted, next)) {
		return 'restart';
	}

	const gatewaysChanged = gatewayChanged(persisted, next);
	const withoutGateway = (settings: ProviderSettings) => {
		const { gateway: _gateway, ...rest } = settings.cometmind;
		return rest;
	};
	const otherCometmindEqual =
		JSON.stringify(withoutGateway(persisted)) === JSON.stringify(withoutGateway(next));
	if (gatewaysChanged && otherCometmindEqual && !providersChanged) {
		return 'gateway';
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
		if (runtimeAction === 'restart') return ' CometMind restarted.';
		if (runtimeAction === 'gateway') return ' Gateway restarted.';
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
	if (runtimeAction === 'gateway' || reload.action === 'gateway') {
		return ' Gateway restarted.';
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
