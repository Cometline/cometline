import type { FocusedPane } from '$lib/stores/shell.svelte';

/** Cmd+[ / Cmd+] use webview history when the web panel owns focus. */
export function shouldUseWebPanelHistory(
	webPanelOpen: boolean,
	focusedPane: FocusedPane
): boolean {
	return webPanelOpen && focusedPane === 'web';
}
