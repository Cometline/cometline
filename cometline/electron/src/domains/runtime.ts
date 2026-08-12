import {
	app,
	BrowserWindow,
	dialog,
	nativeImage,
	net,
	Notification as ElectronNotification,
	protocol,
	screen as electronScreen,
	shell
} from 'electron';
import type { OpenDialogOptions } from 'electron';
import crypto from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import { fileURLToPath } from 'node:url';
import os from 'node:os';
import path from 'node:path';

import { defaultSettings } from '../../../src/lib/settings/schema.js';
import type { ProviderSettings } from '../../../src/lib/types.js';
import { createOllamaService } from '../services/ollama.js';
import { createApplicationMenuTray } from './app-menu-tray.js';
import { APP_SCHEME, registerAppProtocol } from './app-protocol.js';
import { createAutoUpdater } from './auto-updater.js';
import { createBrowserSearchBridge } from './browser-search.js';
import { createCometMindLifecycle } from './cometmind-lifecycle.js';
import { createPersonas } from './personas.js';
import {
	PDF_PREVIEW_SCHEME,
	createPdfPreviewRegistry,
	registerPdfPreviewProtocol
} from './pdf-preview.js';
import { createProviderAuth } from './provider-auth.js';
import { createRouteSignals } from './route-signals.js';
import { registerRuntimeIpcHandlers } from './runtime-ipc.js';
import type { ShellWindowContext } from './runtime-context.js';
import { createScreenCaptureBridge } from './screen-capture.js';
import { createSettingsDomain } from './settings.js';
import { createShortcutCoordinator } from './shortcuts.js';
import { createTerminalManager } from './terminal.js';
import { createWindowChrome } from './window-chrome.js';
import { createWindows } from './windows.js';
import { createWorkspaceWatcher } from './workspace-watcher.js';

// Keep production path resolution stable after compiling this source into electron/dist.
const runtimeDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

type Windows = ReturnType<typeof createWindows>;
type CometMindLifecycle = ReturnType<typeof createCometMindLifecycle>;

let initialized = false;

function resolveCometMindBinary() {
	if (process.env.COMETMIND_BINARY_PATH) return process.env.COMETMIND_BINARY_PATH;
	if (app.isPackaged) return path.join(process.resourcesPath, 'cometmind');
	const devCandidate = path.join(runtimeDirectory, '..', '..', 'cometmind', 'dist', 'cometmind');
	if (fs.existsSync(devCandidate)) return devCandidate;
	return path.join(runtimeDirectory, '..', '..', 'cometmind', 'cometmind');
}

function selectBackupFolder() {
	const window = BrowserWindow.getFocusedWindow();
	const options: OpenDialogOptions = {
		properties: ['openDirectory', 'createDirectory'],
		buttonLabel: 'Select backup folder',
		title: 'Choose a folder for CometMind backups'
	};
	const result = window ? dialog.showOpenDialog(window, options) : dialog.showOpenDialog(options);
	return result.then((result) =>
		result.canceled || result.filePaths.length === 0
			? { canceled: true }
			: { canceled: false, path: result.filePaths[0] }
	);
}

/** Explicit Electron composition entrypoint. Must run before app readiness. */
export function initializeRuntime() {
	if (initialized) return;
	initialized = true;

	app.setName('Cometline');
	protocol.registerSchemesAsPrivileged([
		{
			scheme: APP_SCHEME,
			privileges: {
				standard: true,
				secure: true,
				supportFetchAPI: true,
				stream: true,
				codeCache: true
			}
		},
		{
			scheme: PDF_PREVIEW_SCHEME,
			privileges: {
				standard: true,
				secure: true,
				supportFetchAPI: true,
				stream: true
			}
		}
	]);

	const hasSingleInstanceLock = app.requestSingleInstanceLock();
	if (!hasSingleInstanceLock) {
		app.quit();
		return;
	}

	let tray: Electron.Tray | null = null;
	let stoppingForQuit = false;
	let stoppedForQuit = false;
	let relaunchForUpdate = false;
	let shortcutCaptureActive = false;
	let sessionNavigationSuspended = false;
	let workspacePanelOpen = false;
	let inboxOpen = false;
	let windows: Windows | null = null;

	const shellContext: ShellWindowContext = {
		getWindows: () => [
			windows?.getMainWindow() ?? null,
			windows?.getMiniWindow() ?? null,
			windows?.getSettingsWindow() ?? null
		],
		shouldSuppressSidecarRespawn: () => stoppingForQuit || stoppedForQuit || relaunchForUpdate,
		getMainWindow: () => windows?.getMainWindow() ?? null,
		getMiniWindow: () => windows?.getMiniWindow() ?? null,
		getSettingsWindow: () => windows?.getSettingsWindow() ?? null,
		getTray: () => tray,
		setTray: (nextTray) => {
			tray = nextTray;
		},
		getShortcutCaptureActive: () => shortcutCaptureActive,
		setShortcutCaptureActive: (active) => {
			shortcutCaptureActive = active;
		},
		getSessionNavigationSuspended: () => sessionNavigationSuspended,
		setSessionNavigationSuspended: (suspended) => {
			sessionNavigationSuspended = suspended;
		},
		getWorkspacePanelOpen: () => workspacePanelOpen,
		setWorkspacePanelOpen: (open) => {
			workspacePanelOpen = open;
		},
		getInboxOpen: () => inboxOpen,
		setInboxOpen: (open) => {
			inboxOpen = open;
		}
	};

	const ollama = createOllamaService({
		sendProgress: (payload: object) => {
			for (const window of BrowserWindow.getAllWindows()) {
				if (!window.isDestroyed())
					window.webContents.send('cometline:ollama-pull-progress', payload);
			}
		}
	});
	const browserSearch = createBrowserSearchBridge();
	const screenCapture = createScreenCaptureBridge({
		isPreferred: () =>
			Boolean(settingsDomain.readProviderSettings().app?.screenCapturePreferred)
	});
	const terminals = createTerminalManager(() => windows?.getMainWindow() ?? null);
	const workspaceWatcher = createWorkspaceWatcher({
		fs,
		path,
		onChange: (change) => {
			for (const window of BrowserWindow.getAllWindows()) {
				if (!window.isDestroyed())
					window.webContents.send('cometline:workspace-changed', change);
			}
		},
		setTimeout,
		clearTimeout
	});

	function broadcastProviderSettingsChanged(settings: ProviderSettings) {
		for (const window of BrowserWindow.getAllWindows()) {
			if (!window.isDestroyed()) {
				window.webContents.send('cometline:provider-settings-changed', settings);
			}
		}
	}

	function broadcastPersonaAvatarChanged(personaId: string) {
		if (!personaId) return;
		for (const window of BrowserWindow.getAllWindows()) {
			if (!window.isDestroyed())
				window.webContents.send('cometline:persona-avatar-changed', personaId);
		}
	}

	const personas = createPersonas({
		fs,
		path,
		homedir: os.homedir,
		environment: process.env,
		platform: process.platform,
		app,
		resourcesPath: process.resourcesPath,
		nativeImage,
		runtimeDirectory,
		getMainWindow: () => windows?.getMainWindow() ?? null,
		getMiniWindow: () => windows?.getMiniWindow() ?? null,
		getTray: () => tray,
		getSettings: () => settingsDomain.readProviderSettings(),
		writeSettings: (settings) => settingsDomain.writeProviderSettings(settings),
		reloadCometMind: () => cometMind.reload(),
		broadcastProviderSettingsChanged,
		broadcastPersonaAvatarChanged
	});
	const settingsDomain = createSettingsDomain({
		fs,
		path,
		homedir: os.homedir,
		environment: process.env,
		processId: process.pid,
		now: Date.now,
		readSavedPersonaId: personas.readSavedPersonaId,
		resolveNextPersonaId: personas.resolveNextPersonaId,
		resolveSystemPromptPath: personas.resolveSystemPromptPath,
		getFocusedWindow: () => BrowserWindow.getFocusedWindow(),
		showOpenDialog: (window, options) =>
			window instanceof BrowserWindow
				? dialog.showOpenDialog(window, options)
				: dialog.showOpenDialog(options)
	});

	function providerEnv(): NodeJS.ProcessEnv {
		const settings = settingsDomain.readProviderSettings();
		const runtimeProviders = settings.providers.filter(
			(provider) => provider.enabled && provider.enabledModels.length > 0
		);
		const defaultId = String(settings.defaultProviderId || '').trim();
		const active =
			runtimeProviders.find((provider) => provider.id === defaultId) ??
			runtimeProviders[0] ??
			settings.providers[0];
		const model =
			(active &&
			settings.defaultModelId &&
			active.enabledModels?.includes(settings.defaultModelId)
				? settings.defaultModelId
				: null) ||
			active?.enabledModels[0] ||
			active?.selectedModel ||
			active?.models[0] ||
			'';
		const env: NodeJS.ProcessEnv = {
			...process.env,
			COMETMIND_PROVIDER: active?.id ?? '',
			COMETMIND_MODEL: model,
			COMETMIND_MAX_TOKENS: String(settings.cometmind?.maxTokens ?? 2048),
			COMETMIND_LOG_LEVEL: settings.cometmind?.logLevel ?? 'error'
		};
		if (active?.baseURL) env.COMETMIND_BASE_URL = active.baseURL;
		if (active?.apiKey) env.COMETMIND_API_KEY = active.apiKey;
		return { ...env, ...browserSearch.getEnvironment(), ...screenCapture.getEnvironment() };
	}

	function getLogsDir() {
		const directory = path.join(os.homedir(), '.cometmind', 'logs');
		if (!fs.existsSync(directory)) fs.mkdirSync(directory, { recursive: true });
		return directory;
	}

	function migrateLegacyLogPaths() {
		const root = path.join(os.homedir(), '.cometmind');
		const logsDir = getLogsDir();
		for (const name of [
			'cometline.log',
			'cometline.log.1',
			'cometline-gateway.log',
			'cometline-gateway.log.1'
		]) {
			const from = path.join(root, name);
			const to = path.join(logsDir, name);
			if (!fs.existsSync(from) || fs.existsSync(to)) continue;
			try {
				fs.renameSync(from, to);
			} catch (error) {
				console.warn(`Failed to migrate log ${name}:`, error);
			}
		}
	}

	function getLogPath() {
		migrateLegacyLogPaths();
		return path.join(getLogsDir(), 'cometline.log');
	}

	const cometMind: CometMindLifecycle = createCometMindLifecycle({
		context: shellContext,
		resolveBinary: resolveCometMindBinary,
		providerEnv,
		getLogPath,
		getGatewayLogPath: () => getLogPath().replace(/\.log$/, '-gateway.log')
	});
	const providerAuth = createProviderAuth({
		fs,
		path,
		http,
		crypto,
		fetch,
		platform: { homedir: os.homedir, environment: process.env },
		window: {
			openExternal: shell.openExternal,
			showMessageBox: (options) => dialog.showMessageBox(options)
		},
		ollama
	});
	const routeSignals = createRouteSignals(shellContext);
	const windowChrome = createWindowChrome(shellContext);
	const applicationMenuTray = createApplicationMenuTray({
		context: shellContext,
		readShortcuts: () =>
			settingsDomain.readProviderSettings().shortcuts ?? defaultSettings().shortcuts,
		getPersonaId: personas.getPersonaId,
		builtinPersonaToIconVariant: personas.builtinPersonaToIconVariant,
		resolveTrayResourcePath: personas.resolveTrayResourcePath,
		resolveTrayIcon: personas.resolveTrayIcon,
		pathExists: fs.existsSync,
		showMainWindow: () => windows?.showMainWindow(),
		showSettingsWindow: async () => windows?.showSettingsWindow(),
		requestReload: () => routeSignals.requestReload()
	});
	const shortcuts = createShortcutCoordinator({
		context: shellContext,
		getShortcuts: () =>
			settingsDomain.readProviderSettings().shortcuts ?? defaultSettings().shortcuts,
		routeSignals,
		showSettingsWindow: async () => windows?.showSettingsWindow(),
		toggleMiniWindow: async () => windows?.toggleMiniWindow(),
		hideMainWindow: () => windows?.hideMainWindow(),
		hideMiniWindow: () => windows?.hideMiniWindow(),
		hideSettingsWindow: () => windows?.hideSettingsWindow()
	});
	windows = createWindows({
		app,
		BrowserWindow,
		ensureTray: applicationMenuTray.ensureTray,
		getAppIconImage: personas.getAppIconImage,
		getAllWindows: BrowserWindow.getAllWindows,
		getPersonaId: personas.getPersonaId,
		getTray: () => tray,
		os,
		path,
		platform: process.platform,
		runtimeDirectory,
		screen: electronScreen,
		shell,
		shouldKeepWindowsAlive: () => !stoppingForQuit && !stoppedForQuit,
		shortcuts,
		windowChrome,
		writeMiniWindowState: settingsDomain.writeMiniWindowState
	});
	const pdfPreview = createPdfPreviewRegistry({
		fs,
		path,
		wikiRoot: path.join(os.homedir(), '.cometmind', 'wiki')
	});
	const updater = createAutoUpdater({
		context: shellContext,
		beforeInstall: async () => {
			relaunchForUpdate = true;
			stoppingForQuit = true;
			await cometMind.syncDiscordGateway({
				cometmind: { gateway: { discord: { enabled: false } } }
			});
			await cometMind.stop();
		}
	});

	registerRuntimeIpcHandlers({
		app,
		Notification: ElectronNotification,
		shell,
		workspacePreview: { fs, path },
		pdfPreview,
		selectBackupFolder,
		context: shellContext,
		windows,
		terminals,
		settings: settingsDomain,
		personas,
		providerAuth,
		ollama,
		cometMind,
		updater,
		shortcuts,
		applicationMenuTray,
		windowChrome,
		workspaceWatcher,
		broadcastProviderSettingsChanged
	});

	app.whenReady().then(async () => {
		registerPdfPreviewProtocol({ protocol, net, fs, path, registry: pdfPreview });
		if (app.isPackaged) {
			registerAppProtocol({
				bundleDirectory: path.join(runtimeDirectory, '..', 'build'),
				fs,
				net,
				path,
				protocol
			});
		}
		settingsDomain.pruneWorkspaceStore();
		if (process.platform === 'darwin') applicationMenuTray.ensureTray();
		settingsDomain.writeProviderSettings(settingsDomain.readProviderSettings());
		cometMind.installCliShim();
		const startupSettings = settingsDomain.readProviderSettings();
		personas.applyPersona(startupSettings.app?.personaId, startupSettings);
		windows.applyOpenAtLoginSetting(startupSettings.app?.openAtLogin);
		applicationMenuTray.configureApplicationMenu();
		shortcuts.refreshGlobalShortcuts();
		try {
			await browserSearch.start();
		} catch (error) {
			console.error('Browser search bridge failed to start:', error);
		}
		try {
			await screenCapture.start();
		} catch (error) {
			console.error('Screen capture bridge failed to start:', error);
		}
		cometMind.start();
		const windowReady = windows.createMainWindow();
		cometMind.waitForHealth().then((healthy) => {
			if (!healthy) {
				console.error('CometMind failed to become healthy');
				return;
			}
			if (startupSettings.cometmind?.gateway?.discord?.enabled) {
				void cometMind.syncDiscordGateway(startupSettings);
			}
		});
		await windowReady;
		updater.configure();
		app.on('second-instance', () => windows?.showMainWindow());
		app.on('activate', () => windows?.handleAppActivate());
	});

	app.on('window-all-closed', () => {
		if (process.platform !== 'darwin') app.quit();
	});
	app.on('before-quit', async (event) => {
		if (stoppedForQuit || stoppingForQuit) return;
		if (relaunchForUpdate) {
			updater.stop();
			return;
		}
		event.preventDefault();
		stoppingForQuit = true;
		applicationMenuTray.destroyTray();
		updater.stop();
		await browserSearch.stop();
		await screenCapture.stop();
		terminals.terminateAll();
		workspaceWatcher.close();
		await cometMind.syncDiscordGateway({
			cometmind: { gateway: { discord: { enabled: false } } }
		});
		await cometMind.stop();
		shortcuts.unregisterAll();
		stoppedForQuit = true;
		app.quit();
	});
	process.on('exit', () => {
		terminals.terminateAll();
		workspaceWatcher.close();
		cometMind.terminateForExit();
	});
}
