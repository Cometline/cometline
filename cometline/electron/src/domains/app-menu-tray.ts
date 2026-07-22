import { app, Menu, Tray } from 'electron';
import type { KeyboardShortcuts } from '../../../src/lib/keyboard-shortcuts.js';
import type { MenuItemConstructorOptions, NativeImage } from 'electron';
import { shortcutBindingToAccelerator } from './shortcuts.js';
import type { ShellWindowContext } from './runtime-context.js';

interface ApplicationMenuTrayDependencies {
	context: ShellWindowContext;
	readShortcuts(): KeyboardShortcuts;
	getPersonaId(): string;
	builtinPersonaToIconVariant(personaId: string): 'default' | 'man';
	resolveTrayResourcePath(filename: string): string;
	resolveTrayIcon(variant: 'default' | 'man'): NativeImage | null;
	pathExists(filePath: string): boolean;
	showMainWindow(): void;
	showSettingsWindow(): Promise<void>;
}

/** Owns the macOS tray and application menu while the runtime owns window visibility. */
export function createApplicationMenuTray(dependencies: ApplicationMenuTrayDependencies) {
	const {
		context,
		readShortcuts,
		getPersonaId,
		builtinPersonaToIconVariant,
		resolveTrayResourcePath,
		resolveTrayIcon,
		pathExists,
		showMainWindow,
		showSettingsWindow
	} = dependencies;

	function ensureTray() {
		if (process.platform !== 'darwin') return false;
		if (context.getTray()) return true;

		const variant = builtinPersonaToIconVariant(getPersonaId());
		const trayIconPath = resolveTrayResourcePath(
			variant === 'man' ? 'trayIcon_man.png' : 'trayIcon.png'
		);
		const icon = resolveTrayIcon(variant);
		if (!icon || icon.isEmpty()) {
			console.warn('[tray] Failed to create menu bar icon');
			return false;
		}

		const trayImageSource = pathExists(trayIconPath) ? trayIconPath : icon;
		const tray = new Tray(trayImageSource);
		context.setTray(tray);
		(globalThis as typeof globalThis & { __cometlineTray?: Tray }).__cometlineTray = tray;
		tray.setToolTip('Cometline');
		tray.setContextMenu(
			Menu.buildFromTemplate([
				{ label: 'Show Cometline', click: () => showMainWindow() },
				{ label: 'Settings...', click: () => void showSettingsWindow() },
				{ type: 'separator' },
				{ label: 'Quit Cometline', click: () => app.quit() }
			])
		);
		tray.on('click', () => context.getTray()?.popUpContextMenu());

		setTimeout(() => {
			const currentTray = context.getTray();
			if (currentTray) currentTray.setImage(trayImageSource);
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
		const tray = context.getTray();
		if (!tray) return;
		tray.destroy();
		context.setTray(null);
	}

	function configureApplicationMenu() {
		const settingsAccelerator = shortcutBindingToAccelerator(readShortcuts().openSettings);
		const settingsItem: MenuItemConstructorOptions = {
			label: 'Settings...',
			...(settingsAccelerator ? { accelerator: settingsAccelerator } : {}),
			click: () => void showSettingsWindow()
		};
		const macApplicationSubmenu: MenuItemConstructorOptions[] = [
			settingsItem,
			{ type: 'separator' },
			{ role: 'services' },
			{ type: 'separator' },
			{ role: 'hide' },
			{ role: 'hideOthers' },
			{ role: 'unhide' },
			{ type: 'separator' },
			{ role: 'quit' }
		];
		const nonMacFileItems: MenuItemConstructorOptions[] = [settingsItem, { type: 'separator' }];
		const fileSubmenu: MenuItemConstructorOptions[] = [
			...(process.platform === 'darwin' ? [] : nonMacFileItems),
			{ role: 'close' }
		];
		const editSubmenu: MenuItemConstructorOptions[] = [
			{ role: 'undo' },
			{ role: 'redo' },
			{ type: 'separator' },
			{ role: 'cut' },
			{ role: 'copy' },
			{ role: 'paste' },
			{ role: 'selectAll' }
		];
		const viewSubmenu: MenuItemConstructorOptions[] = [
			{ role: 'reload' },
			...(!app.isPackaged ? [{ role: 'toggleDevTools' as const }] : []),
			{ type: 'separator' },
			{ role: 'resetZoom' },
			{ role: 'zoomIn' },
			{ role: 'zoomOut' },
			{ type: 'separator' },
			{ role: 'togglefullscreen' }
		];
		const template: MenuItemConstructorOptions[] = [
			...(process.platform === 'darwin'
				? [{ label: app.name, submenu: macApplicationSubmenu }]
				: []),
			{ label: 'File', submenu: fileSubmenu },
			{ label: 'Edit', submenu: editSubmenu },
			{ label: 'View', submenu: viewSubmenu },
			{
				label: 'Window',
				submenu: [{ role: 'minimize' }, { role: 'zoom' }]
			}
		];
		Menu.setApplicationMenu(Menu.buildFromTemplate(template));
	}

	return { configureApplicationMenu, destroyTray, ensureTray };
}
