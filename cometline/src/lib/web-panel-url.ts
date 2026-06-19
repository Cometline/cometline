export function isWebPanelUrl(rawUrl: string): boolean {
	try {
		const parsed = new URL(String(rawUrl));
		return parsed.protocol === 'http:' || parsed.protocol === 'https:';
	} catch {
		return false;
	}
}

function buildSearchUrl(query: string): string {
	return `https://www.google.com/search?q=${encodeURIComponent(query)}`;
}

function looksLikeNavigableHost(hostname: string): boolean {
	if (!hostname) return false;
	if (hostname === 'localhost') return true;
	if (/^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)) return true;
	return hostname.includes('.');
}

/** Normalizes user-typed URLs for the web panel address bar. */
export function normalizeUserUrl(input: string): string | null {
	const trimmed = input.trim();
	if (!trimmed) return null;

	if (/\s/.test(trimmed)) {
		return buildSearchUrl(trimmed);
	}

	const withProtocol = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;

	try {
		const parsed = new URL(withProtocol);
		if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
		if (!looksLikeNavigableHost(parsed.hostname)) {
			return buildSearchUrl(trimmed);
		}
		return parsed.href;
	} catch {
		return buildSearchUrl(trimmed);
	}
}
