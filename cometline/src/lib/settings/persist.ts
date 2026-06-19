import type { MemorySettings } from '$lib/client/cometmind';
import { normalizeSettings, validateSettings } from '$lib/settings/schema';
import type { ProviderSettings } from '$lib/types';
import { CometMindApiError, purgeArchivedMemory, putMemorySettings } from '$lib/client/cometmind';
import { connectionState } from '$lib/stores/runtime.svelte';

export interface PersistSettingsOptions {
	restartCometMind?: boolean;
	memory?: MemorySettings;
}

export async function persistSettings(
	draft: ProviderSettings,
	options: PersistSettingsOptions = {}
): Promise<{ settings: ProviderSettings; memory?: MemorySettings }> {
	const restartCometMind = options.restartCometMind ?? true;
	const normalized = validateSettings(normalizeSettings(draft));

	let saved: ProviderSettings;
	if (window.electronAPI?.saveProviderSettings) {
		saved = await window.electronAPI.saveProviderSettings(normalized, {
			restartCometMind
		});
	} else {
		localStorage.setItem('cometline-settings', JSON.stringify(normalized));
		saved = normalized;
	}

	let memory: MemorySettings | undefined;
	if (options.memory) {
		memory = await putMemorySettings(options.memory);
	}

	const purgeDays = normalized.cometmind.storage.archivedMemoryPurgeDays;
	if (purgeDays > 0) {
		try {
			await purgeArchivedMemory(purgeDays);
		} catch (err) {
			if (!(err instanceof CometMindApiError && err.status === 503)) {
				throw err;
			}
		}
	}

	if (restartCometMind) {
		connectionState.reconnect();
	}

	return { settings: saved, memory };
}
