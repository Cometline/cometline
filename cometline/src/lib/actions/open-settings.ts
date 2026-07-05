import { shellStore } from '$lib/stores/shell.svelte';

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
