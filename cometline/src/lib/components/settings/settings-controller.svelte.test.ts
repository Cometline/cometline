// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { createSettingsController } from './settings-controller.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';
import { cloneSettings } from '$lib/settings/settings-draft';

describe('createSettingsController', () => {
	it('disables save when draft matches persisted settings', () => {
		const controller = createSettingsController({
			getDraft: () => settingsStore.settings,
			getMemoryPanelDirty: () => false,
			getMemoryPanelBusy: () => false
		});
		expect(controller.hasPendingChanges).toBe(false);
		expect(controller.saveDisabled).toBe(true);
	});

	it('enables save when draft differs from persisted settings', () => {
		const draft = cloneSettings(settingsStore.settings);
		draft.defaultModelId = draft.defaultModelId ? `${draft.defaultModelId}-changed` : 'changed';
		const controller = createSettingsController({
			getDraft: () => draft,
			getMemoryPanelDirty: () => false,
			getMemoryPanelBusy: () => false
		});
		expect(controller.hasPendingChanges).toBe(true);
		expect(controller.saveDisabled).toBe(false);
	});

	it('disables save on memory tab when memory panel is busy', () => {
		const draft = cloneSettings(settingsStore.settings);
		draft.defaultModelId = draft.defaultModelId ? `${draft.defaultModelId}-changed` : 'changed';
		const controller = createSettingsController({
			getDraft: () => draft,
			getMemoryPanelDirty: () => true,
			getMemoryPanelBusy: () => true
		});
		controller.activeSection = 'memory';
		expect(controller.hasPendingChanges).toBe(true);
		expect(controller.saveDisabled).toBe(true);
	});
});
