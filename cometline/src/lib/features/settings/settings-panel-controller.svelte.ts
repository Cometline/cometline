import { createSettingsPanelApp } from './settings-panel-app.svelte';
import { createSettingsPanelPersistence } from './settings-panel-persistence.svelte';
import { createSettingsPanelPreferences } from './settings-panel-preferences.svelte';
import { createSettingsPanelProviders } from './settings-panel-providers.svelte';
import type { SettingsPanelControllerDeps } from './settings-panel-types';

export function createSettingsPanelController(deps: SettingsPanelControllerDeps) {
	const app = createSettingsPanelApp(deps);
	const preferences = createSettingsPanelPreferences(deps);
	const providers = createSettingsPanelProviders(deps);
	const persistence = createSettingsPanelPersistence({
		...deps,
		replayIntro: app.replayIntro,
		clearWorkspacePruneMessage: app.clearWorkspacePruneMessage
	});

	function initElectron() {
		const api = window.electronAPI;
		if (!api) return () => {};

		app.loadAppInfo(api);
		preferences.loadElectronPreferences(api);
		const unsubscribe = app.subscribeUpdateState(api);
		void providers.refreshCodexAuthStatus();
		void providers.refreshXaiAuthStatus();
		return () => unsubscribe?.();
	}

	return {
		get codexAuthStatus() {
			return providers.codexAuthStatus;
		},
		get checkingCodexAuth() {
			return providers.checkingCodexAuth;
		},
		get startingCodexLogin() {
			return providers.startingCodexLogin;
		},
		get xaiAuthStatus() {
			return providers.xaiAuthStatus;
		},
		get checkingXaiAuth() {
			return providers.checkingXaiAuth;
		},
		get startingXaiLogin() {
			return providers.startingXaiLogin;
		},
		get updateState() {
			return app.updateState;
		},
		get checkingUpdates() {
			return app.checkingUpdates;
		},
		get installingUpdate() {
			return app.installingUpdate;
		},
		get workspacePruning() {
			return app.workspacePruning;
		},
		get workspacePruneMessage() {
			return app.workspacePruneMessage;
		},
		get appVersion() {
			return app.appVersion;
		},
		get cometmindPanelKey() {
			return persistence.cometmindPanelKey;
		},
		get memoryPanelKey() {
			return persistence.memoryPanelKey;
		},
		get updateStatusText() {
			return app.updateStatusText;
		},
		get canCheckUpdates() {
			return app.canCheckUpdates;
		},
		get screenCaptureStatus() {
			return preferences.screenCaptureStatus;
		},
		initElectron,
		checkForUpdates: app.checkForUpdates,
		installUpdate: app.installUpdate,
		changeWorkspace: app.changeWorkspace,
		cleanupWorkspaces: app.cleanupWorkspaces,
		replayIntro: app.replayIntro,
		runSetupWizard: app.runSetupWizard,
		updateProvider: providers.updateProvider,
		updateSelected: providers.updateSelected,
		updateShortcut: preferences.updateShortcut,
		setOpenAtLogin: preferences.setOpenAtLogin,
		setScreenCapturePreferred: preferences.setScreenCapturePreferred,
		openScreenCaptureSettings: preferences.openScreenCaptureSettings,
		setConfirmCloseOnCmdW: preferences.setConfirmCloseOnCmdW,
		setConfirmBeforeDeletingChats: preferences.setConfirmBeforeDeletingChats,
		setConfirmBeforeDeletingMedia: preferences.setConfirmBeforeDeletingMedia,
		setFileSearchSource: preferences.setFileSearchSource,
		save: persistence.save,
		persistDraftForRuntime: persistence.persistDraftForRuntime,
		setSelectedMethod: providers.setSelectedMethod,
		toggleProvider: providers.toggleProvider,
		toggleModel: providers.toggleModel,
		fetchModels: providers.fetchModels,
		refreshCodexAuthStatus: providers.refreshCodexAuthStatus,
		startCodexLogin: providers.startCodexLogin,
		refreshXaiAuthStatus: providers.refreshXaiAuthStatus,
		startXaiLogin: providers.startXaiLogin,
		addProvider: providers.addProvider,
		removeProvider: providers.removeProvider,
		pickGatewayWorkspace: app.pickGatewayWorkspace,
		persistMemoryEmbedding: persistence.persistMemoryEmbedding,
		setPersonaId: persistence.setPersonaId,
		saveCustomPersona: persistence.saveCustomPersona,
		deleteCustomPersona: persistence.deleteCustomPersona,
		discardSettings: persistence.discardSettings,
		selectSection: persistence.selectSection
	};
}
