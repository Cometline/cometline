/**
 * Rich URL embed chips. A bare URL in a message is rendered as a compact inline
 * chip: a favicon (from a public favicon CDN) plus a label (the site title when
 * known, otherwise the domain). No page scraping is performed here — only the
 * domain and a CDN favicon URL are derived from the link itself.
 */

/** Escapes text for safe inclusion in an HTML attribute or text node. */
function escapeHtml(value: string): string {
	return value
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

/** True when the URL uses an http(s) scheme (the only schemes we chip + link). */
export function isHttpUrl(url: string): boolean {
	try {
		const parsed = new URL(url);
		return parsed.protocol === 'http:' || parsed.protocol === 'https:';
	} catch {
		return false;
	}
}

/** Returns the hostname without a leading `www.`, or the raw input on failure. */
export function domainFromUrl(url: string): string {
	try {
		const host = new URL(url).hostname;
		return host.replace(/^www\./i, '');
	} catch {
		return url;
	}
}

/**
 * Favicon CDN URL for a link's domain. DuckDuckGo's icon proxy is fast, has no
 * rate limit, and returns a correct content-type. Swap this constant to switch
 * providers (e.g. Google's `s2/favicons`).
 */
export function faviconUrl(url: string): string {
	const domain = domainFromUrl(url);
	return `https://icons.duckduckgo.com/ip3/${encodeURIComponent(domain)}.ico`;
}

/**
 * Builds the inline embed chip HTML for a URL. The output is sanitizer-friendly:
 * the `href` is only emitted for http(s) URLs, and `data-embed-url` lets the
 * renderer route clicks through the app's external-link handler (the same path
 * DOMPurify tags with `data-external-link`). All dynamic values are escaped.
 */
export function buildEmbedChip(url: string, label?: string): string {
	const safe = isHttpUrl(url);
	const text = (label?.trim() || domainFromUrl(url)) ?? url;
	const escapedLabel = escapeHtml(text);
	const escapedUrl = escapeHtml(url);
	const icon = escapeHtml(faviconUrl(url));
	const hrefAttr = safe ? ` href="${escapedUrl}"` : '';

	return (
		`<a${hrefAttr} class="link-embed" data-embed-url="${escapedUrl}" title="${escapedUrl}">` +
		`<img class="link-embed-icon" src="${icon}" alt="" width="14" height="14" loading="lazy" />` +
		`<span class="link-embed-label">${escapedLabel}</span>` +
		`</a>`
	);
}

/** Matches bare http(s) URLs in free text. */
const BARE_URL_GLOBAL = /https?:\/\/[^\s<]+/g;

/** Trailing punctuation that should not be captured as part of a URL. */
const URL_TRAILING_PUNCTUATION = /[.,;:!?)\]}'"]+$/;

/**
 * Extracts unique http(s) URLs from free text, trimming trailing sentence
 * punctuation. Order is preserved and duplicates are removed. Used by the
 * composer to show live link-preview chips as the user types.
 */
export function extractUrls(text: string): string[] {
	if (!text) return [];
	const seen = new Set<string>();
	const out: string[] = [];
	for (const match of text.matchAll(BARE_URL_GLOBAL)) {
		let url = match[0];
		const trailing = URL_TRAILING_PUNCTUATION.exec(url);
		if (trailing) url = url.slice(0, url.length - trailing[0].length);
		if (!url || !isHttpUrl(url) || seen.has(url)) continue;
		seen.add(url);
		out.push(url);
	}
	return out;
}
