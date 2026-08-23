import type {
	App,
	BrowserWindow,
	BrowserWindowConstructorOptions,
	LoginItemSettingsOptions,
	NativeImage,
	Screen,
	Settings,
	Shell,
	Tray,
	WebContents
} from 'electron';
import type os from 'node:os';
import type path from 'node:path';

import { APP_ORIGIN } from './app-protocol.js';
import {
	mainWindowMinWidthForWorkArea,
	miniWindowOriginForWorkArea,
	miniWindowSizeForWorkArea
} from './window-bounds.js';
import { isExternallyOpenableUrl } from './workspace-preview.js';

const MACOS_LOGIN_ITEMS_SETTINGS_URL =
	'x-apple.systempreferences:com.apple.LoginItems-Settings.extension';
const MIN_WINDOW_HEIGHT = 620;
const MINI_WINDOW_SCREEN_MARGIN = 18;
const SETTINGS_WINDOW_HEIGHT = 760;
const SETTINGS_WINDOW_MIN_WIDTH = 900;
const SETTINGS_WINDOW_MIN_HEIGHT = 640;
const SETTINGS_WINDOW_MAX_WIDTH = 1280;
const SETTINGS_WINDOW_MAX_HEIGHT = 920;
const SETTINGS_WINDOW_WIDTH = 1040;
const AUXILIARY_WINDOW_ACTIVATE_SUPPRESS_MS = 1000;

type BrowserWindowFactory = new (options: BrowserWindowConstructorOptions) => BrowserWindow;

interface Shortcuts {
	attachMainWindowShortcuts(webContents: WebContents): void;
	attachMiniWindowShortcuts(webContents: WebContents): void;
	attachSettingsWindowShortcuts(webContents: WebContents): void;
	attachWebviewPanelShortcuts(webContents: WebContents): void;
}

interface WindowChrome {
	clearWindowButtonAnimation(): void;
	sendFullScreenState(): void;
	setWindowButtonPosition(position: { x: number; y: number }): void;
}

export interface LoginItemState {
	openAtLogin: boolean;
	status: string;
}

export interface ApplyOpenAtLoginResult extends LoginItemState {
	needsApproval?: boolean;
	openedSettings?: boolean;
	isDev?: boolean;
	message?: string;
}

interface WindowsDependencies {
	app: Pick<App, 'dock' | 'getLoginItemSettings' | 'isPackaged' | 'setLoginItemSettings'>;
	BrowserWindow: BrowserWindowFactory;
	ensureTray(): boolean;
	getAppIconImage(): NativeImage | null;
	getAllWindows(): BrowserWindow[];
	getTray(): Tray | null;
	os: Pick<typeof os, 'release'>;
	path: Pick<typeof path, 'join'>;
	platform: NodeJS.Platform;
	runtimeDirectory: string;
	screen: Pick<Screen, 'getCursorScreenPoint' | 'getDisplayNearestPoint'>;
	shell: Pick<Shell, 'openExternal'>;
	shouldKeepWindowsAlive(): boolean;
	shortcuts: Shortcuts;
	windowChrome: WindowChrome;
	touchMiniWindowActivity(): void;
}

function windowCanShow(window: BrowserWindow | null): window is BrowserWindow {
	return Boolean(window && !window.isDestroyed());
}

/** Owns BrowserWindow creation, visibility, and platform-specific window lifecycle. */
export function createWindows(dependencies: WindowsDependencies) {
	const {
		app,
		BrowserWindow,
		ensureTray,
		getAppIconImage,
		getAllWindows,
		getTray,
		os,
		path,
		platform,
		runtimeDirectory,
		screen,
		shell,
		shouldKeepWindowsAlive,
		shortcuts,
		windowChrome,
		touchMiniWindowActivity
	} = dependencies;
	let mainWindow: BrowserWindow | null = null;
	let miniWindow: BrowserWindow | null = null;
	let miniWindowReady: Promise<BrowserWindow> | null = null;
	let settingsWindow: BrowserWindow | null = null;
	let ignoreActivateUntil = 0;
	let miniNeedsActivation = true;

	function loadAppRoute(window: BrowserWindow, route = '/') {
		const requestedRoute = String(route || '/');
		const cleanRoute = requestedRoute.startsWith('/') ? requestedRoute : `/${requestedRoute}`;
		return window.loadURL(
			app.isPackaged ? `${APP_ORIGIN}${cleanRoute}` : `http://127.0.0.1:5173${cleanRoute}`
		);
	}

	function attachExternalNavigationGuards(window: BrowserWindow) {
		window.webContents.on('will-attach-webview', (_event, webPreferences) => {
			webPreferences.devTools = !app.isPackaged;
		});
		window.webContents.setWindowOpenHandler(({ url }) => {
			if (isExternallyOpenableUrl(url)) void shell.openExternal(url);
			return { action: 'deny' };
		});
		window.webContents.on('will-navigate', (event, url) => {
			const allowed =
				url.startsWith(`${APP_ORIGIN}/`) || url.startsWith('http://127.0.0.1:5173');
			if (!allowed) {
				event.preventDefault();
				if (isExternallyOpenableUrl(url)) void shell.openExternal(url);
			}
		});
	}

	function displayAtCursor() {
		const cursorPoint = screen.getCursorScreenPoint();
		return screen.getDisplayNearestPoint(cursorPoint);
	}

	function resolveMainWindowMinWidth() {
		return mainWindowMinWidthForWorkArea(displayAtCursor().workArea.width);
	}

	function layoutMiniWindowOnCursorDisplay() {
		const window = miniWindow;
		if (!windowCanShow(window)) return;
		const display = displayAtCursor();
		const size = miniWindowSizeForWorkArea(
			display.workArea.width,
			display.workArea.height,
			MINI_WINDOW_SCREEN_MARGIN
		);
		const origin = miniWindowOriginForWorkArea(
			display.workArea,
			size,
			MINI_WINDOW_SCREEN_MARGIN
		);
		const next = { ...origin, ...size };
		const current = window.getBounds();
		if (
			current.x === next.x &&
			current.y === next.y &&
			current.width === next.width &&
			current.height === next.height
		) {
			return;
		}
		window.setBounds(next, false);
	}

	function suppressSpuriousActivate() {
		ignoreActivateUntil = Date.now() + AUXILIARY_WINDOW_ACTIVATE_SUPPRESS_MS;
	}

	function applyMiniWindowPresentation(window = miniWindow) {
		if (!windowCanShow(window) || platform !== 'darwin') return;
		window.setVisibleOnAllWorkspaces(true, {
			visibleOnFullScreen: true,
			skipTransformProcessType: true
		});
		if (typeof window.setFullScreenable === 'function') window.setFullScreenable(false);
		// Keep the panel above normal windows, but stay below macOS IME candidate
		// popups. Higher levels like 'pop-up-menu' / 'screen-saver' cover the
		// system candidate window (electron#29459).
		window.setAlwaysOnTop(true, 'floating');
		if (typeof window.setWindowButtonVisibility === 'function') {
			window.setWindowButtonVisibility(false);
		}
	}

	function hideMainWindow() {
		const window = mainWindow;
		if (!windowCanShow(window)) return;
		if (window.isFullScreen()) {
			window.once('leave-full-screen', () => {
				window.hide();
				ensureTray();
			});
			window.setFullScreen(false);
			return;
		}
		window.hide();
		if (!ensureTray()) {
			console.warn('[tray] Failed to create menu bar icon after hide');
		} else {
			getTray()?.setToolTip('Cometline (hidden)');
		}
	}

	function hideMiniWindow() {
		const window = miniWindow;
		if (!windowCanShow(window)) return;
		touchMiniWindowActivity();
		suppressSpuriousActivate();
		window.hide();
	}

	function hideSettingsWindow() {
		const window = settingsWindow;
		if (!windowCanShow(window)) return;
		suppressSpuriousActivate();
		window.hide();
	}

	async function createMainWindow() {
		const appIcon = getAppIconImage();
		const window = new BrowserWindow({
			width: 1200,
			height: 800,
			minWidth: resolveMainWindowMinWidth(),
			minHeight: MIN_WINDOW_HEIGHT,
			titleBarStyle: 'hidden',
			...(platform === 'darwin'
				? {
						backgroundColor: '#00000000',
						transparent: true,
						vibrancy: 'sidebar' as const,
						visualEffectState: 'active' as const
					}
				: {}),
			...(appIcon ? { icon: appIcon } : {}),
			show: false,
			webPreferences: {
				preload: path.join(runtimeDirectory, 'dist', 'preload.cjs'),
				contextIsolation: true,
				nodeIntegration: false,
				sandbox: true,
				allowRunningInsecureContent: false,
				webviewTag: true,
				devTools: !app.isPackaged
			}
		});
		mainWindow = window;
		windowChrome.setWindowButtonPosition({ x: 16, y: 17 });
		if (platform === 'darwin' && appIcon) app.dock?.setIcon(appIcon);

		attachExternalNavigationGuards(window);
		shortcuts.attachMainWindowShortcuts(window.webContents);
		window.webContents.on('did-attach-webview', (_event, webContents) => {
			shortcuts.attachWebviewPanelShortcuts(webContents);
		});
		await loadAppRoute(window);
		window.once('ready-to-show', () => {
			window.show();
			windowChrome.setWindowButtonPosition({ x: 16, y: 17 });
			windowChrome.sendFullScreenState();
		});
		window.on('enter-full-screen', windowChrome.sendFullScreenState);
		window.on('leave-full-screen', windowChrome.sendFullScreenState);
		window.on('close', (event) => {
			if (platform === 'darwin' && shouldKeepWindowsAlive()) {
				event.preventDefault();
				hideMainWindow();
			}
		});
		window.on('closed', () => {
			windowChrome.clearWindowButtonAnimation();
			if (mainWindow === window) mainWindow = null;
		});
	}

	async function createMiniWindow() {
		const appIcon = getAppIconImage();
		const display = displayAtCursor();
		const miniSize = miniWindowSizeForWorkArea(
			display.workArea.width,
			display.workArea.height,
			MINI_WINDOW_SCREEN_MARGIN
		);
		const window = new BrowserWindow({
			width: miniSize.width,
			height: miniSize.height,
			resizable: false,
			titleBarStyle: platform === 'darwin' ? 'hidden' : 'default',
			...(platform === 'darwin'
				? {
						type: 'panel' as const,
						fullscreenable: false,
						backgroundColor: '#111111',
						trafficLightPosition: { x: 14, y: 14 }
					}
				: {}),
			...(appIcon ? { icon: appIcon } : {}),
			show: false,
			// Hidden prewarm still paints so the first shortcut can reveal a ready view.
			paintWhenInitiallyHidden: true,
			webPreferences: {
				preload: path.join(runtimeDirectory, 'dist', 'preload.cjs'),
				contextIsolation: true,
				nodeIntegration: false,
				sandbox: true,
				allowRunningInsecureContent: false,
				webviewTag: true,
				// Keep the hidden prewarmed renderer warm so shortcut toggles
				// do not pay Chromium's background-throttle wake-up hitch.
				backgroundThrottling: false,
				devTools: !app.isPackaged
			}
		});
		miniWindow = window;
		miniNeedsActivation = true;
		applyMiniWindowPresentation(window);
		attachExternalNavigationGuards(window);
		shortcuts.attachMiniWindowShortcuts(window.webContents);
		layoutMiniWindowOnCursorDisplay();
		window.on('close', (event) => {
			if (shouldKeepWindowsAlive()) {
				event.preventDefault();
				hideMiniWindow();
			}
		});
		window.on('closed', () => {
			if (miniWindow === window) {
				miniWindow = null;
				miniNeedsActivation = true;
			}
		});
		try {
			await loadAppRoute(window, '/mini?prewarm=1');
			return window;
		} catch (error) {
			if (miniWindow === window) miniWindow = null;
			window.destroy();
			throw error;
		}
	}

	async function prepareMiniWindow() {
		if (miniWindowReady) return miniWindowReady;
		if (windowCanShow(miniWindow)) return miniWindow;
		miniWindowReady = createMiniWindow().finally(() => {
			miniWindowReady = null;
		});
		return miniWindowReady;
	}

	async function createSettingsWindow() {
		const appIcon = getAppIconImage();
		const window = new BrowserWindow({
			width: SETTINGS_WINDOW_WIDTH,
			height: SETTINGS_WINDOW_HEIGHT,
			minWidth: SETTINGS_WINDOW_MIN_WIDTH,
			minHeight: SETTINGS_WINDOW_MIN_HEIGHT,
			maxWidth: SETTINGS_WINDOW_MAX_WIDTH,
			maxHeight: SETTINGS_WINDOW_MAX_HEIGHT,
			title: 'Settings',
			titleBarStyle: platform === 'darwin' ? 'hiddenInset' : 'default',
			...(appIcon ? { icon: appIcon } : {}),
			show: false,
			backgroundColor: '#f5f7fb',
			webPreferences: {
				preload: path.join(runtimeDirectory, 'dist', 'preload.cjs'),
				contextIsolation: true,
				nodeIntegration: false,
				sandbox: true,
				allowRunningInsecureContent: false,
				webviewTag: false,
				devTools: !app.isPackaged
			}
		});
		settingsWindow = window;
		attachExternalNavigationGuards(window);
		shortcuts.attachSettingsWindowShortcuts(window.webContents);
		await loadAppRoute(window, '/settings');
		window.once('ready-to-show', () => {
			settingsWindow?.show();
			settingsWindow?.focus();
		});
		window.on('close', (event) => {
			if (shouldKeepWindowsAlive()) {
				event.preventDefault();
				hideSettingsWindow();
			}
		});
		window.on('closed', () => {
			if (settingsWindow === window) settingsWindow = null;
		});
		return window;
	}

	function showMainWindow() {
		const window = mainWindow;
		if (!windowCanShow(window)) {
			void createMainWindow();
			return;
		}
		window.show();
		window.focus();
		getTray()?.setToolTip('Cometline');
	}

	function revealMiniWindow(window: BrowserWindow) {
		if (window.isMinimized()) window.restore();
		layoutMiniWindowOnCursorDisplay();
		// showInactive avoids the app-activation hitch that window.show()
		// pays on macOS; focus() then puts keystrokes in the composer.
		if (typeof window.showInactive === 'function') window.showInactive();
		else window.show();
		if (miniNeedsActivation) {
			window.webContents.send('cometline:activate-mini-window');
			miniNeedsActivation = false;
		}
		window.focus();
	}

	async function showMiniWindow() {
		if (windowCanShow(miniWindow)) {
			revealMiniWindow(miniWindow);
			return;
		}
		const window = await prepareMiniWindow();
		if (!windowCanShow(window)) return;
		revealMiniWindow(window);
	}

	async function showSettingsWindow() {
		const window = settingsWindow;
		if (!windowCanShow(window)) {
			await createSettingsWindow();
			return;
		}
		if (window.isMinimized()) window.restore();
		window.show();
		window.focus();
	}

	async function openSessionInMainWindow(sessionId: unknown) {
		const cleanSessionId = typeof sessionId === 'string' ? sessionId.trim() : '';
		if (!cleanSessionId) return false;
		if (!windowCanShow(mainWindow)) await createMainWindow();
		const window = mainWindow;
		if (!windowCanShow(window)) return false;
		if (window.isMinimized()) window.restore();
		await loadAppRoute(window, `/session/${encodeURIComponent(cleanSessionId)}`);
		window.show();
		window.focus();
		hideMiniWindow();
		return true;
	}

	async function triggerMainWindowOnboarding(channel: string) {
		if (!windowCanShow(mainWindow)) await createMainWindow();
		const window = mainWindow;
		if (!windowCanShow(window)) return false;
		if (window.isMinimized()) window.restore();
		window.show();
		window.focus();
		window.webContents.send(channel);
		hideSettingsWindow();
		return true;
	}

	function handleAppActivate() {
		if (Date.now() < ignoreActivateUntil) return;
		for (const window of [settingsWindow, miniWindow, mainWindow]) {
			if (!windowCanShow(window) || !window.isVisible()) continue;
			if (window.isMinimized()) window.restore();
			window.focus();
			return;
		}
		if (windowCanShow(mainWindow)) {
			showMainWindow();
			return;
		}
		if (getAllWindows().length === 0) void createMainWindow();
	}

	async function toggleMiniWindow() {
		const window = miniWindow;
		if (windowCanShow(window) && window.isVisible()) {
			if (window.isFocused()) hideMiniWindow();
			else window.focus();
			return;
		}
		await showMiniWindow();
	}

	function isMacOS13OrLater() {
		return platform === 'darwin' && Number(os.release().split('.')[0]) >= 22;
	}

	function readLoginItemState(): LoginItemState {
		if (!app.isPackaged) return { openAtLogin: false, status: 'unsupported' };
		const query: LoginItemSettingsOptions | undefined =
			platform === 'darwin' && isMacOS13OrLater() ? { type: 'mainAppService' } : undefined;
		const login = app.getLoginItemSettings(query);
		return {
			openAtLogin: Boolean(login.openAtLogin),
			status: login.status ?? (login.openAtLogin ? 'enabled' : 'not-registered')
		};
	}

	function applyOpenAtLoginSetting(openAtLogin: unknown): ApplyOpenAtLoginResult {
		const wantsLogin = Boolean(openAtLogin);
		if (platform !== 'darwin' && platform !== 'win32') {
			return { openAtLogin: false, status: 'unsupported' };
		}
		if (!app.isPackaged) {
			if (wantsLogin) console.warn('Open at login is only supported in packaged builds.');
			return {
				openAtLogin: false,
				status: 'unsupported',
				isDev: wantsLogin && platform === 'darwin',
				message: 'Open at login is only supported in packaged builds.'
			};
		}

		const settings: Settings = { openAtLogin: wantsLogin };
		if (platform === 'darwin' && isMacOS13OrLater()) settings.type = 'mainAppService';
		else if (platform === 'darwin') settings.openAsHidden = false;

		try {
			app.setLoginItemSettings(settings);
		} catch (error) {
			console.error('setLoginItemSettings failed:', error);
			return {
				openAtLogin: false,
				status: 'error',
				message: error instanceof Error ? error.message : String(error)
			};
		}

		try {
			const current = readLoginItemState();
			const needsApproval =
				platform === 'darwin' &&
				wantsLogin &&
				['requires-approval', 'not-registered', 'not-found'].includes(current.status);
			if (needsApproval) void shell.openExternal(MACOS_LOGIN_ITEMS_SETTINGS_URL);
			return {
				...current,
				needsApproval: current.status === 'requires-approval',
				openedSettings: needsApproval,
				isDev: !app.isPackaged && platform === 'darwin' && wantsLogin
			};
		} catch (error) {
			console.error('getLoginItemSettings failed:', error);
			if (platform === 'darwin' && wantsLogin)
				void shell.openExternal(MACOS_LOGIN_ITEMS_SETTINGS_URL);
			return {
				openAtLogin: wantsLogin,
				status: 'unknown',
				needsApproval: wantsLogin && platform === 'darwin',
				openedSettings: wantsLogin && platform === 'darwin',
				isDev: !app.isPackaged && platform === 'darwin' && wantsLogin
			};
		}
	}

	return {
		applyOpenAtLoginSetting,
		createMainWindow,
		getMainWindow: () => mainWindow,
		getMiniWindow: () => miniWindow,
		getSettingsWindow: () => settingsWindow,
		handleAppActivate,
		hideMainWindow,
		hideMiniWindow,
		hideSettingsWindow,
		openSessionInMainWindow,
		prepareMiniWindow,
		readLoginItemState,
		showMainWindow,
		showSettingsWindow,
		toggleMiniWindow,
		triggerMainWindowOnboarding
	};
}
