import type { ElectronAPI } from '#lib/electron-api.js';

declare global {
	interface Window {
		electronAPI?: ElectronAPI;
	}
}

export {};
