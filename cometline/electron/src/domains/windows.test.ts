import path from 'node:path';
import type {
	App,
	BrowserWindow,
	BrowserWindowConstructorOptions,
	Screen,
	Shell,
	Tray
} from 'electron';
import { describe, expect, it, vi } from 'vitest';

import { createWindows } from './windows.js';

type EventHandler = (...args: unknown[]) => void;

class FakeWindow {
	static instances: FakeWindow[] = [];
	readonly handlers = new Map<string, EventHandler>();
	readonly webContents = {
		on: vi.fn((event: string, handler: EventHandler) =>
			this.handlers.set(`web:${event}`, handler)
		),
		setWindowOpenHandler: vi.fn(),
		send: vi.fn()
	};
	readonly focus = vi.fn();
	readonly hide = vi.fn();
	readonly isDestroyed = vi.fn(() => false);
	readonly isFocused = vi.fn(() => false);
	readonly isFullScreen = vi.fn(() => false);
	readonly isMinimized = vi.fn(() => false);
	readonly isVisible = vi.fn(() => false);
	readonly loadURL = vi.fn(async () => undefined);
	readonly once = vi.fn((event: string, handler: EventHandler) =>
		this.handlers.set(`once:${event}`, handler)
	);
	readonly on = vi.fn((event: string, handler: EventHandler) =>
		this.handlers.set(event, handler)
	);
	readonly restore = vi.fn();
	readonly setAlwaysOnTop = vi.fn();
	readonly setFullScreen = vi.fn();
	readonly setFullScreenable = vi.fn();
	readonly setPosition = vi.fn();
	readonly setBounds = vi.fn();
	readonly setVisibleOnAllWorkspaces = vi.fn();
	readonly setWindowButtonVisibility = vi.fn();
	readonly show = vi.fn();

	constructor(readonly options: BrowserWindowConstructorOptions) {
		FakeWindow.instances.push(this);
	}

	getBounds() {
		return { x: 0, y: 0, width: 460, height: 640 };
	}
}

function createController(options: { packaged?: boolean; platform?: NodeJS.Platform } = {}) {
	FakeWindow.instances = [];
	const openExternal = vi.fn(async () => undefined);
	const setLoginItemSettings = vi.fn();
	const getLoginItemSettings = vi.fn(() => ({ openAtLogin: true, status: 'not-registered' }));
	const controller = createWindows({
		app: {
			dock: { setIcon: vi.fn() },
			getLoginItemSettings,
			isPackaged: options.packaged ?? true,
			setLoginItemSettings
		} as unknown as Pick<
			App,
			'dock' | 'getLoginItemSettings' | 'isPackaged' | 'setLoginItemSettings'
		>,
		BrowserWindow: FakeWindow as unknown as new (
			options: BrowserWindowConstructorOptions
		) => BrowserWindow,
		ensureTray: vi.fn(() => true),
		getAppIconImage: vi.fn(() => null),
		getAllWindows: () => FakeWindow.instances as unknown as BrowserWindow[],
		getPersonaId: vi.fn(() => 'minako'),
		getTray: () => ({ setToolTip: vi.fn() }) as unknown as Tray,
		os: { release: () => '22.0.0' },
		path,
		platform: options.platform ?? 'darwin',
		runtimeDirectory: '/runtime',
		screen: {
			getCursorScreenPoint: () => ({ x: 100, y: 100 }),
			getDisplayNearestPoint: () => ({ workArea: { x: 0, y: 0, width: 1440, height: 900 } })
		} as unknown as Pick<Screen, 'getCursorScreenPoint' | 'getDisplayNearestPoint'>,
		shell: { openExternal } as Pick<Shell, 'openExternal'>,
		shouldKeepWindowsAlive: () => true,
		shortcuts: {
			attachMainWindowShortcuts: vi.fn(),
			attachMiniWindowShortcuts: vi.fn(),
			attachSettingsWindowShortcuts: vi.fn(),
			attachWebviewPanelShortcuts: vi.fn()
		},
		windowChrome: {
			clearWindowButtonAnimation: vi.fn(),
			sendFullScreenState: vi.fn(),
			setWindowButtonPosition: vi.fn()
		},
		writeMiniWindowState: vi.fn()
	});
	return { controller, getLoginItemSettings, openExternal, setLoginItemSettings };
}

describe('window lifecycle factory', () => {
	it('creates isolated main, mini, and settings windows with their established presentation', async () => {
		const { controller } = createController();
		await controller.createMainWindow();
		await controller.toggleMiniWindow();
		await controller.showSettingsWindow();

		const [main, mini, settings] = FakeWindow.instances;
		expect(main.options).toMatchObject({
			titleBarStyle: 'hidden',
			transparent: true,
			minWidth: 480,
			minHeight: 620,
			webPreferences: {
				preload: '/runtime/dist/preload.cjs',
				contextIsolation: true,
				nodeIntegration: false,
				sandbox: true,
				webviewTag: true,
				devTools: false
			}
		});
		expect(main.loadURL).toHaveBeenCalledWith('app://bundle/');
		expect(mini.options).toMatchObject({ type: 'panel', fullscreenable: false, width: 480, height: 668 });
		expect(mini.setVisibleOnAllWorkspaces).toHaveBeenCalledWith(true, {
			visibleOnFullScreen: true,
			skipTransformProcessType: true
		});
		expect(settings.options.webPreferences?.webviewTag).toBe(false);
		expect(settings.loadURL).toHaveBeenCalledWith('app://bundle/settings');
	});

	it('uses the macOS main-app login item and opens System Settings for approval', () => {
		const { controller, openExternal, setLoginItemSettings } = createController();
		expect(controller.applyOpenAtLoginSetting(true)).toMatchObject({
			openAtLogin: true,
			status: 'not-registered',
			openedSettings: true
		});
		expect(setLoginItemSettings).toHaveBeenCalledWith({
			openAtLogin: true,
			type: 'mainAppService'
		});
		expect(openExternal).toHaveBeenCalledWith(
			'x-apple.systempreferences:com.apple.LoginItems-Settings.extension'
		);
	});
});
