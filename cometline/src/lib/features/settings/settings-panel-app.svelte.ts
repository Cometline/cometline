import { pruneWorkspaces } from '#lib/client/cometmind.js';
import type { UpdateState } from '#lib/electron-api.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import type { SettingsPanelControllerDeps } from './settings-panel-types';

export function createSettingsPanelApp(
	deps: Pick<SettingsPanelControllerDeps, 'getDraft' | 'setDraft' | 'getMode'>
) {
	let updateState = $state<UpdateState>({ status: 'idle' });
	let checkingUpdates = $state(false);
	let installingUpdate = $state(false);
	let workspacePruning = $state(false);
	let workspacePruneMessage = $state('');
	let appVersion = $state('');

	const updateStatusText = $derived.by(() => {
		switch (updateState.status) {
			case 'checking':
				return 'Checking for updates…';
			case 'downloading':
				return updateState.percent != null
					? `Downloading update ${updateState.percent}%`
					: 'Downloading update…';
			case 'ready':
				return updateState.version
					? `Update available (v${updateState.version})`
					: 'Update available';
			case 'error':
				return updateState.message ?? 'Update check failed';
			default:
				return 'Cometline is up to date';
		}
	});

	const canCheckUpdates = $derived(
		!checkingUpdates && updateState.status !== 'downloading' && !installingUpdate
	);

	function loadAppInfo(api: NonNullable<typeof window.electronAPI>) {
		void api.getAppVersion?.().then((version) => {
			if (version) appVersion = version;
		});
		void api.getUpdateState?.().then((current) => {
			if (current) updateState = current;
		});
	}

	function subscribeUpdateState(api: NonNullable<typeof window.electronAPI>) {
		return api.onUpdateState?.((next) => {
			updateState = next;
			if (next.status !== 'checking') checkingUpdates = false;
		});
	}

	async function checkForUpdates() {
		const api = window.electronAPI;
		if (!api?.checkForUpdates || !canCheckUpdates) return;
		checkingUpdates = true;
		try {
			const next = await api.checkForUpdates();
			updateState = next;
		} catch (error) {
			updateState = {
				status: 'error',
				message: error instanceof Error ? error.message : 'Update check failed'
			};
		} finally {
			checkingUpdates = false;
		}
	}

	async function installUpdate() {
		const api = window.electronAPI;
		if (!api?.installUpdate || updateState.status !== 'ready' || installingUpdate) return;
		installingUpdate = true;
		try {
			await api.installUpdate();
		} catch (error) {
			console.error('Failed to install update:', error);
			installingUpdate = false;
		}
	}

	async function changeWorkspace() {
		const api = window.electronAPI;
		if (!api?.selectWorkspacePath) return;
		const selected = await api.selectWorkspacePath();
		if (!selected) return;
		shellStore.setDefaultWorkspacePath(selected);
	}

	async function cleanupWorkspaces() {
		if (workspacePruning) return;
		workspacePruning = true;
		workspacePruneMessage = '';
		try {
			const [{ pruned }, storeResult] = await Promise.all([
				pruneWorkspaces(),
				window.electronAPI?.pruneWorkspaceStore?.() ?? {
					removedRecent: 0,
					clearedCurrent: false
				}
			]);
			const parts: string[] = [];
			if (pruned > 0) {
				parts.push(
					`Removed ${pruned} stale workspace registration${pruned === 1 ? '' : 's'} from CometMind`
				);
			}
			if (storeResult.removedRecent > 0) {
				parts.push(
					`Cleared ${storeResult.removedRecent} recent path${storeResult.removedRecent === 1 ? '' : 's'}`
				);
			}
			if (storeResult.clearedCurrent) {
				parts.push('Cleared invalid current workspace path');
			}
			workspacePruneMessage =
				parts.length > 0 ? parts.join('. ') + '.' : 'Nothing to clean up.';
		} catch (error) {
			workspacePruneMessage =
				error instanceof Error ? error.message : 'Failed to clean up workspaces.';
		} finally {
			workspacePruning = false;
		}
	}

	function clearWorkspacePruneMessage() {
		workspacePruneMessage = '';
	}

	function replayIntro() {
		// In a separate Settings window the intro has no AppShell to render into,
		// so ask the main window to play it and reveal itself.
		if (deps.getMode() === 'window' && window.electronAPI?.replayIntroInMainWindow) {
			void window.electronAPI.replayIntroInMainWindow();
			return;
		}
		shellStore.closeSettings();
		shellStore.openIntro();
	}

	function runSetupWizard() {
		if (deps.getMode() === 'window' && window.electronAPI?.runSetupWizardInMainWindow) {
			void window.electronAPI.runSetupWizardInMainWindow();
			return;
		}
		shellStore.closeSettings();
		shellStore.openSetup();
	}

	async function pickGatewayWorkspace() {
		const picked = await window.electronAPI?.selectWorkspacePath?.();
		if (!picked) return;
		const draft = deps.getDraft();
		deps.setDraft({
			...draft,
			cometmind: {
				...draft.cometmind,
				gateway: {
					discord: {
						...draft.cometmind.gateway.discord,
						workspacePath: picked
					}
				}
			}
		});
	}

	return {
		get updateState() {
			return updateState;
		},
		get checkingUpdates() {
			return checkingUpdates;
		},
		get installingUpdate() {
			return installingUpdate;
		},
		get workspacePruning() {
			return workspacePruning;
		},
		get workspacePruneMessage() {
			return workspacePruneMessage;
		},
		get appVersion() {
			return appVersion;
		},
		get updateStatusText() {
			return updateStatusText;
		},
		get canCheckUpdates() {
			return canCheckUpdates;
		},
		loadAppInfo,
		subscribeUpdateState,
		checkForUpdates,
		installUpdate,
		changeWorkspace,
		cleanupWorkspaces,
		clearWorkspacePruneMessage,
		replayIntro,
		runSetupWizard,
		pickGatewayWorkspace
	};
}
