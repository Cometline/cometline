import { app } from 'electron';
import electronUpdater from 'electron-updater';
import type { RuntimeContext } from './runtime-context.js';

const UPDATE_CHECK_INTERVAL_MS = 4 * 60 * 60 * 1000;

export interface UpdateState {
	status: string;
	version?: string;
	percent?: number;
	message?: string;
	updatedAt?: number;
}

export interface AutoUpdaterDeps {
	context: RuntimeContext;
	beforeInstall(): Promise<void>;
}

export interface AutoUpdaterDomain {
	configure(): void;
	stop(): void;
	getState(): UpdateState;
	check(): Promise<UpdateState>;
	install(): Promise<boolean>;
}

export function createAutoUpdater(deps: AutoUpdaterDeps): AutoUpdaterDomain {
	let updateState: UpdateState = { status: 'idle' };
	let updateCheckTimer: NodeJS.Timeout | null = null;

	function setState(next: UpdateState) {
		updateState = { ...next, updatedAt: Date.now() };
		for (const window of deps.context.getWindows()) {
			if (window && !window.isDestroyed()) window.webContents.send('cometline:update-state', updateState);
		}
	}

	function getAutoUpdater() {
		return electronUpdater.autoUpdater;
	}

	function checkForUpdates() {
		return getAutoUpdater().checkForUpdates().catch((error) => {
			console.error('Auto-update check failed:', error);
		});
	}

	return {
		configure() {
			if (!app.isPackaged) return;
			getAutoUpdater().autoDownload = true;
			getAutoUpdater().autoInstallOnAppQuit = false;
			getAutoUpdater().logger = {
				info: (message) => console.log(`[auto-updater] ${message}`),
				warn: (message) => console.warn(`[auto-updater] ${message}`),
				error: (message) => console.error(`[auto-updater] ${message}`),
				debug: (message) => console.debug(`[auto-updater] ${message}`)
			};
			getAutoUpdater().on('checking-for-update', () => setState({ status: 'checking' }));
			getAutoUpdater().on('update-available', (info) => setState({ status: 'downloading', version: info?.version, percent: 0 }));
			getAutoUpdater().on('update-not-available', (info) => setState({ status: 'idle', version: info?.version }));
			getAutoUpdater().on('download-progress', (progress) => setState({ status: 'downloading', percent: Math.round(progress?.percent ?? 0) }));
			getAutoUpdater().on('update-downloaded', (info) => setState({ status: 'ready', version: info?.version }));
			getAutoUpdater().on('error', (error) => {
				console.error('Auto-update error:', error);
				setState({
					status: 'error',
					message: error instanceof Error ? error.message : String(error)
				});
			});
			void checkForUpdates();
			updateCheckTimer = setInterval(() => void checkForUpdates(), UPDATE_CHECK_INTERVAL_MS);
		},
		stop() {
			if (!updateCheckTimer) return;
			clearInterval(updateCheckTimer);
			updateCheckTimer = null;
		},
		getState: () => updateState,
		async check() {
			if (!app.isPackaged) return { status: 'idle' };
			try {
				await getAutoUpdater().checkForUpdates();
			} catch (error) {
				console.error('Manual update check failed:', error);
				setState({
					status: 'error',
					message: error instanceof Error ? error.message : String(error)
				});
			}
			return updateState;
		},
		async install() {
			if (updateState.status !== 'ready') return false;
			await deps.beforeInstall();
			setImmediate(() => getAutoUpdater().quitAndInstall(true, true));
			return true;
		}
	};
}
