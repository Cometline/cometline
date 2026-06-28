const {
	app,
	BrowserWindow,
	dialog,
	globalShortcut,
	ipcMain,
	protocol,
	net,
	shell,
	Tray,
	Menu,
	nativeImage,
	Notification: ElectronNotification,
	screen: electronScreen
} = require('electron');
const path = require('path');
const { pathToFileURL } = require('url');
const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const http = require('http');
// eslint-disable-next-line no-redeclare
const crypto = require('crypto');
const {
	defaultSettings,
	normalizeProviders,
	normalizeSettings,
	parseAndNormalizeSettings,
	validateSettings
} = require('./settings-schema.cjs');

app.setName('Cometline');

const MACOS_LOGIN_ITEMS_SETTINGS_URL =
	'x-apple.systempreferences:com.apple.LoginItems-Settings.extension';

function getAutoUpdater() {
	return require('electron-updater').autoUpdater;
}

const COMETMIND_PORT = 7700;
// Custom scheme used to serve the packaged SvelteKit bundle. Loading the
// fallback index.html over file:// breaks because adapter-static emits
// absolute asset paths (/_app/immutable/...) that resolve against the
// filesystem root. Serving the bundle through a registered standard scheme
// makes those absolute paths resolve against the bundle root instead.
const APP_SCHEME = 'app';
const APP_HOST = 'bundle';
const APP_ORIGIN = `${APP_SCHEME}://${APP_HOST}`;
/** Minimum window size — chat-only layout works below the sidebar breakpoint (900px). */
const MIN_WINDOW_WIDTH = 400;
const MIN_WINDOW_HEIGHT = 480;
const MINI_WINDOW_WIDTH = 460;
const MINI_WINDOW_HEIGHT = 640;
const MINI_WINDOW_MIN_WIDTH = 360;
const MINI_WINDOW_MIN_HEIGHT = 440;
const MINI_WINDOW_SCREEN_MARGIN = 18;
const HEALTH_URL = `http://127.0.0.1:${COMETMIND_PORT}/api/v1/health`;
const MAX_RETRIES = 50;
const POLL_MS = 100;
const UPDATE_CHECK_INTERVAL_MS = 4 * 60 * 60 * 1000;
const LOG_MAX_BYTES = 10 * 1024 * 1024;
const LOG_BACKUP_COUNT = 1;
const LOG_ROTATE_CHECK_BYTES = 512 * 1024;
const CODEX_BASE_URL = 'https://chatgpt.com/backend-api/codex';
const CODEX_CLIENT_ID = 'app_EMoamEEZ73f0CkXaXp7hrann';
const CODEX_REFRESH_URL = 'https://auth.openai.com/oauth/token';
const CODEX_CLIENT_VERSION = '1.0.0';
const CODEX_AUTH_CALLBACK_PORT = 1455;
const CODEX_AUTH_CALLBACK_PATH = '/auth/callback';
const CODEX_AUTH_TIMEOUT_MS = 5 * 60 * 1000;
const MCP_OAUTH_CALLBACK_PORT = 1456;
const MCP_OAUTH_CALLBACK_PATH = '/mcp/oauth/callback';

function rotateLogIfNeeded(logPath) {
	try {
		if (!fs.existsSync(logPath)) return;
		const { size } = fs.statSync(logPath);
		if (size < LOG_MAX_BYTES) return;
		const oldest = `${logPath}.${LOG_BACKUP_COUNT}`;
		if (fs.existsSync(oldest)) fs.unlinkSync(oldest);
		fs.renameSync(logPath, oldest);
	} catch (err) {
		console.error(`Failed to rotate log ${logPath}:`, err);
	}
}

function createRotatingLogWriter(logPath) {
	rotateLogIfNeeded(logPath);
	let stream = fs.createWriteStream(logPath, { flags: 'a' });
	let bytesSinceCheck = 0;

	function maybeRotate() {
		try {
			if (!fs.existsSync(logPath)) return;
			if (fs.statSync(logPath).size < LOG_MAX_BYTES) return;
			stream.end();
			rotateLogIfNeeded(logPath);
			stream = fs.createWriteStream(logPath, { flags: 'a' });
			bytesSinceCheck = 0;
		} catch (err) {
			console.error(`Failed to rotate log ${logPath}:`, err);
		}
	}

	return {
		write(data) {
			stream.write(data);
			bytesSinceCheck += data.length;
			if (bytesSinceCheck >= LOG_ROTATE_CHECK_BYTES) {
				bytesSinceCheck = 0;
				maybeRotate();
			}
		},
		end() {
			stream.end();
		}
	};
}

// Must run before app `ready`. Marking the scheme standard + secure gives the
// loaded page a normal web origin (so history API routing, fetch, and module
// scripts behave like https) instead of the restricted file:// origin.
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
	}
]);

let mainWindow = null;
let miniWindow = null;
let tray = null;
let cometMindProcess = null;
let cometMindGatewayProcess = null;
let stoppingForQuit = false;
let stoppedForQuit = false;
let relaunchForUpdate = false;
let updateCheckTimer = null;
let windowButtonAnimationTimer = null;
let windowButtonPosition = { x: 16, y: 17 };
let registeredMiniWindowShortcut = '';

const hasSingleInstanceLock = app.requestSingleInstanceLock();

if (!hasSingleInstanceLock) {
	app.quit();
}

// Vertically center the native buttons on the sidebar search bar. The titlebar
// row now sits flush against the window top, so the search input center is:
// titlebar row top padding (10px) + half the 28px search input (14px) = 24px.
// A traffic light is ~14px tall, so y = 24 - 7 = 17 lines the centers up.
const WINDOW_BUTTON_OPEN_POSITION = { x: 16, y: 17 };
const WINDOW_BUTTON_CLOSED_POSITION = { x: 17, y: 17 };
const WINDOW_BUTTON_DEFAULT_DURATION = 240;
const sidebarChromeEase = cubicBezier(0.22, 1, 0.36, 1);

function cubicBezier(x1, y1, x2, y2) {
	function sampleCurveX(t) {
		return ((1 - 3 * x2 + 3 * x1) * t + (3 * x2 - 6 * x1)) * t * t + 3 * x1 * t;
	}

	function sampleCurveY(t) {
		return ((1 - 3 * y2 + 3 * y1) * t + (3 * y2 - 6 * y1)) * t * t + 3 * y1 * t;
	}

	function sampleCurveDerivativeX(t) {
		return (3 * (1 - 3 * x2 + 3 * x1) * t + 2 * (3 * x2 - 6 * x1)) * t + 3 * x1;
	}

	function solveCurveX(x) {
		let t = x;
		for (let i = 0; i < 8; i++) {
			const currentX = sampleCurveX(t) - x;
			if (Math.abs(currentX) < 0.000001) return t;
			const derivative = sampleCurveDerivativeX(t);
			if (Math.abs(derivative) < 0.000001) break;
			t -= currentX / derivative;
		}

		let start = 0;
		let end = 1;
		t = x;
		for (let i = 0; i < 12; i++) {
			const currentX = sampleCurveX(t);
			if (Math.abs(currentX - x) < 0.000001) return t;
			if (x > currentX) start = t;
			else end = t;
			t = (end - start) * 0.5 + start;
		}
		return t;
	}

	return (x) => {
		if (x <= 0) return 0;
		if (x >= 1) return 1;
		return sampleCurveY(solveCurveX(x));
	};
}

function setWindowButtonPosition(position) {
	if (
		process.platform !== 'darwin' ||
		!mainWindow ||
		typeof mainWindow.setWindowButtonPosition !== 'function'
	) {
		return;
	}
	const next = { x: Math.round(position.x), y: Math.round(position.y) };
	mainWindow.setWindowButtonPosition(next);
	windowButtonPosition = next;
}

function animateWindowButtons(payload) {
	if (process.platform !== 'darwin' || !mainWindow) return;

	const open = typeof payload?.open === 'boolean' ? payload.open : Boolean(payload);
	const target = open ? WINDOW_BUTTON_OPEN_POSITION : WINDOW_BUTTON_CLOSED_POSITION;
	const rawDuration = Number(payload?.duration);
	const duration = Number.isFinite(rawDuration)
		? Math.max(0, Math.min(rawDuration, 1000))
		: WINDOW_BUTTON_DEFAULT_DURATION;
	const start = { ...windowButtonPosition };

	if (windowButtonAnimationTimer) {
		clearTimeout(windowButtonAnimationTimer);
		windowButtonAnimationTimer = null;
	}

	if (duration <= 16 || (start.x === target.x && start.y === target.y)) {
		setWindowButtonPosition(target);
		return;
	}

	const startedAt = Date.now();
	const step = () => {
		if (!mainWindow) return;
		const progress = Math.min(1, (Date.now() - startedAt) / duration);
		const eased = sidebarChromeEase(progress);
		setWindowButtonPosition({
			x: start.x + (target.x - start.x) * eased,
			y: start.y + (target.y - start.y) * eased
		});

		if (progress < 1) {
			windowButtonAnimationTimer = setTimeout(step, 16);
		} else {
			windowButtonAnimationTimer = null;
			setWindowButtonPosition(target);
		}
	};
	step();
}

function sendFullScreenState() {
	if (!mainWindow || mainWindow.isDestroyed()) return;
	mainWindow.webContents.send('cometline:fullscreen-changed', mainWindow.isFullScreen());
}

function resolveCometMindBinary() {
	if (process.env.COMETMIND_BINARY_PATH) {
		return process.env.COMETMIND_BINARY_PATH;
	}
	if (app.isPackaged) {
		return path.join(process.resourcesPath, 'cometmind');
	}
	// Dev: repository layout from cometline/electron/main.cjs
	const devCandidate = path.join(__dirname, '..', '..', 'cometmind', 'dist', 'cometmind');
	if (fs.existsSync(devCandidate)) return devCandidate;
	return path.join(__dirname, '..', '..', 'cometmind', 'cometmind');
}

function cometMindCliBinDirs() {
	const home = os.homedir();
	const dirs = [path.join(home, '.cometmind', 'bin'), path.join(home, '.local', 'bin')];
	if (process.platform === 'darwin') dirs.push('/opt/homebrew/bin', '/usr/local/bin');
	return dirs;
}

function installCometMindCliShim() {
	if (process.platform === 'win32') return;
	const binary = resolveCometMindBinary();
	if (!fs.existsSync(binary)) return;

	for (const dir of cometMindCliBinDirs()) {
		try {
			fs.mkdirSync(dir, { recursive: true });
			const shim = path.join(dir, 'cometmind');
			try {
				const stat = fs.lstatSync(shim);
				if (!stat.isSymbolicLink()) continue;
				if (fs.readlinkSync(shim) === binary) continue;
				fs.unlinkSync(shim);
			} catch (err) {
				if (err.code !== 'ENOENT') throw err;
			}
			fs.symlinkSync(binary, shim);
		} catch (err) {
			console.warn(`Unable to install CometMind CLI shim in ${dir}:`, err);
		}
	}
}

function pathWithCometMindCliBins(envPath = '') {
	const delimiter = path.delimiter;
	const existing = String(envPath || '')
		.split(delimiter)
		.filter(Boolean);
	const entries = [...cometMindCliBinDirs(), ...existing];
	return [...new Set(entries)].join(delimiter);
}

function resolveSystemPromptPath(variant = 'default') {
	if (process.env.COMETMIND_SYSTEM_PROMPT_PATH) {
		return path.resolve(process.env.COMETMIND_SYSTEM_PROMPT_PATH);
	}
	const filename = variant === 'man' ? 'SOUL_MAN.md' : 'SOUL.md';
	if (app.isPackaged) {
		return path.join(process.resourcesPath, filename);
	}
	return path.join(__dirname, '..', filename);
}

function getLogPath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'cometline.log');
}

function getSettingsPath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'cometline-settings.json');
}

function settingsNormalizeOptions(iconVariant = 'default') {
	return {
		fallbackWorkspacePath: readStoredWorkspacePath() || getDefaultWorkspacePath(),
		systemPromptPath: resolveSystemPromptPath(iconVariant)
	};
}

function readSavedIconVariant(saved) {
	return saved?.app?.iconVariant === 'man' ? 'man' : 'default';
}

function resolveNextIconVariant(settings, current) {
	if (settings.app?.iconVariant === 'man' || settings.app?.iconVariant === 'default') {
		return settings.app.iconVariant;
	}
	return current.app?.iconVariant === 'man' ? 'man' : 'default';
}

function shortcutKeyMatches(a, b) {
	return a === b || String(a).toLowerCase() === String(b).toLowerCase();
}

function matchesInputShortcut(input, binding) {
	if (input.type !== 'keyDown' || !binding?.key) return false;
	if (!shortcutKeyMatches(input.key, binding.key)) return false;

	const expectsCommand = binding.command ?? false;
	if (expectsCommand) {
		if (!(input.control || input.meta)) return false;
		if (binding.alt !== undefined ? binding.alt !== input.alt : input.alt) return false;
		if (binding.shift !== undefined ? binding.shift !== input.shift : input.shift) return false;
		return true;
	}

	if (binding.ctrl !== undefined && binding.ctrl !== input.control) return false;
	if (binding.meta !== undefined && binding.meta !== input.meta) return false;
	if (binding.alt !== undefined && binding.alt !== input.alt) return false;
	if (binding.shift !== undefined && binding.shift !== input.shift) return false;
	return true;
}

function acceleratorKeyForShortcut(key) {
	const normalized = String(key || '').trim();
	if (!normalized) return '';
	const named = {
		ArrowUp: 'Up',
		ArrowDown: 'Down',
		ArrowLeft: 'Left',
		ArrowRight: 'Right',
		',': 'Comma',
		'.': 'Period',
		Escape: 'Escape',
		Esc: 'Escape',
		Enter: 'Enter',
		Space: 'Space',
		' ': 'Space'
	};
	if (named[normalized]) return named[normalized];
	if (/^F\d{1,2}$/i.test(normalized)) return normalized.toUpperCase();
	if (normalized.length === 1) return normalized.toUpperCase();
	return normalized;
}

function shortcutBindingToAccelerator(binding) {
	if (!binding?.key) return '';
	const modifiers = [];
	if (binding.command) modifiers.push('CommandOrControl');
	if (binding.ctrl) modifiers.push('Control');
	if (binding.meta) modifiers.push('Meta');
	if (binding.alt) modifiers.push('Alt');
	if (binding.shift) modifiers.push('Shift');
	const key = acceleratorKeyForShortcut(binding.key);
	if (!key) return '';
	return [...modifiers, key].join('+');
}

let shortcutCaptureActive = false;
let sessionNavigationSuspended = false;
let webPanelOpen = false;

function sendCloseWebPanel() {
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:close-web-panel');
	}
}

function sendToggleWebPanel() {
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:toggle-web-panel');
	}
}

function sendOpenWebPanel() {
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:open-web-panel');
	}
}

function loadAppRoute(window, route = '/') {
	const cleanRoute = String(route || '/').startsWith('/') ? String(route || '/') : `/${route}`;
	if (app.isPackaged) {
		return window.loadURL(`${APP_ORIGIN}${cleanRoute}`);
	}
	return window.loadURL(`http://127.0.0.1:5173${cleanRoute}`);
}

function attachExternalNavigationGuards(window) {
	window.webContents.setWindowOpenHandler(({ url }) => {
		if (isExternallyOpenableUrl(url)) void shell.openExternal(url);
		return { action: 'deny' };
	});
	window.webContents.on('will-navigate', (event, url) => {
		const allowed = url.startsWith(`${APP_ORIGIN}/`) || url.startsWith('http://127.0.0.1:5173');
		if (!allowed) {
			event.preventDefault();
			if (isExternallyOpenableUrl(url)) void shell.openExternal(url);
		}
	});
}

function windowCanShow(window) {
	return Boolean(window && !window.isDestroyed());
}

function positionMiniWindowBottomRight() {
	if (!windowCanShow(miniWindow)) return;
	const cursorPoint = electronScreen.getCursorScreenPoint();
	const display = electronScreen.getDisplayNearestPoint(cursorPoint);
	const { width, height } = miniWindow.getBounds();
	const x = Math.round(display.workArea.x + display.workArea.width - width - MINI_WINDOW_SCREEN_MARGIN);
	const y = Math.round(display.workArea.y + display.workArea.height - height - MINI_WINDOW_SCREEN_MARGIN);
	miniWindow.setPosition(x, y, false);
}

// macOS menu bar icons are 16pt. Ship trayIcon.png (16px) + trayIcon@2x.png (32px);
// Electron picks @2x on Retina when both sit in the same folder.
// Regenerate from buildResources/icon.png (center-crop ~850px) — see docs/postmortem/macos-tray-icon-oversized-and-gray.md.
function resolveTrayResourcePath(filename) {
	if (app.isPackaged) {
		return path.join(process.resourcesPath, filename);
	}
	return path.join(app.getAppPath(), 'buildResources', filename);
}

function loadMacOSTrayImage(baseFilename, { template = false } = {}) {
	const imagePath = resolveTrayResourcePath(baseFilename);
	if (!fs.existsSync(imagePath)) return null;

	const image = nativeImage.createFromPath(imagePath);
	if (image.isEmpty()) return null;

	const size = image.getSize();
	const scaleFactors = image.getScaleFactors();
	// Legacy single 32px asset without an @2x pair is interpreted as 32pt (huge).
	if (scaleFactors.length === 1 && scaleFactors[0] === 1 && size.width > 18) {
		const resized = image.resize({ width: 16, height: 16, quality: 'best' });
		if (resized.isEmpty()) return null;
		if (template) resized.setTemplateImage(true);
		return resized;
	}

	if (template) image.setTemplateImage(true);
	return image;
}

function resolveTrayIconCandidates(variant = 'default') {
	const trayIcon = variant === 'man' ? 'trayIcon_man.png' : 'trayIcon.png';
	if (process.platform === 'darwin') {
		const candidates = [trayIcon];
		if (variant === 'man') candidates.push('trayIcon.png');
		candidates.push('trayTemplate.png');
		return candidates;
	}
	if (app.isPackaged) {
		return [trayIcon, 'icon.png'];
	}
	return [trayIcon, 'icon.png'];
}

function resolveTrayIcon(variant = 'default') {
	const candidates = resolveTrayIconCandidates(variant);
	for (const filename of candidates) {
		const resourcePath = resolveTrayResourcePath(filename);
		if (!fs.existsSync(resourcePath)) continue;

		if (process.platform === 'darwin') {
			const isTemplateAsset = filename === 'trayTemplate.png';
			const image = loadMacOSTrayImage(filename, { template: isTemplateAsset });
			if (image) {
				if (!app.isPackaged) {
					console.log(
						'[tray] Using',
						resourcePath,
						image.getSize(),
						image.getScaleFactors()
					);
				}
				return image;
			}
			continue;
		}

		const source = nativeImage.createFromPath(resourcePath);
		if (source.isEmpty()) continue;
		return source.resize({ width: 18, height: 18, quality: 'best' });
	}

	const checked = candidates.map((name) => resolveTrayResourcePath(name));
	console.warn('[tray] No tray icon found; checked:', checked.join(', '));
	return null;
}

function ensureTray() {
	if (process.platform !== 'darwin') return false;
	if (tray) return true;

	const variant = getIconVariant();
	const trayIconPath = resolveTrayResourcePath(
		variant === 'man' ? 'trayIcon_man.png' : 'trayIcon.png'
	);
	const icon = resolveTrayIcon(variant);
	if (!icon || icon.isEmpty()) {
		console.warn('[tray] Failed to create menu bar icon');
		return false;
	}

	// Prefer the file path on macOS so Electron loads trayIcon@2x.png for Retina.
	// Passing a NativeImage object can fail to show when dimensions look correct in logs.
	const trayImageSource = fs.existsSync(trayIconPath) ? trayIconPath : icon;
	tray = new Tray(trayImageSource);
	// Keep a strong reference; a collected Tray can vanish from the menu bar in dev.
	global.__cometlineTray = tray;
	tray.setToolTip('Cometline');
	const menu = Menu.buildFromTemplate([
		{
			label: 'Show Cometline',
			click: () => showMainWindow()
		},
		{ type: 'separator' },
		{
			label: 'Quit Cometline',
			click: () => app.quit()
		}
	]);
	tray.setContextMenu(menu);
	tray.on('click', () => showMainWindow());

	// macOS may hide new status items when the menu bar is crowded; re-assert once.
	setTimeout(() => {
		if (!tray) return;
		tray.setImage(trayImageSource);
	}, 500);

	if (!app.isPackaged) {
		console.log('[tray] Menu bar icon ready', trayImageSource);
		console.log(
			'[tray] If the icon is missing, macOS may be hiding menu bar extras — quit other tray apps or check System Settings → Control Center → Menu Bar Only.'
		);
	}
	return true;
}

function destroyTray() {
	if (!tray) return;
	tray.destroy();
	tray = null;
}

function showMainWindow() {
	if (!mainWindow || mainWindow.isDestroyed()) {
		void createWindow();
		return;
	}
	mainWindow.show();
	mainWindow.focus();
	if (tray) {
		tray.setToolTip('Cometline');
	}
}

async function openSessionInMainWindow(sessionId) {
	const cleanSessionId = typeof sessionId === 'string' ? sessionId.trim() : '';
	if (!cleanSessionId) return false;
	if (!windowCanShow(mainWindow)) {
		await createWindow();
	}
	if (!windowCanShow(mainWindow)) return false;
	if (mainWindow.isMinimized()) {
		mainWindow.restore();
	}
	await loadAppRoute(mainWindow, `/session/${encodeURIComponent(cleanSessionId)}`);
	mainWindow.show();
	mainWindow.focus();
	hideMiniWindow();
	return true;
}

function hideMainWindow() {
	if (!mainWindow || mainWindow.isDestroyed()) return;
	if (mainWindow.isFullScreen()) {
		mainWindow.once('leave-full-screen', () => {
			mainWindow?.hide();
			ensureTray();
		});
		mainWindow.setFullScreen(false);
		return;
	}
	mainWindow.hide();
	if (!ensureTray()) {
		console.warn('[tray] Failed to create menu bar icon after hide');
	} else if (tray) {
		tray.setToolTip('Cometline (hidden)');
	}
}

async function showMiniWindow() {
	if (!windowCanShow(miniWindow)) {
		await createMiniWindow();
		return;
	}
	if (miniWindow.isMinimized()) {
		miniWindow.restore();
	}
	miniWindow.show();
	miniWindow.focus();
}

function hideMiniWindow() {
	if (!windowCanShow(miniWindow)) return;
	writeMiniWindowState({ lastActiveAt: Date.now() });
	miniWindow.hide();
}

async function toggleMiniWindow() {
	if (!windowCanShow(miniWindow) || !miniWindow.isVisible()) {
		await showMiniWindow();
		return;
	}
	if (miniWindow.isFocused()) {
		hideMiniWindow();
		return;
	}
	miniWindow.focus();
}

function isDarwinCloseWindowShortcut(input) {
	return (
		process.platform === 'darwin' &&
		input.type === 'keyDown' &&
		input.meta &&
		!input.control &&
		!input.alt &&
		!input.shift &&
		input.key?.toLowerCase() === 'w'
	);
}

function handleDarwinCloseWindowShortcut(event, input, onCloseWindow) {
	if (!isDarwinCloseWindowShortcut(input)) return false;
	event.preventDefault();
	if (onCloseWindow === hideMainWindow && webPanelOpen) {
		sendCloseWebPanel();
	} else {
		onCloseWindow();
	}
	return true;
}

function attachWindowShortcuts(webContents, { onCloseWindow, includeSessionNavigation = false }) {
	webContents.on('before-input-event', (event, input) => {
		if (handleDarwinCloseWindowShortcut(event, input, onCloseWindow)) {
			return;
		}

		if (!includeSessionNavigation) return;
		if (shortcutCaptureActive || sessionNavigationSuspended) return;
		const shortcuts = readProviderSettings().shortcuts ?? defaultSettings().shortcuts;
		if (matchesInputShortcut(input, shortcuts.previousSession)) {
			event.preventDefault();
			webContents.send('cometline:navigate-session', 'prev');
			return;
		}
		if (matchesInputShortcut(input, shortcuts.nextSession)) {
			event.preventDefault();
			webContents.send('cometline:navigate-session', 'next');
		}
	});
}

function attachMainWindowShortcuts(webContents) {
	attachWindowShortcuts(webContents, {
		onCloseWindow: hideMainWindow,
		includeSessionNavigation: true
	});
}

function attachMiniWindowShortcuts(webContents) {
	attachWindowShortcuts(webContents, {
		onCloseWindow: hideMiniWindow
	});
}

function handleWebPanelGuestShortcuts(event, input) {
	if (handleDarwinCloseWindowShortcut(event, input, hideMainWindow)) {
		return true;
	}
	if (shortcutCaptureActive || sessionNavigationSuspended) return false;
	const shortcuts = readProviderSettings().shortcuts ?? defaultSettings().shortcuts;
	if (matchesInputShortcut(input, shortcuts.toggleWebPanel)) {
		event.preventDefault();
		sendToggleWebPanel();
		return true;
	}
	if (matchesInputShortcut(input, shortcuts.openWebPanel)) {
		event.preventDefault();
		sendOpenWebPanel();
		return true;
	}
	return false;
}

function attachWebviewPanelShortcuts(webContents) {
	webContents.on('before-input-event', (event, input) => {
		handleWebPanelGuestShortcuts(event, input);
	});
}

function readProviderSettings() {
	const fromEnv = {
		activeProviderId: process.env.COMETMIND_PROVIDER,
		baseURL: process.env.COMETMIND_BASE_URL,
		apiKey:
			process.env.COMETMIND_API_KEY ||
			process.env.OPENAI_API_KEY ||
			process.env.ANTHROPIC_API_KEY,
		selectedModel: process.env.COMETMIND_MODEL
	};

	const base = readSavedProviderSettings();

	// Allow env overrides for the active provider only. Apply provider first so
	// key/baseURL/model attach to the provider selected by COMETMIND_PROVIDER.
	if (fromEnv.activeProviderId) {
		const matched = base.providers.find((p) => p.id === fromEnv.activeProviderId.trim());
		if (matched) base.activeProviderId = matched.id;
	}
	const active = base.providers.find((p) => p.id === base.activeProviderId) ?? base.providers[0];
	if (fromEnv.baseURL) active.baseURL = fromEnv.baseURL.trim();
	if (fromEnv.apiKey) active.apiKey = fromEnv.apiKey.trim();
	if (fromEnv.selectedModel) {
		const model = fromEnv.selectedModel.trim();
		active.selectedModel = model;
		active.enabled = true;
		if (model && !active.models.includes(model)) active.models = [...active.models, model];
		if (model && !active.enabledModels.includes(model)) active.enabledModels = [model];
	}

	return base;
}

function readSavedProviderSettings() {
	let saved = {};
	const settingsPath = getSettingsPath();
	if (fs.existsSync(settingsPath)) {
		try {
			saved = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
		} catch {
			saved = {};
		}
	}

	const iconVariant = readSavedIconVariant(saved);
	return parseAndNormalizeSettings(saved, settingsNormalizeOptions(iconVariant));
}

function writeProviderSettings(settings) {
	const current = readSavedProviderSettings();
	const nextProviders = Array.isArray(settings.providers)
		? normalizeProviders(settings.providers)
		: current.providers;
	const requestedActive = nextProviders.find((p) => p.id === settings.activeProviderId);
	const nextActive =
		requestedActive?.enabled && requestedActive.enabledModels.length > 0
			? requestedActive.id
			: (nextProviders.find((p) => p.enabled && p.enabledModels.length > 0)?.id ??
				nextProviders[0]?.id ??
				'');
	const iconVariant = resolveNextIconVariant(settings, current);
	const nextCometMind = {
		...(settings.cometmind ?? current.cometmind),
		systemPromptPath: resolveSystemPromptPath(iconVariant)
	};
	const next = validateSettings(
		normalizeSettings(
			{
				providers: nextProviders,
				activeProviderId: nextActive,
				defaultModelId: settings.defaultModelId ?? current.defaultModelId ?? '',
				defaultProviderId: settings.defaultProviderId ?? current.defaultProviderId ?? '',
				appearance: settings.appearance ?? current.appearance,
				shortcuts: settings.shortcuts ?? current.shortcuts,
				cometmind: nextCometMind,
				app: { ...(current.app ?? {}), ...(settings.app ?? {}), iconVariant }
			},
			settingsNormalizeOptions(iconVariant)
		)
	);
	const settingsPath = getSettingsPath();
	fs.writeFileSync(settingsPath, JSON.stringify(next, null, 2));
	try {
		fs.chmodSync(settingsPath, 0o600);
	} catch {
		/* ignore */
	}
	return next;
}

function readMiniWindowState() {
	const settings = readProviderSettings();
	return {
		sessionId: String(settings.app?.miniWindowSessionId || ''),
		lastActiveAt: Number(settings.app?.miniWindowLastActiveAt || 0),
		inactivityTimeoutMinutes: Number(settings.app?.miniWindowInactivityTimeoutMinutes || 30)
	};
}

function writeMiniWindowState(partial) {
	const settings = readProviderSettings();
	settings.app = {
		...settings.app,
		...(typeof partial?.sessionId === 'string'
			? { miniWindowSessionId: partial.sessionId }
			: {}),
		...(Number.isFinite(partial?.lastActiveAt)
			? { miniWindowLastActiveAt: Math.max(0, Math.floor(partial.lastActiveAt)) }
			: {})
	};
	const saved = writeProviderSettings(settings);
	return {
		sessionId: String(saved.app?.miniWindowSessionId || ''),
		lastActiveAt: Number(saved.app?.miniWindowLastActiveAt || 0),
		inactivityTimeoutMinutes: Number(saved.app?.miniWindowInactivityTimeoutMinutes || 30)
	};
}

function refreshGlobalShortcuts() {
	if (!app.isReady()) return;
	if (registeredMiniWindowShortcut) {
		globalShortcut.unregister(registeredMiniWindowShortcut);
		registeredMiniWindowShortcut = '';
	}
	const shortcuts = readProviderSettings().shortcuts ?? defaultSettings().shortcuts;
	const accelerator = shortcutBindingToAccelerator(shortcuts.toggleMiniWindow);
	if (!accelerator) return;
	const registered = globalShortcut.register(accelerator, () => {
		if (shortcutCaptureActive) return;
		void toggleMiniWindow();
	});
	if (!registered) {
		console.warn(`Failed to register mini window shortcut: ${accelerator}`);
		return;
	}
	registeredMiniWindowShortcut = accelerator;
}

async function exportProviderSettings() {
	const settings = readSavedProviderSettings();
	const result = await dialog.showSaveDialog(mainWindow, {
		title: 'Export Cometline settings',
		defaultPath: 'cometline-settings.json',
		filters: [{ name: 'JSON', extensions: ['json'] }]
	});
	if (result.canceled || !result.filePath) {
		return { canceled: true };
	}

	fs.writeFileSync(result.filePath, JSON.stringify(settings, null, 2));
	try {
		fs.chmodSync(result.filePath, 0o600);
	} catch {
		/* ignore */
	}
	return { canceled: false, path: result.filePath };
}

async function importProviderSettings() {
	const result = await dialog.showOpenDialog(mainWindow, {
		title: 'Import Cometline settings',
		properties: ['openFile'],
		filters: [{ name: 'JSON', extensions: ['json'] }]
	});
	if (result.canceled || result.filePaths.length === 0) {
		return { canceled: true };
	}

	const filePath = result.filePaths[0];
	const raw = fs.readFileSync(filePath, 'utf8');
	const parsed = JSON.parse(raw);
	const iconVariant = readSavedIconVariant(parsed);
	const imported = validateSettings(
		parseAndNormalizeSettings(parsed, settingsNormalizeOptions(iconVariant))
	);
	const saved = writeProviderSettings(imported);
	await stopCometMind();
	startCometMind();
	await waitForHealth();
	await syncDiscordGatewayFromSettings(saved);
	applyOpenAtLoginSetting(saved.app?.openAtLogin);
	applyIconVariant(saved.app?.iconVariant);
	return { canceled: false, path: filePath, settings: saved };
}

function providerEnv() {
	const settings = readProviderSettings();
	const runtimeProviders = settings.providers.filter(
		(p) => p.enabled && p.enabledModels.length > 0
	);
	const active =
		runtimeProviders.find((p) => p.id === settings.activeProviderId) ??
		runtimeProviders[0] ??
		settings.providers[0];
	const env = {
		...process.env,
		PATH: pathWithCometMindCliBins(process.env.PATH),
		COMETMIND_PROVIDER: active.id,
		COMETMIND_MODEL: active.enabledModels[0] || active.selectedModel || active.models[0] || '',
		COMETMIND_MAX_TOKENS: String(settings.cometmind?.maxTokens ?? 2048),
		COMETMIND_LOG_LEVEL: settings.cometmind?.logLevel ?? 'error',
		COMETMIND_SYSTEM_PROMPT_PATH: resolveSystemPromptPath(getIconVariant())
	};
	if (active.baseURL) env.COMETMIND_BASE_URL = active.baseURL;
	if (active.apiKey) env.COMETMIND_API_KEY = active.apiKey;
	return env;
}

function getWorkspaceStoragePath() {
	const dir = path.join(os.homedir(), '.cometmind');
	if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
	return path.join(dir, 'cometline-workspace.json');
}

function workspacePathExists(candidate) {
	const clean = String(candidate || '').trim();
	if (!clean) return false;
	try {
		return fs.existsSync(clean) && fs.statSync(clean).isDirectory();
	} catch {
		return false;
	}
}

function writeWorkspaceStore({ workspacePath, recentPaths }) {
	fs.writeFileSync(
		getWorkspaceStoragePath(),
		JSON.stringify({ workspacePath, recentPaths }, null, 2)
	);
	fs.chmodSync(getWorkspaceStoragePath(), 0o600);
}

/** Drop recent/current paths whose directories no longer exist. */
function pruneWorkspaceStore() {
	const store = readWorkspaceStore();
	let workspacePath = store.workspacePath;
	if (workspacePath && !workspacePathExists(workspacePath)) {
		workspacePath = '';
	}
	const recentPaths = store.recentPaths.filter((item) => workspacePathExists(item));
	const removedRecent = store.recentPaths.length - recentPaths.length;
	const clearedCurrent = Boolean(store.workspacePath && !workspacePath);
	const changed = clearedCurrent || removedRecent > 0;
	if (changed) {
		writeWorkspaceStore({ workspacePath, recentPaths });
	}
	return { removedRecent, clearedCurrent };
}

function readWorkspaceStore() {
	try {
		const raw = fs.readFileSync(getWorkspaceStoragePath(), 'utf8');
		const parsed = JSON.parse(raw);
		return {
			workspacePath: String(parsed?.workspacePath || '').trim(),
			recentPaths: Array.isArray(parsed?.recentPaths)
				? parsed.recentPaths.map((item) => String(item || '').trim()).filter(Boolean)
				: []
		};
	} catch {
		return { workspacePath: '', recentPaths: [] };
	}
}

function readStoredWorkspacePath() {
	pruneWorkspaceStore();
	const { workspacePath } = readWorkspaceStore();
	if (workspacePath && workspacePathExists(workspacePath)) return path.resolve(workspacePath);
	return '';
}

function rememberWorkspacePath(workspacePath) {
	const clean = path.resolve(workspacePath);
	const store = readWorkspaceStore();
	const recentPaths = [
		clean,
		...store.recentPaths.filter((item) => path.resolve(item) !== clean)
	].slice(0, 20);
	writeWorkspaceStore({ workspacePath: clean, recentPaths });
	return clean;
}

function writeStoredWorkspacePath(workspacePath) {
	return rememberWorkspacePath(workspacePath);
}

function listRecentWorkspacePaths() {
	pruneWorkspaceStore();
	const store = readWorkspaceStore();
	const seen = new Set();
	const out = [];
	const add = (candidate) => {
		const clean = String(candidate || '').trim();
		if (!clean || !workspacePathExists(clean)) return;
		const resolved = path.resolve(clean);
		if (seen.has(resolved)) return;
		seen.add(resolved);
		out.push(resolved);
	};
	add(store.workspacePath);
	for (const item of store.recentPaths) add(item);
	return out;
}

function removeRecentWorkspacePath(workspacePath) {
	const clean = String(workspacePath || '').trim();
	if (!clean) return { removed: false };
	const target = path.resolve(clean);
	const store = readWorkspaceStore();
	const before = store.recentPaths.length;
	const recentPaths = store.recentPaths.filter((item) => path.resolve(item) !== target);
	if (recentPaths.length === before) {
		return { removed: false };
	}
	writeWorkspaceStore({ workspacePath: store.workspacePath, recentPaths });
	return { removed: true };
}

function filterExistingWorkspacePaths(paths) {
	const seen = new Set();
	const out = [];
	for (const candidate of paths) {
		const clean = String(candidate || '').trim();
		if (!clean || !workspacePathExists(clean)) continue;
		const resolved = path.resolve(clean);
		if (seen.has(resolved)) continue;
		seen.add(resolved);
		out.push(resolved);
	}
	return out;
}

function getDefaultWorkspacePath() {
	const defaultPath = path.join(os.homedir(), 'Cometline');
	if (!fs.existsSync(defaultPath)) {
		fs.mkdirSync(defaultPath, { recursive: true });
	}
	return defaultPath;
}

function getWorkspacePath() {
	if (process.env.COMETMIND_WORKSPACE_PATH) {
		return path.resolve(process.env.COMETMIND_WORKSPACE_PATH);
	}
	const stored = readStoredWorkspacePath();
	if (stored) return stored;
	return getDefaultWorkspacePath();
}

async function selectWorkspacePath() {
	const window = BrowserWindow.getFocusedWindow();
	const result = await dialog.showOpenDialog(window || undefined, {
		properties: ['openDirectory', 'createDirectory'],
		buttonLabel: 'Select workspace',
		title: 'Choose a workspace folder'
	});
	if (result.canceled || result.filePaths.length === 0) return null;
	return writeStoredWorkspacePath(result.filePaths[0]);
}

function getIconVariant() {
	const variant = readProviderSettings().app?.iconVariant;
	return variant === 'man' ? 'man' : 'default';
}

function resolveAppIconPaths(variant = 'default') {
	if (variant === 'man') {
		if (app.isPackaged) {
			return [path.join(process.resourcesPath, 'app_icon_man.png')];
		}
		return [
			path.join(app.getAppPath(), 'static', 'app_icon_man.png'),
			path.join(__dirname, '..', 'static', 'app_icon_man.png')
		];
	}
	if (app.isPackaged) {
		return [path.join(process.resourcesPath, 'icon.png')];
	}
	return [path.join(__dirname, '..', 'buildResources', 'icon.png')];
}

function getAppIconPath(variant = getIconVariant()) {
	return resolveAppIconPaths(variant).find((candidate) => fs.existsSync(candidate));
}

function resolveTrayImageSource(variant = getIconVariant()) {
	const trayIconPath = resolveTrayResourcePath(
		variant === 'man' ? 'trayIcon_man.png' : 'trayIcon.png'
	);
	if (fs.existsSync(trayIconPath)) return trayIconPath;
	const fallbackTrayPath = resolveTrayResourcePath('trayIcon.png');
	if (fs.existsSync(fallbackTrayPath)) return fallbackTrayPath;
	return resolveTrayIcon(variant);
}

function applyIconVariant(variant = getIconVariant()) {
	const iconPath = getAppIconPath(variant);
	if (!iconPath) {
		console.warn('[icon] No app icon found for variant', variant);
		return;
	}
	const image = nativeImage.createFromPath(iconPath);
	if (image.isEmpty()) {
		console.warn('[icon] Failed to load app icon for variant', variant, iconPath);
		return;
	}
	if (process.platform === 'darwin') {
		app.dock?.setIcon(image);
	}
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.setIcon(image);
	}
	if (!tray) return;
	const trayImageSource = resolveTrayImageSource(variant);
	if (typeof trayImageSource === 'string') {
		tray.setImage(trayImageSource);
		return;
	}
	if (trayImageSource) tray.setImage(trayImageSource);
}

function isCometMindRunning() {
	return cometMindProcess != null && cometMindProcess.exitCode === null;
}

function startCometMind() {
	if (isCometMindRunning()) return;
	if (cometMindProcess && cometMindProcess.exitCode !== null) {
		cometMindProcess = null;
	}

	const binary = resolveCometMindBinary();
	const logPath = getLogPath();
	const logStream = createRotatingLogWriter(logPath);

	if (!fs.existsSync(binary)) {
		console.error(`CometMind binary not found: ${binary}`);
		return;
	}

	const child = spawn(binary, ['serve', '--port', String(COMETMIND_PORT), '--watch-parent'], {
		stdio: ['ignore', 'pipe', 'pipe'],
		env: providerEnv()
	});
	cometMindProcess = child;

	child.stdout.on('data', (data) => logStream.write(data));
	child.stderr.on('data', (data) => logStream.write(data));

	child.on('exit', (code) => {
		console.log(`CometMind exited with code ${code}`);
		logStream.end();
		if (cometMindProcess === child) {
			cometMindProcess = null;
		}
	});

	child.on('error', (err) => {
		console.error('CometMind spawn error:', err);
		logStream.end();
		if (cometMindProcess === child) {
			cometMindProcess = null;
		}
	});
}

function getGatewayLogPath() {
	return getLogPath().replace(/\.log$/, '-gateway.log');
}

function isMacOS13OrLater() {
	return process.platform === 'darwin' && Number(os.release().split('.')[0]) >= 22;
}

function openMacLoginItemsSettings() {
	return shell.openExternal(MACOS_LOGIN_ITEMS_SETTINGS_URL);
}

function readLoginItemState() {
	if (!app.isPackaged) {
		return {
			openAtLogin: false,
			status: 'unsupported'
		};
	}
	const query =
		process.platform === 'darwin' && isMacOS13OrLater()
			? { type: 'mainAppService' }
			: undefined;
	const login = app.getLoginItemSettings(query);
	const status = login.status ?? (login.openAtLogin ? 'enabled' : 'not-registered');
	return {
		openAtLogin: Boolean(login.openAtLogin),
		status
	};
}

function applyOpenAtLoginSetting(openAtLogin) {
	const wantsLogin = Boolean(openAtLogin);
	if (process.platform !== 'darwin' && process.platform !== 'win32') {
		return { openAtLogin: false, status: 'unsupported' };
	}
	if (!app.isPackaged) {
		if (wantsLogin) {
			console.warn('Open at login is only supported in packaged builds.');
		}
		return {
			openAtLogin: false,
			status: 'unsupported',
			isDev: wantsLogin && process.platform === 'darwin',
			message: 'Open at login is only supported in packaged builds.'
		};
	}

	const settings = { openAtLogin: wantsLogin };
	if (process.platform === 'darwin' && isMacOS13OrLater()) {
		settings.type = 'mainAppService';
	} else if (process.platform === 'darwin') {
		settings.openAsHidden = false;
	}

	try {
		app.setLoginItemSettings(settings);
	} catch (err) {
		console.error('setLoginItemSettings failed:', err);
		return {
			openAtLogin: false,
			status: 'error',
			message: err instanceof Error ? err.message : String(err)
		};
	}

	try {
		const current = readLoginItemState();
		const needsApproval =
			process.platform === 'darwin' &&
			wantsLogin &&
			(current.status === 'requires-approval' ||
				current.status === 'not-registered' ||
				current.status === 'not-found');

		if (needsApproval) {
			void openMacLoginItemsSettings();
		}

		const devHint = !app.isPackaged && process.platform === 'darwin' && wantsLogin;

		return {
			openAtLogin: current.openAtLogin,
			status: current.status,
			needsApproval: current.status === 'requires-approval',
			openedSettings: needsApproval,
			isDev: devHint
		};
	} catch (err) {
		console.error('getLoginItemSettings failed:', err);
		if (process.platform === 'darwin' && wantsLogin) {
			void openMacLoginItemsSettings();
		}
		return {
			openAtLogin: wantsLogin,
			status: 'unknown',
			needsApproval: wantsLogin && process.platform === 'darwin',
			openedSettings: wantsLogin && process.platform === 'darwin',
			isDev: !app.isPackaged && process.platform === 'darwin' && wantsLogin
		};
	}
}

function startDiscordGateway() {
	if (cometMindGatewayProcess) return;

	const settings = readProviderSettings();
	const discord = settings.cometmind?.gateway?.discord ?? {};
	if (!String(discord.botToken ?? '').trim() && !process.env.DISCORD_BOT_TOKEN) {
		console.error('Discord gateway: bot token is not configured');
		return;
	}

	const binary = resolveCometMindBinary();
	const logPath = getGatewayLogPath();
	const logStream = createRotatingLogWriter(logPath);

	if (!fs.existsSync(binary)) {
		console.error(`CometMind binary not found: ${binary}`);
		return;
	}

	cometMindGatewayProcess = spawn(binary, ['gateway', 'run', '--platform', 'discord'], {
		stdio: ['ignore', 'pipe', 'pipe'],
		env: providerEnv()
	});

	cometMindGatewayProcess.stdout.on('data', (data) => logStream.write(data));
	cometMindGatewayProcess.stderr.on('data', (data) => logStream.write(data));

	cometMindGatewayProcess.on('exit', (code) => {
		console.log(`Discord gateway exited with code ${code}`);
		logStream.end();
		cometMindGatewayProcess = null;
	});

	cometMindGatewayProcess.on('error', (err) => {
		console.error('Discord gateway spawn error:', err);
		logStream.end();
		cometMindGatewayProcess = null;
	});
}

function stopDiscordGateway() {
	const proc = cometMindGatewayProcess;
	if (!proc) return Promise.resolve();
	cometMindGatewayProcess = null;

	return new Promise((resolve) => {
		let settled = false;
		let forceTimer = null;
		const finish = () => {
			if (settled) return;
			settled = true;
			if (forceTimer) clearTimeout(forceTimer);
			resolve();
		};

		proc.once('exit', finish);
		forceTimer = setTimeout(() => {
			try {
				proc.kill('SIGKILL');
			} catch {
				// ignore
			}
			finish();
		}, 6000);

		try {
			proc.kill('SIGTERM');
		} catch {
			finish();
		}
	});
}

async function syncDiscordGatewayFromSettings(settings) {
	const enabled = Boolean(settings?.cometmind?.gateway?.discord?.enabled);
	if (enabled) {
		await stopDiscordGateway();
		startDiscordGateway();
	} else {
		await stopDiscordGateway();
	}
}

function stopCometMind() {
	const proc = cometMindProcess;
	cometMindProcess = null;
	if (!proc) return Promise.resolve();

	return new Promise((resolve) => {
		let settled = false;
		let forceTimer = null;
		const finish = () => {
			if (settled) return;
			settled = true;
			if (forceTimer) clearTimeout(forceTimer);
			resolve();
		};

		if (proc.exitCode !== null) {
			finish();
			return;
		}

		// Wait for the process to actually exit so it releases the TCP port
		// (127.0.0.1:7700) and the SQLite WAL lock before a new `serve` spawns.
		// Spawning a replacement too early causes "address already in use" and
		// SQLITE_BUSY (database is locked) while both processes hold the DB.
		proc.once('exit', finish);

		// Escalate to SIGKILL if graceful shutdown stalls past the server's
		// 5s shutdown budget, then resolve once it is gone.
		forceTimer = setTimeout(() => {
			try {
				proc.kill('SIGKILL');
			} catch {
				// ignore
			}
			finish();
		}, 6000);

		try {
			proc.kill('SIGTERM');
		} catch {
			finish();
		}
	});
}

async function waitForHealth() {
	for (let i = 0; i < MAX_RETRIES; i++) {
		try {
			const res = await fetch(HEALTH_URL, { signal: AbortSignal.timeout(1000) });
			if (res.ok) return true;
		} catch {
			// keep polling
		}
		await new Promise((resolve) => setTimeout(resolve, POLL_MS));
	}
	return false;
}

// Tracks the latest known auto-update state so a freshly loaded renderer can
// query it via IPC instead of waiting for the next event.
let updateState = { status: 'idle' };

function setUpdateState(next) {
	updateState = { ...next, updatedAt: Date.now() };
	if (mainWindow && !mainWindow.isDestroyed()) {
		mainWindow.webContents.send('cometline:update-state', updateState);
	}
}

function configureAutoUpdater() {
	if (!app.isPackaged) return;

	// We surface a button in the UI and let the user choose when to restart, so
	// download automatically but never install behind their back.
	getAutoUpdater().autoDownload = true;
	getAutoUpdater().autoInstallOnAppQuit = false;
	getAutoUpdater().logger = {
		info: (message) => console.log(`[auto-updater] ${message}`),
		warn: (message) => console.warn(`[auto-updater] ${message}`),
		error: (message) => console.error(`[auto-updater] ${message}`),
		debug: (message) => console.debug(`[auto-updater] ${message}`)
	};

	getAutoUpdater().on('checking-for-update', () => {
		setUpdateState({ status: 'checking' });
	});

	getAutoUpdater().on('update-available', (info) => {
		setUpdateState({ status: 'downloading', version: info?.version, percent: 0 });
	});

	getAutoUpdater().on('update-not-available', (info) => {
		setUpdateState({ status: 'idle', version: info?.version });
	});

	getAutoUpdater().on('download-progress', (progress) => {
		setUpdateState({
			status: 'downloading',
			percent: Math.round(progress?.percent ?? 0)
		});
	});

	getAutoUpdater().on('update-downloaded', (info) => {
		setUpdateState({ status: 'ready', version: info?.version });
	});

	getAutoUpdater().on('error', (err) => {
		console.error('Auto-update error:', err);
		setUpdateState({ status: 'error', message: String(err?.message ?? err) });
	});

	const check = () => {
		getAutoUpdater()
			.checkForUpdates()
			.catch((err) => {
				console.error('Auto-update check failed:', err);
			});
	};

	check();
	updateCheckTimer = setInterval(check, UPDATE_CHECK_INTERVAL_MS);
}

function getBundleDir() {
	return path.join(__dirname, '..', 'build');
}

// Serves the packaged SvelteKit bundle over the custom app:// scheme. Absolute
// asset paths (e.g. /_app/immutable/x.js) map onto the bundle root, and any
// request that does not resolve to a real file falls back to index.html so the
// SPA router can handle client-side routes on reload.
function registerAppProtocol() {
	const bundleDir = getBundleDir();
	const fallback = path.join(bundleDir, 'index.html');

	protocol.handle(APP_SCHEME, (request) => {
		const requestUrl = new URL(request.url);
		// Decode and strip the leading slash so it resolves within the bundle.
		let relativePath = decodeURIComponent(requestUrl.pathname).replace(/^\/+/, '');
		if (!relativePath) relativePath = 'index.html';

		let resolved = path.normalize(path.join(bundleDir, relativePath));

		// Guard against path traversal escaping the bundle directory.
		const withinBundle = resolved === bundleDir || resolved.startsWith(bundleDir + path.sep);
		if (!withinBundle) {
			return new Response('Forbidden', { status: 403 });
		}

		// Fall back to the SPA shell for client-side routes (no real file).
		if (!fs.existsSync(resolved) || !fs.statSync(resolved).isFile()) {
			resolved = fallback;
		}

		return net.fetch(pathToFileURL(resolved).toString());
	});
}

async function createWindow() {
	const iconPath = getAppIconPath(getIconVariant());
	mainWindow = new BrowserWindow({
		width: 1200,
		height: 800,
		minWidth: MIN_WINDOW_WIDTH,
		minHeight: MIN_WINDOW_HEIGHT,
		// 'hidden' (not 'hiddenInset') is required for setWindowButtonPosition to
		// take effect: Electron only honors custom traffic-light positions on a
		// frameless window. With 'hiddenInset' the buttons are pinned at a fixed
		// inset and every setWindowButtonPosition call is silently ignored.
		titleBarStyle: 'hidden',
		...(process.platform === 'darwin'
			? {
					backgroundColor: '#00000000',
					transparent: true,
					vibrancy: 'sidebar',
					visualEffectState: 'active'
				}
			: {}),
		...(iconPath ? { icon: iconPath } : {}),
		show: false,
		webPreferences: {
			preload: path.join(__dirname, 'preload.cjs'),
			contextIsolation: true,
			nodeIntegration: false,
			allowRunningInsecureContent: false,
			webviewTag: true
		}
	});
	setWindowButtonPosition(WINDOW_BUTTON_OPEN_POSITION);
	if (process.platform === 'darwin' && iconPath) {
		const dockIcon = nativeImage.createFromPath(iconPath);
		if (!dockIcon.isEmpty()) app.dock?.setIcon(dockIcon);
	}

	attachExternalNavigationGuards(mainWindow);
	attachMainWindowShortcuts(mainWindow.webContents);
	mainWindow.webContents.on('did-attach-webview', (_event, webContents) => {
		attachWebviewPanelShortcuts(webContents);
	});

	await loadAppRoute(mainWindow, '/');

	mainWindow.once('ready-to-show', () => {
		mainWindow.show();
		// Re-apply once the window is realized; an early call right after
		// construction can be dropped before the buttons exist.
		setWindowButtonPosition(WINDOW_BUTTON_OPEN_POSITION);
		sendFullScreenState();
	});

	// macOS hides the native traffic lights in fullscreen, freeing the gutter.
	// Tell the renderer so it can reclaim that space for the search bar.
	mainWindow.on('enter-full-screen', sendFullScreenState);
	mainWindow.on('leave-full-screen', sendFullScreenState);

	// On macOS, Cmd+W (and the red traffic light) should hide the window rather
	// than destroy it. Recreating the BrowserWindow on every reopen forces a
	// cold renderer boot — reloading the whole SvelteKit bundle and reconnecting
	// to CometMind — which feels slow. Hiding keeps the renderer warm so the
	// next Dock click re-shows instantly. We still allow a real close during
	// quit (Cmd+Q / before-quit), where stoppingForQuit is set.
	mainWindow.on('close', (event) => {
		if (process.platform === 'darwin' && !stoppingForQuit && !stoppedForQuit) {
			event.preventDefault();
			hideMainWindow();
		}
	});

	mainWindow.on('closed', () => {
		if (windowButtonAnimationTimer) {
			clearTimeout(windowButtonAnimationTimer);
			windowButtonAnimationTimer = null;
		}
		mainWindow = null;
	});
}

async function createMiniWindow() {
	const iconPath = getAppIconPath(getIconVariant());
	miniWindow = new BrowserWindow({
		width: MINI_WINDOW_WIDTH,
		height: MINI_WINDOW_HEIGHT,
		minWidth: MINI_WINDOW_MIN_WIDTH,
		minHeight: MINI_WINDOW_MIN_HEIGHT,
		resizable: false,
		titleBarStyle: process.platform === 'darwin' ? 'hidden' : 'default',
		...(process.platform === 'darwin'
			? {
				backgroundColor: '#111111',
				trafficLightPosition: { x: 14, y: 14 }
			}
			: {}),
		...(iconPath ? { icon: iconPath } : {}),
		show: false,
		webPreferences: {
			preload: path.join(__dirname, 'preload.cjs'),
			contextIsolation: true,
			nodeIntegration: false,
			allowRunningInsecureContent: false,
			webviewTag: true
		}
	});
	attachExternalNavigationGuards(miniWindow);
	attachMiniWindowShortcuts(miniWindow.webContents);
	positionMiniWindowBottomRight();
	await loadAppRoute(miniWindow, '/mini');
	miniWindow.once('ready-to-show', () => {
		positionMiniWindowBottomRight();
		miniWindow?.show();
		miniWindow?.focus();
	});
	miniWindow.on('hide', () => {
		writeMiniWindowState({ lastActiveAt: Date.now() });
	});
	miniWindow.on('close', (event) => {
		if (!stoppingForQuit && !stoppedForQuit) {
			event.preventDefault();
			hideMiniWindow();
		}
	});
	miniWindow.on('closed', () => {
		miniWindow = null;
	});
	return miniWindow;
}

const FETCH_MODELS_TIMEOUT_MS = 30_000;

function stripTrailingSlashes(url) {
	return String(url || '')
		.trim()
		.replace(/\/+$/, '');
}

// Mirrors comet-sdk providerbase.Endpoint: tolerates base URLs that already end in /v1.
function openAICompatibleEndpoint(rawBaseURL, path) {
	let baseURL = stripTrailingSlashes(rawBaseURL);
	if (!baseURL) throw new Error('Base URL is required');
	baseURL = baseURL.replace(/\/chat\/completions$/i, '');
	const suffix = path.startsWith('/') ? path : `/${path}`;
	if (baseURL.endsWith('/v1')) {
		return `${baseURL}${suffix}`;
	}
	return `${baseURL}/v1${suffix}`;
}

function normalizeModelsBaseURL(rawBaseURL) {
	return openAICompatibleEndpoint(rawBaseURL, '/models');
}

function normalizeAnthropicModelsURL(rawBaseURL) {
	return openAICompatibleEndpoint(rawBaseURL, '/models');
}

async function fetchModelsFromURL(url, headers) {
	try {
		return await fetch(url, {
			headers,
			signal: AbortSignal.timeout(FETCH_MODELS_TIMEOUT_MS)
		});
	} catch (err) {
		if (err?.name === 'TimeoutError' || err?.name === 'AbortError') {
			throw new Error(
				`Timed out after ${FETCH_MODELS_TIMEOUT_MS / 1000}s contacting ${url}. ` +
					'Check the base URL, VPN or network access, and that the provider exposes GET /v1/models.'
			);
		}
		const message = err instanceof Error ? err.message : String(err);
		throw new Error(`Failed to reach ${url}: ${message}`);
	}
}

async function fetchOpenAIModels(baseURL, apiKey) {
	const url = normalizeModelsBaseURL(baseURL);
	const res = await fetchModelsFromURL(url, {
		Authorization: `Bearer ${apiKey}`,
		Accept: 'application/json'
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	const payload = await res.json();
	const rawModels = Array.isArray(payload?.data)
		? payload.data
		: Array.isArray(payload)
			? payload
			: [];
	const result = normalizeModelFetchResult(rawModels);
	if (result.models.length === 0) throw new Error('No models returned by provider');
	return result;
}

async function fetchAnthropicModels(baseURL, apiKey) {
	const url = normalizeAnthropicModelsURL(baseURL);
	const res = await fetchModelsFromURL(url, {
		'x-api-key': apiKey,
		'anthropic-version': '2023-06-01',
		Accept: 'application/json'
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	const payload = await res.json();
	const rawModels = Array.isArray(payload?.data) ? payload.data : [];
	const result = normalizeModelFetchResult(rawModels);
	if (result.models.length === 0) throw new Error('No models returned by Anthropic');
	return result;
}

function codexAuthPath() {
	const codexHome = String(process.env.CODEX_HOME || '').trim();
	return path.join(codexHome || path.join(os.homedir(), '.codex'), 'auth.json');
}

function codexRedirectURI() {
	return `http://localhost:${CODEX_AUTH_CALLBACK_PORT}${CODEX_AUTH_CALLBACK_PATH}`;
}

function base64URLEncode(buffer) {
	return Buffer.from(buffer)
		.toString('base64')
		.replace(/\+/g, '-')
		.replace(/\//g, '_')
		.replace(/=+$/g, '');
}

function codexCodeVerifier() {
	return base64URLEncode(crypto.randomBytes(48));
}

function codexCodeChallenge(verifier) {
	return base64URLEncode(crypto.createHash('sha256').update(verifier).digest());
}

function parseJWTPayload(token) {
	const parts = String(token || '').split('.');
	if (parts.length < 2) return {};
	try {
		return JSON.parse(Buffer.from(parts[1], 'base64url').toString('utf8'));
	} catch {
		return {};
	}
}

function codexAccountIDFromTokens(tokens) {
	if (tokens?.account_id) return tokens.account_id;
	const accessPayload = parseJWTPayload(tokens?.access_token);
	if (typeof accessPayload.account_id === 'string') return accessPayload.account_id;
	const idPayload = parseJWTPayload(tokens?.id_token);
	if (typeof idPayload.account_id === 'string') return idPayload.account_id;
	return '';
}

function writeCodexAuth(tokens) {
	const authPath = codexAuthPath();
	const authDir = path.dirname(authPath);
	fs.mkdirSync(authDir, { recursive: true, mode: 0o700 });
	const auth = {
		auth_mode: 'chatgpt',
		tokens: {
			access_token: tokens.access_token,
			refresh_token: tokens.refresh_token || '',
			id_token: tokens.id_token || '',
			account_id: codexAccountIDFromTokens(tokens),
			last_refresh: new Date().toISOString()
		}
	};
	const tmpPath = `${authPath}.tmp`;
	fs.writeFileSync(tmpPath, `${JSON.stringify(auth, null, 2)}\n`, { mode: 0o600 });
	fs.renameSync(tmpPath, authPath);
	return auth;
}

function jwtExpiresSoon(token) {
	const payload = parseJWTPayload(token);
	const exp = Number(payload.exp || 0);
	if (!exp) return false;
	return exp * 1000 <= Date.now() + 30_000;
}

async function refreshCodexAuth(auth, authPath) {
	const refreshToken = String(auth?.tokens?.refresh_token || '').trim();
	if (!refreshToken) throw new Error('Codex session expired. Sign in with ChatGPT again.');
	const res = await fetch(CODEX_REFRESH_URL, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
		body: JSON.stringify({
			client_id: CODEX_CLIENT_ID,
			grant_type: 'refresh_token',
			refresh_token: refreshToken
		}),
		signal: AbortSignal.timeout(FETCH_MODELS_TIMEOUT_MS)
	});
	const payload = await res.json().catch(() => ({}));
	if (!res.ok || payload.error) {
		const detail = payload.error_description || payload.error || res.statusText;
		throw new Error(`Codex refresh failed: ${detail}. Sign in with ChatGPT again.`);
	}
	if (!payload.access_token) throw new Error('Codex refresh did not return an access token');
	auth.tokens.access_token = payload.access_token;
	if (payload.refresh_token) auth.tokens.refresh_token = payload.refresh_token;
	if (payload.id_token) auth.tokens.id_token = payload.id_token;
	auth.tokens.account_id = codexAccountIDFromTokens(auth.tokens);
	auth.tokens.last_refresh = new Date().toISOString();
	const tmpPath = `${authPath}.tmp`;
	fs.writeFileSync(tmpPath, `${JSON.stringify(auth, null, 2)}\n`, { mode: 0o600 });
	fs.renameSync(tmpPath, authPath);
	return auth;
}

async function borrowCodexAuth() {
	const authPath = codexAuthPath();
	if (!fs.existsSync(authPath)) {
		throw new Error(`Codex auth file not found at ${authPath}. Sign in with ChatGPT first.`);
	}
	let auth;
	try {
		auth = JSON.parse(fs.readFileSync(authPath, 'utf8'));
	} catch (err) {
		throw new Error(
			`Failed to read Codex auth file: ${err instanceof Error ? err.message : err}`
		);
	}
	if (auth?.auth_mode !== 'chatgpt') {
		throw new Error(
			'Codex is not signed in with ChatGPT browser auth. Sign in with ChatGPT first.'
		);
	}
	if (!auth?.tokens?.access_token) {
		throw new Error('Codex auth file has no access token. Sign in with ChatGPT first.');
	}
	if (jwtExpiresSoon(auth.tokens.access_token)) {
		auth = await refreshCodexAuth(auth, authPath);
	}
	return {
		accessToken: auth.tokens.access_token,
		accountID: auth.tokens.account_id || ''
	};
}

function getCodexAuthStatus() {
	const authPath = codexAuthPath();
	if (!fs.existsSync(authPath)) {
		return { authenticated: false, authPath, error: 'Not signed in' };
	}
	try {
		const auth = JSON.parse(fs.readFileSync(authPath, 'utf8'));
		if (auth?.auth_mode !== 'chatgpt') {
			return {
				authenticated: false,
				authPath,
				error: 'Codex is not signed in with ChatGPT browser auth'
			};
		}
		if (!auth?.tokens?.access_token) {
			return { authenticated: false, authPath, error: 'Codex auth file has no access token' };
		}
		return {
			authenticated: true,
			authPath,
			accountID: auth.tokens.account_id || undefined
		};
	} catch (err) {
		return {
			authenticated: false,
			authPath,
			error: err instanceof Error ? err.message : String(err)
		};
	}
}

async function exchangeCodexCode(code, codeVerifier) {
	const res = await fetch(CODEX_REFRESH_URL, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
		body: JSON.stringify({
			client_id: CODEX_CLIENT_ID,
			grant_type: 'authorization_code',
			code,
			redirect_uri: codexRedirectURI(),
			code_verifier: codeVerifier
		}),
		signal: AbortSignal.timeout(FETCH_MODELS_TIMEOUT_MS)
	});
	const payload = await res.json().catch(() => ({}));
	if (!res.ok || payload.error) {
		const detail = payload.error_description || payload.error || res.statusText;
		throw new Error(`ChatGPT sign-in failed: ${detail}`);
	}
	if (!payload.access_token) throw new Error('ChatGPT sign-in did not return an access token');
	return payload;
}

function codexAuthorizeURL(state, codeChallenge) {
	const params = new URLSearchParams({
		response_type: 'code',
		client_id: CODEX_CLIENT_ID,
		redirect_uri: codexRedirectURI(),
		scope: 'openid profile email offline_access',
		code_challenge: codeChallenge,
		code_challenge_method: 'S256',
		id_token_add_organizations: 'true',
		codex_cli_simplified_flow: 'true',
		state,
		originator: 'cometline'
	});
	return `https://auth.openai.com/oauth/authorize?${params.toString()}`;
}

async function startCodexLogin() {
	const state = base64URLEncode(crypto.randomBytes(32));
	const codeVerifier = codexCodeVerifier();
	const codeChallenge = codexCodeChallenge(codeVerifier);
	let server;

	try {
		const code = await new Promise((resolve, reject) => {
			const timeout = setTimeout(() => {
				reject(new Error('Timed out waiting for ChatGPT sign-in to complete.'));
			}, CODEX_AUTH_TIMEOUT_MS);

			server = http.createServer((req, res) => {
				const requestURL = new URL(req.url || '/', codexRedirectURI());
				if (requestURL.pathname !== CODEX_AUTH_CALLBACK_PATH) {
					res.writeHead(404, { 'Content-Type': 'text/plain' });
					res.end('Not found');
					return;
				}

				const error = requestURL.searchParams.get('error');
				const returnedState = requestURL.searchParams.get('state');
				const returnedCode = requestURL.searchParams.get('code');
				if (returnedState !== state) {
					res.writeHead(400, { 'Content-Type': 'text/html' });
					res.end('<h1>ChatGPT sign-in failed</h1><p>Invalid OAuth state.</p>');
					clearTimeout(timeout);
					reject(new Error('ChatGPT sign-in failed: invalid OAuth state.'));
					return;
				}
				if (error) {
					res.writeHead(400, { 'Content-Type': 'text/html' });
					res.end(`<h1>ChatGPT sign-in failed</h1><p>${error}</p>`);
					clearTimeout(timeout);
					reject(new Error(`ChatGPT sign-in failed: ${error}`));
					return;
				}
				if (!returnedCode) {
					res.writeHead(400, { 'Content-Type': 'text/html' });
					res.end(
						'<h1>ChatGPT sign-in failed</h1><p>No authorization code returned.</p>'
					);
					clearTimeout(timeout);
					reject(new Error('ChatGPT sign-in failed: no authorization code returned.'));
					return;
				}

				res.writeHead(200, { 'Content-Type': 'text/html' });
				res.end('<h1>Signed in with ChatGPT</h1><p>You can return to Cometline.</p>');
				clearTimeout(timeout);
				resolve(returnedCode);
			});

			server.once('error', (err) => {
				clearTimeout(timeout);
				reject(err);
			});
			server.listen(CODEX_AUTH_CALLBACK_PORT, async () => {
				try {
					await shell.openExternal(codexAuthorizeURL(state, codeChallenge));
				} catch (err) {
					clearTimeout(timeout);
					reject(
						new Error(
							`Failed to open ChatGPT sign-in in your browser: ${err instanceof Error ? err.message : err}`
						)
					);
				}
			});
		});

		const tokens = await exchangeCodexCode(code, codeVerifier);
		writeCodexAuth(tokens);
		return {
			started: true,
			message: 'Signed in with ChatGPT. You can fetch Codex models now.'
		};
	} finally {
		if (server) server.close();
	}
}

function mcpOAuthTokenPath(serverId) {
	const id = String(serverId || '').trim();
	return path.join(os.homedir(), '.cometmind', 'mcp-oauth', `${id}.json`);
}

function mcpOAuthRedirectURI() {
	return `http://localhost:${MCP_OAUTH_CALLBACK_PORT}${MCP_OAUTH_CALLBACK_PATH}`;
}

function getMcpOAuthStatus(serverId) {
	const authPath = mcpOAuthTokenPath(serverId);
	try {
		if (!fs.existsSync(authPath)) {
			return { authenticated: false, authPath };
		}
		const raw = JSON.parse(fs.readFileSync(authPath, 'utf8'));
		return {
			authenticated: Boolean(raw?.access_token),
			authPath,
			expiry: raw?.expiry || raw?.expires_at || undefined
		};
	} catch (err) {
		return {
			authenticated: false,
			authPath,
			error: err instanceof Error ? err.message : String(err)
		};
	}
}

function readCursorMcpConfig() {
	const filePath = path.join(os.homedir(), '.cursor', 'mcp.json');
	if (!fs.existsSync(filePath)) {
		return {
			ok: false,
			error: 'Cursor MCP config not found at ~/.cursor/mcp.json'
		};
	}
	try {
		const raw = fs.readFileSync(filePath, 'utf8');
		return { ok: true, path: filePath, config: JSON.parse(raw) };
	} catch (err) {
		return {
			ok: false,
			error: err instanceof Error ? err.message : 'Failed to read Cursor MCP config'
		};
	}
}

function writeMcpOAuthToken(serverId, tokens) {
	const authPath = mcpOAuthTokenPath(serverId);
	fs.mkdirSync(path.dirname(authPath), { recursive: true, mode: 0o700 });
	fs.writeFileSync(authPath, JSON.stringify(tokens, null, 2), { mode: 0o600 });
}

async function exchangeMcpOAuthCode(oauth, code, codeVerifier) {
	const body = new URLSearchParams({
		grant_type: 'authorization_code',
		client_id: String(oauth.clientId || '').trim(),
		code,
		redirect_uri: mcpOAuthRedirectURI(),
		code_verifier: codeVerifier
	});
	const res = await fetch(String(oauth.tokenUrl || '').trim(), {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			Accept: 'application/json'
		},
		body
	});
	const payload = await res.json().catch(() => ({}));
	if (!res.ok || payload.error) {
		const detail = payload.error_description || payload.error || res.statusText;
		throw new Error(`MCP OAuth failed: ${detail}`);
	}
	if (!payload.access_token) {
		throw new Error('MCP OAuth did not return an access token');
	}
	if (payload.expires_in && !payload.expiry) {
		payload.expiry = new Date(Date.now() + Number(payload.expires_in) * 1000).toISOString();
	}
	return payload;
}

function mcpAuthorizeURL(oauth, state, codeChallenge) {
	const params = new URLSearchParams({
		response_type: 'code',
		client_id: String(oauth.clientId || '').trim(),
		redirect_uri: mcpOAuthRedirectURI(),
		state,
		code_challenge: codeChallenge,
		code_challenge_method: 'S256'
	});
	const scopes = Array.isArray(oauth.scopes)
		? oauth.scopes.map((scope) => String(scope).trim()).filter(Boolean)
		: [];
	if (scopes.length > 0) params.set('scope', scopes.join(' '));
	const base = String(oauth.authorizationUrl || '').trim();
	return `${base}${base.includes('?') ? '&' : '?'}${params.toString()}`;
}

async function startMcpOAuth({ serverId, oauth }) {
	const id = String(serverId || '').trim();
	if (!id) throw new Error('MCP server id is required');
	if (!oauth?.clientId || !oauth?.authorizationUrl || !oauth?.tokenUrl) {
		throw new Error('OAuth client ID, authorization URL, and token URL are required');
	}

	const state = base64URLEncode(crypto.randomBytes(32));
	const codeVerifier = codexCodeVerifier();
	const codeChallenge = codexCodeChallenge(codeVerifier);
	let server;

	try {
		const code = await new Promise((resolve, reject) => {
			const timeout = setTimeout(() => {
				reject(new Error('Timed out waiting for MCP OAuth to complete.'));
			}, CODEX_AUTH_TIMEOUT_MS);

			server = http.createServer((req, res) => {
				const requestURL = new URL(req.url || '/', mcpOAuthRedirectURI());
				if (requestURL.pathname !== MCP_OAUTH_CALLBACK_PATH) {
					res.writeHead(404, { 'Content-Type': 'text/plain' });
					res.end('Not found');
					return;
				}

				const error = requestURL.searchParams.get('error');
				const returnedState = requestURL.searchParams.get('state');
				const returnedCode = requestURL.searchParams.get('code');
				if (returnedState !== state) {
					res.writeHead(400, { 'Content-Type': 'text/html' });
					res.end('<h1>MCP OAuth failed</h1><p>Invalid OAuth state.</p>');
					clearTimeout(timeout);
					reject(new Error('MCP OAuth failed: invalid OAuth state.'));
					return;
				}
				if (error) {
					res.writeHead(400, { 'Content-Type': 'text/html' });
					res.end(`<h1>MCP OAuth failed</h1><p>${error}</p>`);
					clearTimeout(timeout);
					reject(new Error(`MCP OAuth failed: ${error}`));
					return;
				}
				if (!returnedCode) {
					res.writeHead(400, { 'Content-Type': 'text/html' });
					res.end('<h1>MCP OAuth failed</h1><p>No authorization code returned.</p>');
					clearTimeout(timeout);
					reject(new Error('MCP OAuth failed: no authorization code returned.'));
					return;
				}

				res.writeHead(200, { 'Content-Type': 'text/html' });
				res.end('<h1>MCP connected</h1><p>You can return to Cometline.</p>');
				clearTimeout(timeout);
				resolve(returnedCode);
			});

			server.once('error', (err) => {
				clearTimeout(timeout);
				reject(err);
			});
			server.listen(MCP_OAUTH_CALLBACK_PORT, async () => {
				try {
					await shell.openExternal(mcpAuthorizeURL(oauth, state, codeChallenge));
				} catch (err) {
					clearTimeout(timeout);
					reject(
						new Error(
							`Failed to open MCP OAuth in your browser: ${err instanceof Error ? err.message : err}`
						)
					);
				}
			});
		});

		const tokens = await exchangeMcpOAuthCode(oauth, code, codeVerifier);
		writeMcpOAuthToken(id, tokens);
		return {
			started: true,
			message: `Saved MCP OAuth token for ${id}. Reconnect the server to apply.`
		};
	} finally {
		if (server) server.close();
	}
}

function normalizeModelFetchResult(rawModels, pickModel = (item) => item?.id) {
	const models = [];
	for (const item of rawModels) {
		if (typeof item === 'string') {
			const id = item.trim();
			if (id) models.push(id);
			continue;
		}
		if (!item || typeof item !== 'object') continue;
		const id = String(pickModel(item) || '').trim();
		if (!id) continue;
		models.push(id);
	}
	const uniqueModels = Array.from(new Set(models)).sort();
	return { models: uniqueModels };
}

function normalizeCodexModelsURL(rawBaseURL) {
	const base = String(rawBaseURL || CODEX_BASE_URL).replace(/\/+$/, '');
	return `${base}/models?client_version=${encodeURIComponent(CODEX_CLIENT_VERSION)}`;
}

async function fetchCodexModels(baseURL) {
	const auth = await borrowCodexAuth();
	const headers = {
		Authorization: `Bearer ${auth.accessToken}`,
		Accept: 'application/json'
	};
	if (auth.accountID) headers['ChatGPT-Account-ID'] = auth.accountID;
	const res = await fetchModelsFromURL(normalizeCodexModelsURL(baseURL), headers);
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	const payload = await res.json();
	const rawModels = Array.isArray(payload?.models)
		? payload.models
		: Array.isArray(payload?.data)
			? payload.data
			: [];
	const filtered = rawModels.filter((item) => {
		if (typeof item === 'string') return true;
		if (!item || typeof item !== 'object') return false;
		return item.supported_in_api !== false && item.visibility !== 'hidden';
	});
	const result = normalizeModelFetchResult(filtered, (item) => item.slug || item.id);
	if (result.models.length === 0) throw new Error('No models returned by Codex');
	return result;
}

async function fetchOpenCodeGoModels(baseURL) {
	const url = normalizeModelsBaseURL(baseURL || 'https://opencode.ai/zen/go/v1');
	const res = await fetchModelsFromURL(url, {
		Accept: 'application/json'
	});
	if (!res.ok) {
		const body = await res.text();
		throw new Error(`${res.status}: ${body || res.statusText}`);
	}
	const payload = await res.json();
	const rawModels = Array.isArray(payload?.data)
		? payload.data
		: Array.isArray(payload)
			? payload
			: [];
	const result = normalizeModelFetchResult(rawModels);
	if (result.models.length === 0) throw new Error('No models returned by OpenCode Go');
	return result;
}

async function fetchProviderModels(config) {
	const method = config.method;
	if (method === 'opencode-go') {
		return fetchOpenCodeGoModels(config.baseURL);
	}
	if (method === 'codex') {
		return fetchCodexModels(config.baseURL);
	}

	const baseURL = String(config.baseURL || '').trim();
	const apiKey = String(config.apiKey || '').trim();
	if (!baseURL) throw new Error('Base URL is required');
	if (!apiKey) throw new Error('API key is required');

	if (method === 'anthropic') {
		return fetchAnthropicModels(baseURL, apiKey);
	}
	return fetchOpenAIModels(baseURL, apiKey);
}

app.whenReady().then(async () => {
	if (!hasSingleInstanceLock) {
		return;
	}
	// Serve the packaged bundle over the custom app:// scheme.
	if (app.isPackaged) registerAppProtocol();

	pruneWorkspaceStore();

	if (process.platform === 'darwin') {
		ensureTray();
	}

	// Ensure settings JSON exists with system prompt path before starting CometMind.
	writeProviderSettings(readProviderSettings());
	installCometMindCliShim();
	const startupSettings = readProviderSettings();
	applyOpenAtLoginSetting(startupSettings.app?.openAtLogin);
	refreshGlobalShortcuts();
	startCometMind();
	// Create the window immediately and let the sidecar warm up in parallel.
	// The renderer shows its own connecting/loading state until the health
	// check passes, so we no longer block the window behind waitForHealth().
	const windowReady = createWindow();
	waitForHealth().then((healthy) => {
		if (!healthy) {
			console.error('CometMind failed to become healthy');
			return;
		}
		if (startupSettings.cometmind?.gateway?.discord?.enabled) {
			startDiscordGateway();
		}
	});
	await windowReady;
	applyIconVariant(startupSettings.app?.iconVariant);
	configureAutoUpdater();

	app.on('second-instance', () => {
		showMainWindow();
	});

	app.on('activate', () => {
		// Reopening from the Dock: re-show the warm, hidden window if it still
		// exists (instant); only build a fresh one if it was actually destroyed.
		if (mainWindow && !mainWindow.isDestroyed()) {
			showMainWindow();
		} else if (BrowserWindow.getAllWindows().length === 0) {
			createWindow();
		}
	});
});

app.on('window-all-closed', () => {
	if (process.platform !== 'darwin') {
		app.quit();
	}
});

app.on('before-quit', async (event) => {
	if (stoppedForQuit || stoppingForQuit) return;

	if (relaunchForUpdate) {
		// quitAndInstall() is driving the quit; don't intercept it so the
		// updater can relaunch the app after installation.
		if (updateCheckTimer) {
			clearInterval(updateCheckTimer);
			updateCheckTimer = null;
		}
		return;
	}

	event.preventDefault();
	stoppingForQuit = true;
	destroyTray();
	if (updateCheckTimer) {
		clearInterval(updateCheckTimer);
		updateCheckTimer = null;
	}
	await stopDiscordGateway();
	await stopCometMind();
	globalShortcut.unregisterAll();
	stoppedForQuit = true;
	app.quit();
});

// Last-resort synchronous cleanup. `before-quit` handles graceful shutdown,
// but if the main process exits without it (e.g. an uncaught crash) this best
// effort SIGTERM avoids leaving an orphaned sidecar holding the port and DB
// lock. The Go-side --watch-parent watchdog is the real safety net for hard
// kills where no JS runs at all.
process.on('exit', () => {
	if (cometMindGatewayProcess) {
		try {
			cometMindGatewayProcess.kill('SIGTERM');
		} catch {
			// ignore
		}
	}
	if (cometMindProcess) {
		try {
			cometMindProcess.kill('SIGTERM');
		} catch {
			// ignore
		}
	}
});

ipcMain.on('jobs:notify', (_event, payload) => {
	if (!payload || typeof payload.title !== 'string') return;
	if (!ElectronNotification.isSupported()) return;
	const notification = new ElectronNotification({
		title: payload.title,
		body: typeof payload.body === 'string' ? payload.body : ''
	});
	notification.show();
});

ipcMain.on('cometmind:restart', async () => {
	await stopCometMind();
	startCometMind();
	await waitForHealth();
});

ipcMain.on('cometline:shortcut-capture-active', (_event, active) => {
	shortcutCaptureActive = Boolean(active);
});

ipcMain.on('cometline:session-navigation-suspended', (_event, suspended) => {
	sessionNavigationSuspended = Boolean(suspended);
});

ipcMain.on('cometline:web-panel-open', (_event, open) => {
	webPanelOpen = Boolean(open);
});

ipcMain.on('cometline:set-sidebar-open', (_event, payload) => {
	animateWindowButtons(payload);
});

ipcMain.handle('cometline:get-fullscreen', () =>
	Boolean(mainWindow && !mainWindow.isDestroyed() && mainWindow.isFullScreen())
);

ipcMain.handle('cometline:get-workspace-path', () => getWorkspacePath());

ipcMain.handle('cometline:open-session-in-main-window', (_event, sessionId) =>
	openSessionInMainWindow(sessionId)
);

ipcMain.handle('cometline:select-workspace-path', selectWorkspacePath);

ipcMain.handle('cometline:set-workspace-path', (_event, workspacePath) => {
	const clean = writeStoredWorkspacePath(workspacePath);
	return clean;
});

ipcMain.handle('cometline:list-recent-workspaces', () => listRecentWorkspacePaths());

ipcMain.handle('cometline:remove-recent-workspace-path', (_event, workspacePath) =>
	removeRecentWorkspacePath(workspacePath)
);

ipcMain.handle('cometline:filter-existing-workspace-paths', (_event, paths) =>
	filterExistingWorkspacePaths(Array.isArray(paths) ? paths : [])
);

ipcMain.handle('cometline:prune-workspace-store', () => pruneWorkspaceStore());

ipcMain.handle('cometline:read-workspace-file', (_event, workspacePath, relativePath) =>
	readWorkspaceFileForPreview(workspacePath, relativePath)
);

ipcMain.handle('cometline:get-provider-settings', () => readProviderSettings());

ipcMain.handle('cometline:get-codex-auth-status', () => getCodexAuthStatus());

ipcMain.handle('cometline:start-codex-login', () => startCodexLogin());

ipcMain.handle('cometline:get-mcp-oauth-status', (_event, serverId) => getMcpOAuthStatus(serverId));

ipcMain.handle('cometline:start-mcp-oauth', (_event, payload) => startMcpOAuth(payload));

ipcMain.handle('cometline:read-cursor-mcp-config', () => readCursorMcpConfig());

ipcMain.handle('cometline:fetch-provider-models', async (_event, config) => {
	return fetchProviderModels(config);
});

ipcMain.handle('cometline:save-provider-settings', async (_event, settings, options = {}) => {
	const previous = readProviderSettings();
	const saved = writeProviderSettings(settings);
	const iconVariantChanged =
		(previous.app?.iconVariant ?? 'default') !== (saved.app?.iconVariant ?? 'default');
	const shouldRestartCometMind = options.restartCometMind !== false || iconVariantChanged;
	refreshGlobalShortcuts();
	if (shouldRestartCometMind) {
		await stopCometMind();
		startCometMind();
		await waitForHealth();
	}
	await syncDiscordGatewayFromSettings(saved);
	applyOpenAtLoginSetting(saved.app?.openAtLogin);
	applyIconVariant(saved.app?.iconVariant);
	return saved;
});

ipcMain.handle('cometline:get-mini-window-state', () => readMiniWindowState());

ipcMain.handle('cometline:save-mini-window-state', (_event, state) => writeMiniWindowState(state));

ipcMain.handle('cometline:export-provider-settings', () => exportProviderSettings());

ipcMain.handle('cometline:import-provider-settings', () => importProviderSettings());

ipcMain.handle('cometline:get-discord-gateway-status', () => {
	const settings = readProviderSettings();
	return {
		running: Boolean(cometMindGatewayProcess),
		enabled: Boolean(settings.cometmind?.gateway?.discord?.enabled)
	};
});

ipcMain.handle('cometline:set-discord-gateway-enabled', async (_event, enabled) => {
	const settings = readProviderSettings();
	settings.cometmind.gateway.discord.enabled = Boolean(enabled);
	const saved = writeProviderSettings(settings);
	await syncDiscordGatewayFromSettings(saved);
	return {
		running: Boolean(cometMindGatewayProcess),
		enabled: Boolean(saved.cometmind?.gateway?.discord?.enabled)
	};
});

ipcMain.handle('cometline:get-open-at-login', () => {
	const settings = readProviderSettings();
	try {
		const login = readLoginItemState();
		return {
			openAtLogin: login.openAtLogin,
			status: login.status
		};
	} catch {
		return {
			openAtLogin: Boolean(settings.app?.openAtLogin),
			status: 'unknown'
		};
	}
});

ipcMain.handle('cometline:set-open-at-login', (_event, openAtLogin) => {
	const settings = readProviderSettings();
	settings.app = {
		...settings.app,
		openAtLogin: Boolean(openAtLogin)
	};
	const saved = writeProviderSettings(settings);
	const result = applyOpenAtLoginSetting(saved.app.openAtLogin);
	return {
		openAtLogin: result.openAtLogin ?? saved.app.openAtLogin,
		status: result.status ?? 'unknown',
		needsApproval: Boolean(result.needsApproval),
		openedSettings: Boolean(result.openedSettings),
		isDev: Boolean(result.isDev),
		message: result.message
	};
});

// Opens a markdown link in the user's default browser. Only http(s)/mailto are
// allowed so a malicious link can't launch arbitrary local handlers.
function isExternallyOpenableUrl(rawUrl) {
	try {
		const parsed = new URL(String(rawUrl));
		return (
			parsed.protocol === 'http:' ||
			parsed.protocol === 'https:' ||
			parsed.protocol === 'mailto:'
		);
	} catch {
		return false;
	}
}

async function readWorkspaceFileForPreview(workspacePath, relativePath) {
	const WORKSPACE_FILE_MAX_BYTES = 256 * 1024;
	const IMAGE_MIME_BY_EXT = {
		'.png': 'image/png',
		'.jpg': 'image/jpeg',
		'.jpeg': 'image/jpeg',
		'.gif': 'image/gif',
		'.webp': 'image/webp',
		'.svg': 'image/svg+xml'
	};

	const root = path.resolve(String(workspacePath || ''));
	const clean = String(relativePath || '').replace(/^[/\\]+/, '');
	if (!root || root === path.sep || !clean) {
		return { ok: false, error: 'Invalid file path' };
	}

	const abs = path.resolve(root, clean);
	if (abs !== root && !abs.startsWith(root + path.sep)) {
		return { ok: false, error: 'Path escapes workspace' };
	}

	let stat;
	try {
		stat = await fs.promises.stat(abs);
	} catch {
		return { ok: false, error: 'File not found' };
	}
	if (!stat.isFile()) {
		return { ok: false, error: 'Not a file' };
	}
	if (stat.size > WORKSPACE_FILE_MAX_BYTES) {
		return { ok: false, error: 'File exceeds 256 KB preview limit' };
	}

	const ext = path.extname(abs).toLowerCase();
	const mimeType = IMAGE_MIME_BY_EXT[ext];
	if (mimeType) {
		const buffer = await fs.promises.readFile(abs);
		return {
			ok: true,
			kind: 'image',
			mimeType,
			dataUrl: `data:${mimeType};base64,${buffer.toString('base64')}`
		};
	}

	let content;
	try {
		content = await fs.promises.readFile(abs, 'utf8');
	} catch {
		return { ok: false, error: 'Cannot read file as text' };
	}
	if (content.includes('\0')) {
		return { ok: false, error: 'Binary file cannot be previewed' };
	}

	return { ok: true, kind: 'text', content, extension: ext };
}

ipcMain.handle('cometline:open-external', async (_event, rawUrl) => {
	if (!isExternallyOpenableUrl(rawUrl)) return false;
	await shell.openExternal(String(rawUrl));
	return true;
});

ipcMain.handle('cometline:get-app-version', () => app.getVersion());

ipcMain.handle('cometline:get-update-state', () => updateState);

ipcMain.handle('cometline:check-for-updates', async () => {
	if (!app.isPackaged) return { status: 'idle' };
	try {
		await getAutoUpdater().checkForUpdates();
	} catch (err) {
		console.error('Manual update check failed:', err);
		setUpdateState({ status: 'error', message: String(err?.message ?? err) });
	}
	return updateState;
});

ipcMain.handle('cometline:install-update', async () => {
	if (updateState.status !== 'ready') return false;
	relaunchForUpdate = true;
	stoppingForQuit = true;
	// Stop the sidecar gracefully before the updater takes over the quit flow.
	await stopDiscordGateway();
	await stopCometMind();
	// isSilent=true, isForceRunAfter=true so the updater relaunches the app.
	setImmediate(() => getAutoUpdater().quitAndInstall(true, true));
	return true;
});
