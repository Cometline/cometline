/**
 * Opens an external URL outside the app.
 *
 * In the Electron app, this routes through the validated `openExternal` IPC
 * bridge so the OS browser handles it. In a plain browser (e.g. `vite dev`),
 * `window.electronAPI` is undefined, so we fall back to `window.open` with a
 * blank target — which works during development.
 */
export function openExternalLink(url: string): void {
	if (!url) return;
	const api = window.electronAPI?.openExternal;
	if (api) {
		void api(url);
		return;
	}
	window.open(url, '_blank', 'noopener,noreferrer');
}
