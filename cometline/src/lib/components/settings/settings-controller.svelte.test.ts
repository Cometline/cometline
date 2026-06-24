// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { createSettingsController } from './settings-controller.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';

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
});
