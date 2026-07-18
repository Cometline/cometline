const WIKI_FILE_EXT = /\.(md|html|htm)$/i;

/** Basename stem without `.md` / `.html` extension. */
export function wikiStemFromPath(path: string): string {
	const normalized = path.trim().replace(/\\/g, '/');
	const base = normalized.split('/').pop() ?? normalized;
	return base.replace(WIKI_FILE_EXT, '');
}

function stripWikiExt(value: string): string {
	return value.replace(WIKI_FILE_EXT, '');
}

export type ParsedWikilink = {
	target: string;
	alias?: string;
};

/** Parses the inside of `[[...]]` (without brackets): `Page` or `Page|alias`. */
export function parseWikilinkInner(inner: string): ParsedWikilink | null {
	const trimmed = inner.trim();
	if (!trimmed) return null;
	const pipe = trimmed.indexOf('|');
	const hash = trimmed.indexOf('#');
	let targetPart = trimmed;
	let alias: string | undefined;
	if (pipe >= 0) {
		targetPart = trimmed.slice(0, pipe);
		alias = trimmed.slice(pipe + 1).trim() || undefined;
	}
	if (hash >= 0 && (pipe < 0 || hash < pipe)) {
		targetPart = targetPart.slice(0, hash);
	}
	const target = targetPart.trim();
	if (!target) return null;
	return alias ? { target, alias } : { target };
}

function pathRank(path: string): number {
	const p = path.replace(/\\/g, '/').toLowerCase();
	if (p.startsWith('entities/')) return 0;
	if (p.startsWith('concepts/')) return 1;
	if (p.startsWith('syntheses/')) return 2;
	return 3;
}

/**
 * Resolves a wikilink target to a wiki-root-relative `.md` path.
 * Prefers exact path match, then unique basename stem (case-insensitive).
 */
export function resolveWikilink(target: string, files: readonly string[]): string | null {
	const raw = target.trim().replace(/\\/g, '/');
	if (!raw || files.length === 0) return null;

	const cleaned = stripWikiExt(raw);
	const originalExt = raw.match(WIKI_FILE_EXT)?.[0]?.toLowerCase() ?? '';
	const candidates = [
		raw,
		cleaned,
		`${cleaned}.md`,
		`${cleaned}.html`,
		`${cleaned}.htm`,
		...(originalExt ? [`${cleaned}${originalExt}`] : [])
	];
	for (const candidate of candidates) {
		const exact = files.find((f) => f.replace(/\\/g, '/').toLowerCase() === candidate.toLowerCase());
		if (exact) return exact.replace(/\\/g, '/');
	}

	const needle = (cleaned.split('/').pop() ?? cleaned).toLowerCase();
	const matches = files.filter((f) => wikiStemFromPath(f).toLowerCase() === needle);
	if (matches.length === 0) return null;
	if (matches.length === 1) return matches[0]!.replace(/\\/g, '/');

	return [...matches]
		.map((f) => f.replace(/\\/g, '/'))
		.sort((a, b) => pathRank(a) - pathRank(b) || a.localeCompare(b))[0]!;
}
