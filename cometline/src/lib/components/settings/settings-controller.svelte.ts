import type { ProviderSettings } from '$lib/types';
import { settingsStore } from '$lib/stores/settings.svelte';
import {
	sectionPendingDirty,
	settingsPendingDirty,
	type SettingsSection as PendingSettingsSection
} from '$lib/settings/pending-settings';

export type SettingsSection = 'models' | 'memory' | 'agent' | 'appearance' | 'shortcuts' | 'app';

export function createSettingsController(deps: {
	getDraft: () => ProviderSettings;
	getMemoryPanelDirty: () => boolean;
	getMemoryPanelBusy: () => boolean;
}) {
	let activeSection = $state<SettingsSection>('models');
	let status = $state('');

	const draftPendingDirty = $derived(
		settingsPendingDirty(deps.getDraft(), settingsStore.settings)
	);
	const memoryPendingDirty = $derived(deps.getMemoryPanelDirty());
	const hasPendingChanges = $derived(draftPendingDirty || memoryPendingDirty);

	const saveDisabled = $derived(
		settingsStore.isSaving ||
			settingsStore.isFetchingModels ||
			!hasPendingChanges ||
			(activeSection === 'memory' && deps.getMemoryPanelBusy())
	);

	function navSectionDirty(section: PendingSettingsSection): boolean {
		if (section === 'memory') return memoryPendingDirty;
		return sectionPendingDirty(section, deps.getDraft(), settingsStore.settings);
	}

	return {
		get activeSection() {
			return activeSection;
		},
		set activeSection(section: SettingsSection) {
			activeSection = section;
		},
		get status() {
			return status;
		},
		set status(message: string) {
			status = message;
		},
		get hasPendingChanges() {
			return hasPendingChanges;
		},
		get saveDisabled() {
			return saveDisabled;
		},
		navSectionDirty
	};
}
