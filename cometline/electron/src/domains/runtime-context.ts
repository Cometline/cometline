import type { BrowserWindow, Tray } from 'electron';

/** Shared process state exposed to isolated runtime domains without reverse imports. */
export interface RuntimeContext {
	getWindows(): Array<BrowserWindow | null>;
	shouldSuppressSidecarRespawn(): boolean;
}

/** Mutable shell state injected into UI coordination domains by the runtime composition root. */
export interface ShellWindowContext extends RuntimeContext {
	getMainWindow(): BrowserWindow | null;
	getMiniWindow(): BrowserWindow | null;
	getSettingsWindow(): BrowserWindow | null;
	getTray(): Tray | null;
	setTray(tray: Tray | null): void;
	getShortcutCaptureActive(): boolean;
	setShortcutCaptureActive(active: boolean): void;
	getSessionNavigationSuspended(): boolean;
	setSessionNavigationSuspended(suspended: boolean): void;
	getWebPanelOpen(): boolean;
	setWebPanelOpen(open: boolean): void;
	getInboxOpen(): boolean;
	setInboxOpen(open: boolean): void;
}
