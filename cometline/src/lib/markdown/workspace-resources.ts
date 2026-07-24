import { toWikiUiPath } from '$lib/wiki/paths';

/** Context for resolving local image/link paths inside a workspace or wiki markdown file. */
export type WorkspaceMarkdownResources = {
	kind: 'workspace' | 'wiki';
	workspacePath: string;
	/** Workspace- or wiki-relative path of the open markdown file. */
	filePath: string;
	/** Loads a relative file for image hydration (injected by the preview UI). */
	readFile: (relativePath: string) => Promise<{ kind: string; data_url?: string }>;
};

/** Posix dirname; empty string for a root-level file. */
export function dirnamePosix(path: string): string {
	const normalized = path.replace(/\\/g, '/').replace(/\/+$/, '');
	const idx = normalized.lastIndexOf('/');
	if (idx < 0) return '';
	return normalized.slice(0, idx);
}

/**
 * Joins path segments with posix rules. Returns null when the result would
 * escape above the workspace/wiki root via `..`.
 */
export function joinPosixSafe(...parts: string[]): string | null {
	const segments: string[] = [];
	for (const part of parts) {
		if (!part) continue;
		for (const seg of part.replace(/\\/g, '/').split('/')) {
			if (!seg || seg === '.') continue;
			if (seg === '..') {
				if (segments.length === 0) return null;
				segments.pop();
				continue;
			}
			segments.push(seg);
		}
	}
	return segments.join('/');
}

/**
 * Resolves a markdown `src`/`href` against the open file. Returns a workspace-
 * or wiki-relative path, or null for remote/special URLs and in-page anchors.
 */
export function resolveWorkspaceMarkdownPath(
	rawUrl: string,
	markdownFilePath: string
): string | null {
	const trimmed = rawUrl.trim();
	if (!trimmed) return null;
	if (/^(https?:|mailto:|data:|blob:|file:)/i.test(trimmed)) return null;
	if (trimmed.startsWith('#')) return null;

	let pathPart = trimmed;
	const hashIdx = trimmed.indexOf('#');
	if (hashIdx >= 0) pathPart = trimmed.slice(0, hashIdx);
	if (!pathPart) return null;

	let decoded = pathPart;
	try {
		decoded = decodeURIComponent(pathPart);
	} catch {
		// Keep the raw path segment when decoding fails.
	}

	if (/^[a-zA-Z]:/.test(decoded) || decoded.startsWith('//')) return null;

	const mdPath = markdownFilePath.replace(/\\/g, '/').replace(/^\.\//, '');
	if (decoded.startsWith('/')) {
		return joinPosixSafe(decoded.replace(/^\/+/, ''));
	}
	return joinPosixSafe(dirnamePosix(mdPath), decoded);
}

/** Rewrites local `<img src>` / `<a href>` to workspace data attributes before sanitize. */
export function rewriteLocalResourcesInHtml(
	html: string,
	markdownFilePath: string,
	kind: WorkspaceMarkdownResources['kind']
): string {
	if (!html || typeof DOMParser === 'undefined') return html;

	const doc = new DOMParser().parseFromString(`<div id="__md_root">${html}</div>`, 'text/html');
	const root = doc.getElementById('__md_root');
	if (!root) return html;

	for (const img of root.querySelectorAll('img[src]')) {
		const src = img.getAttribute('src');
		if (!src) continue;
		const resolved = resolveWorkspaceMarkdownPath(src, markdownFilePath);
		if (!resolved) continue;
		img.setAttribute('src', '');
		img.setAttribute('data-workspace-src', resolved);
		if (!img.getAttribute('alt')) img.setAttribute('alt', resolved);
	}

	for (const anchor of root.querySelectorAll('a[href]')) {
		const href = anchor.getAttribute('href');
		if (!href) continue;
		const resolved = resolveWorkspaceMarkdownPath(href, markdownFilePath);
		if (!resolved) continue;
		const openPath = kind === 'wiki' ? toWikiUiPath(resolved) : resolved;
		anchor.removeAttribute('href');
		anchor.setAttribute('data-file-path', openPath);
		anchor.setAttribute('role', 'link');
		anchor.setAttribute('tabindex', '0');
		anchor.classList.add('md-workspace-link');
		if (!anchor.getAttribute('title')) anchor.setAttribute('title', openPath);
	}

	return root.innerHTML;
}

/** Loads `data-workspace-src` images via the content API and sets `src` to data URLs. */
export async function hydrateWorkspaceMarkdownImages(
	html: string,
	readFile: WorkspaceMarkdownResources['readFile']
): Promise<string> {
	if (!html.includes('data-workspace-src') || typeof DOMParser === 'undefined') return html;

	const doc = new DOMParser().parseFromString(`<div id="__md_root">${html}</div>`, 'text/html');
	const root = doc.getElementById('__md_root');
	if (!root) return html;

	const imgs = [...root.querySelectorAll('img[data-workspace-src]')];
	const uniquePaths = [
		...new Set(
			imgs
				.map((img) => img.getAttribute('data-workspace-src'))
				.filter((path): path is string => Boolean(path))
		)
	];

	const cache = new Map<string, string>();
	await Promise.all(
		uniquePaths.map(async (path) => {
			try {
				const content = await readFile(path);
				if (content.kind === 'image' && content.data_url) {
					cache.set(path, content.data_url);
				}
			} catch {
				// Leave the image placeholder broken when the file is missing.
			}
		})
	);

	for (const img of imgs) {
		const path = img.getAttribute('data-workspace-src');
		if (!path) continue;
		const dataUrl = cache.get(path);
		if (dataUrl) {
			img.setAttribute('src', dataUrl);
			img.removeAttribute('data-workspace-src');
		} else {
			img.setAttribute('title', `Missing: ${path}`);
		}
	}

	return root.innerHTML;
}
