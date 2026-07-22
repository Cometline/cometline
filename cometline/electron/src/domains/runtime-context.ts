import type { BrowserWindow } from 'electron';

/** Shared process state exposed to isolated runtime domains without reverse imports. */
export interface RuntimeContext {
	getWindows(): Array<BrowserWindow | null>;
	shouldSuppressSidecarRespawn(): boolean;
}
