import type { MemorySettings } from '#lib/client/cometmind.js';
import type { DeleteCustomPersonaResult, SaveCustomPersonaResult } from '#lib/electron-api.js';
import { settingsStore } from '#lib/stores/settings.svelte.js';
import {
	applyMemoryEmbeddingToDraft,
	applyMemorySettingsToDraft,
	cloneSettings,
	providerPayloadFromDraft
} from '#lib/features/settings/settings-draft.js';
import {
	runtimeActionForSettingsSave,
	saveStatusMessage
} from '#lib/features/settings/settings-save.js';
import type { ProviderSettings } from '#lib/types.js';
import type { SettingsSection } from './settings-controller.svelte';
import type { SettingsPanelControllerDeps } from './settings-panel-types';

export function createSettingsPanelPersistence(
	deps: Omit<SettingsPanelControllerDeps, 'getSelectedProvider' | 'getMode'> & {
		replayIntro: () => void;
		clearWorkspacePruneMessage: () => void;
	}
) {
	let cometmindPanelKey = $state(0);
	let memoryPanelKey = $state(0);

	async function saveMemorySection() {
		try {
			const memoryPayload = deps.getMemoryPanel()?.buildSavePayload?.();
			if (!memoryPayload) {
				throw new Error('Memory settings are not available');
			}
			const draft = applyMemorySettingsToDraft(deps.getDraft(), memoryPayload);
			deps.setDraft(draft);
			const payload = providerPayloadFromDraft(draft);
			const runtimeAction = runtimeActionForSettingsSave(settingsStore.settings, payload);
			const {
				settings: saved,
				memory,
				reload
			} = await settingsStore.save(payload, {
				runtimeAction,
				memory: memoryPayload
			});
			if (memory) {
				deps.getMemoryPanel()?.applySavedMemory?.(memory);
			}
			deps.setDraft(cloneSettings(saved));
			deps.settingsController.status = saveStatusMessage(
				'memory',
				runtimeAction,
				false,
				reload
			);
		} catch (error) {
			deps.settingsController.status =
				error instanceof Error ? error.message : 'Failed to save memory settings';
		}
	}

	async function save() {
		deps.settingsController.status = '';
		deps.getCometmindPanel()?.syncFields?.();
		const preservedSection = deps.settingsController.activeSection;
		const preservedProviderId = deps.getSelectedProviderId();
		const preservedModelSearch = deps.getModelSearch();

		if (deps.settingsController.activeSection === 'memory') {
			await saveMemorySection();
			return;
		}

		const draft = deps.getDraft();
		const payload: ProviderSettings = providerPayloadFromDraft(draft);
		const personaIdChanged = settingsStore.settings.app.personaId !== draft.app.personaId;
		const runtimeAction = personaIdChanged
			? 'reload'
			: runtimeActionForSettingsSave(settingsStore.settings, payload);
		const { settings: saved, reload } = await settingsStore.save(payload, { runtimeAction });
		deps.setDraft(cloneSettings(saved));
		cometmindPanelKey += 1;
		deps.settingsController.activeSection = preservedSection;
		deps.setSelectedProviderId(
			saved.providers.some((provider) => provider.id === preservedProviderId)
				? preservedProviderId
				: (saved.providers[0]?.id ?? '')
		);
		deps.setModelSearch(preservedModelSearch);
		deps.settingsController.status = saveStatusMessage(
			preservedSection,
			runtimeAction,
			personaIdChanged,
			reload
		);
		if (personaIdChanged) {
			setTimeout(deps.replayIntro, 600);
		}
	}

	async function persistDraftForRuntime(
		overrides: Partial<Pick<ProviderSettings['cometmind'], 'mcp'>> = {}
	) {
		deps.getCometmindPanel()?.syncFields?.();
		const draft = deps.getDraft();
		const payload: ProviderSettings = providerPayloadFromDraft(draft);
		payload.cometmind = { ...payload.cometmind, ...overrides };
		const runtimeAction = runtimeActionForSettingsSave(settingsStore.settings, payload);
		const { settings: saved, reload } = await settingsStore.save(payload, { runtimeAction });
		deps.setDraft(cloneSettings(saved));
		// Surface a failed-but-fallback-worked reload so a caller like the MCP
		// OAuth pre-save flow doesn't silently proceed to open a browser against
		// config that may not have actually applied. A hard failure (unhealthy)
		// already throws from settingsStore.save/persistSettings.
		if (reload && reload.action === 'restart-fallback') {
			throw new Error(
				`Settings saved, but CometMind had to restart instead of reloading in place: ${reload.error ?? 'unknown error'}`
			);
		}
	}

	async function persistMemoryEmbedding(embedding: MemorySettings['embedding']) {
		const draft = applyMemoryEmbeddingToDraft(deps.getDraft(), embedding);
		deps.setDraft(draft);
		await settingsStore.save(providerPayloadFromDraft(draft), { restartCometMind: false });
	}

	function setPersonaId(personaId: string) {
		const draft = deps.getDraft();
		deps.setDraft({ ...draft, app: { ...draft.app, personaId } });
	}

	async function saveCustomPersona(payload: {
		id?: string;
		name: string;
		soulMarkdown: string;
		avatarDataUrl?: string;
	}): Promise<SaveCustomPersonaResult> {
		if (!window.electronAPI?.saveCustomPersona) {
			return { ok: false, error: 'Custom personas are only available in the desktop app.' };
		}
		const result = await window.electronAPI.saveCustomPersona(payload);
		if (result.ok) {
			const settings = await window.electronAPI.getProviderSettings?.();
			if (settings) {
				settingsStore.apply(settings);
				deps.setDraft(cloneSettings(settings));
			}
			setTimeout(deps.replayIntro, 600);
		}
		return result;
	}

	async function deleteCustomPersona(id: string): Promise<DeleteCustomPersonaResult> {
		if (!window.electronAPI?.deleteCustomPersona) {
			return { ok: false, error: 'Custom personas are only available in the desktop app.' };
		}
		const result = await window.electronAPI.deleteCustomPersona(id);
		if (result.ok) {
			const settings = await window.electronAPI.getProviderSettings?.();
			if (settings) {
				settingsStore.apply(settings);
				deps.setDraft(cloneSettings(settings));
			}
			setTimeout(deps.replayIntro, 600);
		}
		return result;
	}

	function discardSettings() {
		const saved = cloneSettings(settingsStore.settings);
		const selectedProviderId = deps.getSelectedProviderId();
		deps.setDraft(saved);
		deps.setSelectedProviderId(
			saved.providers.some((provider) => provider.id === selectedProviderId)
				? selectedProviderId
				: (saved.providers[0]?.id ?? '')
		);
		deps.setModelSearch('');
		cometmindPanelKey += 1;
		memoryPanelKey += 1;
		deps.clearWorkspacePruneMessage();
		deps.settingsController.status = 'Discarded unsaved changes.';
		deps.closeSettings();
	}

	function selectSection(section: SettingsSection) {
		deps.settingsController.activeSection = section;
		deps.settingsController.status = '';
	}

	return {
		get cometmindPanelKey() {
			return cometmindPanelKey;
		},
		get memoryPanelKey() {
			return memoryPanelKey;
		},
		save,
		persistDraftForRuntime,
		persistMemoryEmbedding,
		setPersonaId,
		saveCustomPersona,
		deleteCustomPersona,
		discardSettings,
		selectSection
	};
}
