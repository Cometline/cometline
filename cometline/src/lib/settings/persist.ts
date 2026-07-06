import type { MemorySettings } from '$lib/client/cometmind';
import { runStorageRetentionAndSyncSessions } from '$lib/retention/storage-retention-sync';
import { normalizeSettings, validateSettings } from '$lib/settings/schema';
import type { RuntimeApplyAction } from '$lib/settings/settings-save';
import type { ProviderSettings } from '$lib/types';
import { putMemorySettings } from '$lib/client/cometmind';
import { connectionState } from '$lib/stores/runtime.svelte';

export interface PersistSettingsOptions {
	runtimeAction?: RuntimeApplyAction;
	restartCometMind?: boolean;
	memory?: MemorySettings;
}

export interface PersistSettingsResult {
	settings: ProviderSettings;
	memory?: MemorySettings;
	/**
	 * Real outcome of applying the save to the running sidecar, or null when no
	 * runtime action was requested, or undefined outside Electron (browser/dev
	 * fallback path, which only writes to localStorage).
	 */
	reload?: RuntimeReloadOutcome | null;
}

export async function persistSettings(
	draft: ProviderSettings,
	options: PersistSettingsOptions = {}
): Promise<PersistSettingsResult> {
	const runtimeAction = options.runtimeAction ?? (options.restartCometMind === false ? 'none' : 'restart');
	const normalized = validateSettings(normalizeSettings(draft));

	let saved: ProviderSettings;
	let reload: RuntimeReloadOutcome | null | undefined;
	if (window.electronAPI?.saveProviderSettings) {
		const result = await window.electronAPI.saveProviderSettings(normalized, {
			runtimeAction
		});
		saved = result.settings;
		reload = result.reload;
	} else {
		localStorage.setItem('cometline-settings', JSON.stringify(normalized));
		saved = normalized;
	}

	let memory: MemorySettings | undefined;
	if (options.memory) {
		memory = await putMemorySettings(options.memory);
	}

	if (connectionState.status === 'ready') {
		await runStorageRetentionAndSyncSessions();
	}

	if (runtimeAction === 'restart') {
		connectionState.reconnect();
	}

	return { settings: saved, memory, reload };
}
