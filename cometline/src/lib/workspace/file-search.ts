import { listWikiFiles } from '$lib/client/cometmind';
import type { FileSearchSource } from '$lib/settings/schema';
import { getCachedWikiFiles, refreshWikiFileIndex } from '$lib/wiki/wiki-file-index';
import {
	filterFileIndex,
	getFileIndex,
	isFileIndexFresh,
	normalizeWorkspacePath,
	refreshFileIndex,
	searchWorkspaceFiles
} from '$lib/workspace/file-index';

export type { FileSearchSource };

const DEFAULT_LIMIT = 50;

function onlyFiles(paths: string[]): string[] {
	return paths.filter((path) => !path.endsWith('/'));
}

function visibleFiles(paths: string[]): string[] {
	return onlyFiles(paths).filter((path) => !path.split('/').some((part) => part.startsWith('.')));
}

function basename(path: string): string {
	const slash = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
	return slash >= 0 ? path.slice(slash + 1) : path;
}

function stem(name: string): string {
	const dot = name.lastIndexOf('.');
	return dot > 0 ? name.slice(0, dot) : name;
}

function rankFile(path: string, query: string): number {
	const p = path.toLowerCase();
	const rawBasename = basename(path);
	const rawStem = stem(rawBasename);
	const b = rawBasename.toLowerCase();
	const s = rawStem.toLowerCase();
	const q = query.toLowerCase();
	if (path === query) return 0;
	if (p === q) return 1;
	if (rawBasename === query) return 2;
	if (b === q) return 3;
	if (rawStem === query) return 4;
	if (s === q) return 5;
	if (p.startsWith(q)) return 6;
	if (b.startsWith(q)) return 7;
	return 8;
}

export function rankMatchingFiles(files: string[], query: string, limit: number): string[] {
	return rankFilePaths(onlyFiles(filterFileIndex(files, query)), query).slice(0, limit);
}

/** Rank paths for quick-open: exact / basename / prefix beats substring. */
export function rankFilePaths(files: string[], query: string): string[] {
	const trimmed = query.trim();
	if (!trimmed) {
		return [...files].sort((a, b) => a.localeCompare(b));
	}
	return [...files].sort((a, b) => {
		const rank = rankFile(a, trimmed) - rankFile(b, trimmed);
		if (rank !== 0) return rank;
		return a.localeCompare(b);
	});
}

async function loadWikiOptions(query: string, limit: number): Promise<string[]> {
	const trimmed = query.trim();
	if (!trimmed) {
		return rankFilePaths(visibleFiles(await refreshWikiFileIndex()), '').slice(0, limit);
	}

	const cached = getCachedWikiFiles();
	if (cached.length > 0) {
		const local = onlyFiles(filterFileIndex(cached, trimmed));
		if (local.length >= limit) {
			return rankFilePaths(local, trimmed).slice(0, limit);
		}
	}

	const { files } = await listWikiFiles(trimmed, Math.max(limit, 100));
	return rankFilePaths(onlyFiles(files ?? []), trimmed).slice(0, limit);
}

async function loadWorkspaceOptions(
	workspacePath: string,
	query: string,
	limit: number
): Promise<string[]> {
	const path = normalizeWorkspacePath(workspacePath);
	if (!path || path === '/') return [];

	const trimmed = query.trim();
	let index = getFileIndex(path);
	if (!isFileIndexFresh(path)) {
		index = await refreshFileIndex(path);
	}
	if (!trimmed) {
		return rankFilePaths(visibleFiles(index?.files ?? []), '').slice(0, limit);
	}

	const matches = index?.truncated
		? await searchWorkspaceFiles(path, trimmed)
		: filterFileIndex(index?.files ?? [], trimmed);

	return rankFilePaths(onlyFiles(matches), trimmed).slice(0, limit);
}

/** Load ranked file paths for the ⌘P file search modal. */
export async function loadFileSearchOptions(
	source: FileSearchSource,
	workspacePath: string,
	query: string,
	limit = DEFAULT_LIMIT
): Promise<string[]> {
	if (source === 'wiki') {
		return loadWikiOptions(query, limit);
	}
	return loadWorkspaceOptions(workspacePath, query, limit);
}
