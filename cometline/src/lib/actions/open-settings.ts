import { shellStore } from '#lib/stores/shell.svelte.js';

export function openSettings() {
	const openWindow = window.electronAPI?.openSettingsWindow;
	if (!openWindow) {
		shellStore.openSettings();
		return;
	}

	void openWindow().catch(() => {
		shellStore.openSettings();
	});
}
