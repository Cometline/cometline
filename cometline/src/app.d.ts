import type { ElectronAPI } from '$lib/electron-api';

declare global {
	interface Window {
		electronAPI?: ElectronAPI;
	}
}

export {};
