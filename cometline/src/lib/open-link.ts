import { openExternalLink } from '$lib/external-link';
import { shellStore } from '$lib/stores/shell.svelte';

export { isWebPanelUrl, normalizeUserUrl } from '$lib/web-panel-url';

/** Opens http(s) links in the in-app web panel; mailto and dev fallback stay external. */
export function openLink(rawUrl: string): void {
	if (!rawUrl) return;
	try {
		const parsed = new URL(String(rawUrl));
		if (parsed.protocol === 'mailto:') {
			openExternalLink(rawUrl);
			return;
		}
		if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
			if (window.electronAPI?.setWebPanelOpen) {
				shellStore.openWebPanelForActive(String(rawUrl));
				return;
			}
			openExternalLink(rawUrl);
		}
	} catch {
		// Ignore malformed URLs.
	}
}
