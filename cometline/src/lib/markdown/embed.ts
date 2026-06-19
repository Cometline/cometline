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

/** Workspace-relative file mention in user text (not email addresses). */
const FILE_MENTION_GLOBAL = /(?<![A-Za-z0-9_])@([A-Za-z0-9_][A-Za-z0-9_./-]*)/g;

/** Slash skill command in user text. */
const SKILL_MENTION_GLOBAL = /(^|\s)\/([\w-]+)(?=\s|$|[.,;:!?)\]}'"])/g;

/** Returns the final path segment for compact file chip labels. */
export function fileLabelFromPath(relativePath: string): string {
	const parts = relativePath.split(/[/\\]/).filter(Boolean);
	return parts[parts.length - 1] || relativePath;
}

/**
 * Builds a clickable file embed chip for user messages. Uses `data-file-path` so
 * the renderer can open the side-panel preview on click.
 */
export function buildFileEmbedChip(relativePath: string): string {
	const escapedPath = escapeHtml(relativePath);
	const label = escapeHtml(fileLabelFromPath(relativePath));
	return (
		`<span class="file-embed" role="button" tabindex="0" data-file-path="${escapedPath}" title="${escapedPath}">` +
		`<span class="file-embed-label">@${label}</span>` +
		`</span>`
	);
}

/** Visual-only skill chip for user messages (mirrors composer skill chips). */
export function buildSkillEmbedChip(skillName: string): string {
	const escaped = escapeHtml(skillName);
	return (
		`<span class="skill-embed" data-skill-name="${escaped}" title="/${escaped} skill">` +
		`<span class="skill-embed-label">/${escaped}</span>` +
		`</span>`
	);
}

export type UserTextTokenMatch =
	| { index: number; length: number; type: 'url'; url: string; urlSuffix: string }
	| { index: number; length: number; type: 'file'; path: string }
	| { index: number; length: number; type: 'skill'; name: string; leading: string };

/** Finds the next URL, @file, or /skill token at or after `from`. */
export function findNextUserTextToken(source: string, from: number): UserTextTokenMatch | null {
	let best: UserTextTokenMatch | null = null;

	const consider = (candidate: UserTextTokenMatch) => {
		if (candidate.index < from) return;
		if (!best || candidate.index < best.index) best = candidate;
	};

	const urlRe = /https?:\/\/[^\s<]+/g;
	urlRe.lastIndex = from;
	const urlMatch = urlRe.exec(source);
	if (urlMatch) {
		let url = urlMatch[0];
		const trailing = URL_TRAILING_PUNCTUATION.exec(url);
		const urlSuffix = trailing ? trailing[0] : '';
		if (urlSuffix) url = url.slice(0, url.length - urlSuffix.length);
		if (url && isHttpUrl(url)) {
			consider({
				index: urlMatch.index,
				length: urlMatch[0].length,
				type: 'url',
				url,
				urlSuffix
			});
		}
	}

	FILE_MENTION_GLOBAL.lastIndex = from;
	const fileMatch = FILE_MENTION_GLOBAL.exec(source);
	if (fileMatch) {
		consider({
			index: fileMatch.index,
			length: fileMatch[0].length,
			type: 'file',
			path: fileMatch[1]
		});
	}

	SKILL_MENTION_GLOBAL.lastIndex = from;
	const skillMatch = SKILL_MENTION_GLOBAL.exec(source);
	if (skillMatch) {
		consider({
			index: skillMatch.index,
			length: skillMatch[0].length,
			type: 'skill',
			name: skillMatch[2],
			leading: skillMatch[1]
		});
	}

	return best;
}

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
