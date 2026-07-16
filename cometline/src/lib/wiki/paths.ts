export const WIKI_RUNTIME_PREFIX = '@runtime/wiki/';

export function isWikiUiPath(path: string): boolean {
	return path.trim().startsWith(WIKI_RUNTIME_PREFIX);
}

export function toWikiRelative(path: string): string {
	const trimmed = path.trim();
	if (!isWikiUiPath(trimmed)) return trimmed;
	return trimmed.slice(WIKI_RUNTIME_PREFIX.length);
}

export function toWikiUiPath(relativePath: string): string {
	const trimmed = relativePath.trim().replace(/^\/+/, '');
	if (!trimmed) return WIKI_RUNTIME_PREFIX;
	if (isWikiUiPath(trimmed)) return trimmed;
	return `${WIKI_RUNTIME_PREFIX}${trimmed}`;
}

/** True when the wiki page must not be edited from the web panel. */
export function isWikiReadOnlyPath(path: string): boolean {
	const rel = toWikiRelative(path).replace(/\\/g, '/');
	if (!rel) return true;
	if (rel.toLowerCase() === 'wiki.md') return true;
	return rel === 'raw' || rel.startsWith('raw/');
}
