import type { MemorySettings } from '#lib/client/cometmind.js';
import type { ProviderConfig, ProviderSettings } from '#lib/types.js';
import type { createSettingsController } from './settings-controller.svelte';

export type CodexAuthStatus = {
	authenticated: boolean;
	authPath: string;
	accountID?: string;
	error?: string;
};

export type XaiAuthStatus = {
	authenticated: boolean;
	authPath: string;
	error?: string;
};

export type CometMindPanelRef = {
	syncFields?: () => void;
};

export type MemoryPanelRef = {
	isDirty?: () => boolean;
	isBusy?: () => boolean;
	buildSavePayload?: () => MemorySettings;
	applySavedMemory?: (memory: MemorySettings) => void;
};

export type SettingsPanelMode = 'modal' | 'window';

export type SettingsPanelControllerDeps = {
	getDraft: () => ProviderSettings;
	setDraft: (draft: ProviderSettings) => void;
	getSelectedProviderId: () => string;
	setSelectedProviderId: (id: string) => void;
	getModelSearch: () => string;
	setModelSearch: (search: string) => void;
	getSelectedProvider: () => ProviderConfig | undefined;
	getCometmindPanel: () => CometMindPanelRef | undefined;
	getMemoryPanel: () => MemoryPanelRef | undefined;
	closeSettings: () => void;
	/**
	 * How the panel is presented. In `'window'` mode Settings runs in its own
	 * Electron window whose route is not wrapped in AppShell, so the intro and
	 * setup wizard (which only mount inside AppShell in the main window) must be
	 * triggered there via IPC instead of the local shell store.
	 */
	getMode: () => SettingsPanelMode;
	settingsController: ReturnType<typeof createSettingsController>;
};
